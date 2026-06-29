package botworker

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"therunish/internal/domain"
	"therunish/internal/storage"
	"therunish/internal/survey"
	"therunish/internal/telegram"
)

// maybeStartOrContinueSurvey запускает онбординг-анкету для нового юзера (status=pending)
// сразу после регистрации в боте, либо повторяет текущий вопрос, если анкета уже идёт.
// Для пользователей без записи (зарегистрированы до фичи) и завершивших — ничего не делает.
func (w *Worker) maybeStartOrContinueSurvey(ctx context.Context, userID, chatID int64) {
	sv, err := w.store.GetSurvey(ctx, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return
	}
	if err != nil {
		w.logger.Error("survey: get", "err", err, "user_id", userID)
		return
	}

	switch sv.Status {
	case domain.SurveyPending:
		if err := w.bot.SendMessage(ctx, chatID, survey.Greeting()); err != nil {
			w.logger.Error("survey: send greeting", "err", err, "user_id", userID)
		}
		w.sendStep(ctx, chatID, userID, "", survey.FirstStep(), map[string]any{})
	case domain.SurveyInProgress:
		// Повторный /start посреди анкеты — просто переспросим текущий вопрос.
		w.sendStep(ctx, chatID, userID, sv.Branch, sv.Step, sv.Answers)
	}
}

// handleSurveyText обрабатывает свободный ввод как ответ на текстовый шаг анкеты.
func (w *Worker) handleSurveyText(ctx context.Context, tgID, chatID int64, text string) {
	if text == "" {
		return
	}
	user, err := w.store.GetUserByTelegramID(ctx, tgID)
	if err != nil {
		return
	}
	sv, err := w.store.GetSurvey(ctx, user.ID)
	if err != nil || sv.Status != domain.SurveyInProgress {
		return
	}
	st, ok := survey.Get(sv.Step)
	if !ok {
		return
	}
	if st.Kind != survey.Text {
		// Ждём нажатия кнопки, а не текста.
		_ = w.bot.SendMessage(ctx, chatID, "Пожалуйста, выбери вариант кнопкой ниже 🙂")
		return
	}

	if sv.Answers == nil {
		sv.Answers = map[string]any{}
	}
	sv.Answers[sv.Step] = text
	w.advance(ctx, chatID, user.ID, sv.Branch, sv.Step, sv.Answers)
}

// handleCallbackQuery обрабатывает нажатия инлайн-кнопок (одиночный и мульти-выбор).
func (w *Worker) handleCallbackQuery(ctx context.Context, u telegram.Update) {
	cq := u.CallbackQuery
	_ = w.bot.AnswerCallbackQuery(ctx, cq.ID)

	chatID := cq.From.ID
	if cq.Message != nil {
		chatID = cq.Message.Chat.ID
	}

	user, err := w.store.GetUserByTelegramID(ctx, cq.From.ID)
	if err != nil {
		return
	}
	sv, err := w.store.GetSurvey(ctx, user.ID)
	if err != nil || sv.Status != domain.SurveyInProgress {
		return
	}

	stepKey, arg, ok := splitCallback(cq.Data)
	if !ok || stepKey != sv.Step {
		return // устаревшая или чужая кнопка
	}
	st, ok := survey.Get(stepKey)
	if !ok {
		return
	}
	if sv.Answers == nil {
		sv.Answers = map[string]any{}
	}

	switch st.Kind {
	case survey.Single:
		idx, err := strconv.Atoi(arg)
		if err != nil || idx < 0 || idx >= len(st.Options) {
			return
		}
		chosen := st.Options[idx]
		sv.Answers[stepKey] = chosen
		if sv.MsgID != nil {
			_ = w.bot.EditMessageText(ctx, chatID, *sv.MsgID, st.Text+"\n\n✅ "+chosen, nil)
		}
		branch := sv.Branch
		if stepKey == "experience" {
			branch = survey.BranchFor(idx)
		}
		w.advance(ctx, chatID, user.ID, branch, stepKey, sv.Answers)

	case survey.Multi:
		if arg == "done" {
			summary := "—"
			if sel := selectedStrings(sv.Answers[stepKey]); len(sel) > 0 {
				summary = strings.Join(sel, ", ")
			}
			if sv.MsgID != nil {
				_ = w.bot.EditMessageText(ctx, chatID, *sv.MsgID, st.Text+"\n\n✅ "+summary, nil)
			}
			w.advance(ctx, chatID, user.ID, sv.Branch, stepKey, sv.Answers)
			return
		}
		idx, err := strconv.Atoi(arg)
		if err != nil || idx < 0 || idx >= len(st.Options) {
			return
		}
		selected := toggle(selectedStrings(sv.Answers[stepKey]), st.Options[idx])
		sv.Answers[stepKey] = selected
		w.saveState(ctx, user.ID, sv.Branch, stepKey, sv.Answers, sv.MsgID)
		if sv.MsgID != nil {
			_ = w.bot.EditMessageText(ctx, chatID, *sv.MsgID, st.Text, multiKeyboard(stepKey, st.Options, selected))
		}
	}
}

// sendStep отправляет вопрос текущего шага и сохраняет состояние анкеты.
func (w *Worker) sendStep(ctx context.Context, chatID, userID int64, branch, stepKey string, answers map[string]any) {
	st, ok := survey.Get(stepKey)
	if !ok {
		w.logger.Error("survey: unknown step", "step", stepKey)
		return
	}

	switch st.Kind {
	case survey.Text:
		if err := w.bot.SendMessage(ctx, chatID, st.Text); err != nil {
			w.logger.Error("survey: send text step", "err", err, "step", stepKey)
			return
		}
		w.saveState(ctx, userID, branch, stepKey, answers, nil)

	case survey.Single:
		msgID, err := w.bot.SendMessageWithKeyboard(ctx, chatID, st.Text, singleKeyboard(stepKey, st.Options))
		if err != nil {
			w.logger.Error("survey: send single step", "err", err, "step", stepKey)
			return
		}
		w.saveState(ctx, userID, branch, stepKey, answers, &msgID)

	case survey.Multi:
		selected := selectedStrings(answers[stepKey])
		msgID, err := w.bot.SendMessageWithKeyboard(ctx, chatID, st.Text, multiKeyboard(stepKey, st.Options, selected))
		if err != nil {
			w.logger.Error("survey: send multi step", "err", err, "step", stepKey)
			return
		}
		w.saveState(ctx, userID, branch, stepKey, answers, &msgID)
	}
}

// advance переходит к следующему шагу либо завершает анкету.
func (w *Worker) advance(ctx context.Context, chatID, userID int64, branch, curStep string, answers map[string]any) {
	next, done := survey.Next(branch, curStep)
	if done {
		if err := w.store.CompleteSurvey(ctx, userID, answers); err != nil {
			w.logger.Error("survey: complete", "err", err, "user_id", userID)
		}
		if err := w.bot.SendMessage(ctx, chatID, survey.Final()); err != nil {
			w.logger.Error("survey: send final", "err", err, "user_id", userID)
		}
		return
	}
	w.sendStep(ctx, chatID, userID, branch, next, answers)
}

func (w *Worker) saveState(ctx context.Context, userID int64, branch, step string, answers map[string]any, msgID *int64) {
	if err := w.store.SaveSurvey(ctx, userID, domain.SurveyInProgress, branch, step, answers, msgID); err != nil {
		w.logger.Error("survey: save state", "err", err, "user_id", userID)
	}
}

// --- клавиатуры и утилиты ---

func singleKeyboard(stepKey string, options []string) telegram.InlineKeyboard {
	kb := make(telegram.InlineKeyboard, 0, len(options))
	for i, opt := range options {
		kb = append(kb, []telegram.InlineButton{{
			Text: opt,
			Data: stepKey + "|" + strconv.Itoa(i),
		}})
	}
	return kb
}

func multiKeyboard(stepKey string, options, selected []string) telegram.InlineKeyboard {
	kb := make(telegram.InlineKeyboard, 0, len(options)+1)
	for i, opt := range options {
		text := opt
		if contains(selected, opt) {
			text = "✅ " + opt
		}
		kb = append(kb, []telegram.InlineButton{{
			Text: text,
			Data: stepKey + "|" + strconv.Itoa(i),
		}})
	}
	kb = append(kb, []telegram.InlineButton{{Text: "Готово ✅", Data: stepKey + "|done"}})
	return kb
}

// splitCallback разбирает callback-данные вида "<stepKey>|<arg>".
func splitCallback(data string) (stepKey, arg string, ok bool) {
	i := strings.IndexByte(data, '|')
	if i < 0 {
		return "", "", false
	}
	return data[:i], data[i+1:], true
}

// selectedStrings извлекает выбранные опции из значения answers (string-слайс,
// который после JSON-раунда приходит как []any).
func selectedStrings(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func toggle(list []string, item string) []string {
	for i, x := range list {
		if x == item {
			return append(list[:i], list[i+1:]...)
		}
	}
	return append(list, item)
}

func contains(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}
