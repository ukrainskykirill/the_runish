package botworker

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"therunish/internal/observability"
	"therunish/internal/payment"
	"therunish/internal/storage"
	"therunish/internal/telegram"
)

func mskLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

var monthsRuGen = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

func formatDateRu(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return fmt.Sprintf("%d %s", t.Day(), monthsRuGen[t.Month()-1])
}

func (w *Worker) quietNow(ctx context.Context) bool {
	from, to := w.store.QuietWindow(ctx)
	if from == to {
		return false
	}
	h := time.Now().In(mskLocation()).Hour()
	if from < to {
		return h >= from && h < to
	}
	return h >= from || h < to
}

func renderTemplate(tmpl string, vars map[string]string) string {
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

func pluralRu(n int, one, few, many string) string {
	m := n % 100
	if m >= 11 && m <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

func humanLeft(d time.Duration) string {
	h := int(d.Hours() + 0.5)
	if h < 1 {
		h = 1
	}
	if h%24 == 0 {
		days := h / 24
		return fmt.Sprintf("%d %s", days, pluralRu(days, "день", "дня", "дней"))
	}
	return fmt.Sprintf("%d %s", h, pluralRu(h, "час", "часа", "часов"))
}

func (w *Worker) runTrainingReminders(ctx context.Context) {
	if w.quietNow(ctx) {
		return
	}

	leadHours, err := w.store.TrainingReminderHours(ctx)
	if err != nil {
		leadHours = 24
	}

	candidates, err := w.store.ListTrainingReminderCandidates(ctx, leadHours)
	if err != nil {
		w.logger.Error("list training reminder candidates", "err", err)
		return
	}
	if len(candidates) == 0 {
		return
	}

	tmpl := w.store.MessageTemplate(ctx, storage.SettingTmplTrainingReminder)
	scheduleURL := w.cfg.BaseURL + "/schedule"

	for _, c := range candidates {
		text := renderTemplate(tmpl, map[string]string{
			"title": c.Title,
			"date":  formatDateRu(c.SessionDate),
			"time":  c.StartTime,
			"place": c.Place,
			"url":   scheduleURL,
		})
		if err := w.bot.SendMessage(ctx, c.TelegramID, text); err != nil {
			var apiErr *telegram.APIError
			if errors.As(err, &apiErr) && apiErr.IsForbidden() {
				_ = w.store.SetBotDialogOpen(ctx, c.UserID, false)
				w.logger.Info("bot blocked by user", "user_id", c.UserID)
			} else {
				observability.Alert(ctx, w.logger, "Уведомление в бот не отправлено (напоминание о тренировке)", err, "user_id", c.UserID)
			}
			continue
		}

		if err := w.store.MarkTrainingReminded(ctx, c.RegID); err != nil {
			w.logger.Error("mark training reminded", "err", err, "reg_id", c.RegID)
		}
	}
}

func (w *Worker) runReminders(ctx context.Context) {
	if w.quietNow(ctx) {
		return
	}

	hours, err := w.store.SubscriptionReminderHours(ctx)
	if err != nil {
		w.logger.Error("load reminder hours", "err", err)
		hours = []int{168, 72, 24}
	}

	cands, err := w.store.ListReminderCandidatesByHours(ctx, hours)
	if err != nil {
		w.logger.Error("list reminder candidates", "err", err)
		return
	}
	if len(cands) == 0 {
		return
	}

	type group struct {
		cand       storage.ReminderCandidate
		thresholds []int
	}
	order := make([]int64, 0)
	groups := make(map[int64]*group)
	for _, c := range cands {
		g, ok := groups[c.ID]
		if !ok {
			g = &group{cand: c}
			groups[c.ID] = g
			order = append(order, c.ID)
		}
		g.thresholds = append(g.thresholds, c.HoursBefore)
	}

	w.logger.Info("processing reminders", "subscriptions", len(order))

	tmpl := w.store.MessageTemplate(ctx, storage.SettingTmplSubReminder)
	runnersURL := w.cfg.BaseURL + "/runners"

	for _, id := range order {
		g := groups[id]

		user, err := w.store.GetUserByID(ctx, g.cand.UserID)
		if err != nil {
			w.logger.Error("get user for reminder", "err", err, "user_id", g.cand.UserID)
			continue
		}

		svc, err := w.store.GetService(ctx, g.cand.ServiceID)
		if err != nil {
			w.logger.Error("get service for reminder", "err", err, "service_id", g.cand.ServiceID)
			continue
		}

		left := time.Until(g.cand.ExpiresAt)
		text := renderTemplate(tmpl, map[string]string{
			"title": svc.Title,
			"left":  humanLeft(left),
			"hours": strconv.Itoa(int(left.Hours() + 0.5)),
			"url":   runnersURL,
		})
		if err := w.bot.SendMessage(ctx, user.TelegramID, text); err != nil {
			var apiErr *telegram.APIError
			if errors.As(err, &apiErr) && apiErr.IsForbidden() {
				_ = w.store.SetBotDialogOpen(ctx, user.ID, false)
				w.logger.Info("bot blocked by user", "user_id", user.ID)
			} else {
				observability.Alert(ctx, w.logger, "Уведомление в бот не отправлено (напоминание о подписке)", err, "user_id", user.ID)
			}
			continue
		}

		for _, h := range g.thresholds {
			if err := w.store.MarkRemindedHours(ctx, id, h); err != nil {
				w.logger.Error("mark reminded", "err", err, "sub_id", id, "hours", h)
			}
		}
	}
}

func (w *Worker) runExpire(ctx context.Context) {
	n, err := w.store.ExpireOverdue(ctx)
	if err != nil {
		w.logger.Error("expire overdue", "err", err)
		return
	}
	if n > 0 {
		w.logger.Info("expired subscriptions", "count", n)
	}
}

func (w *Worker) runGetState(ctx context.Context) {
	payments, err := w.store.ListPendingPaymentsOlderThan(ctx, 30*time.Minute)
	if err != nil {
		w.logger.Error("list pending payments", "err", err)
		return
	}
	if len(payments) == 0 {
		return
	}

	w.logger.Info("checking stuck payments", "count", len(payments))

	for _, p := range payments {
		if p.TBankPaymentID == "" {
			continue
		}

		status, err := w.provider.GetState(ctx, p.TBankPaymentID)
		if err != nil {
			w.logger.Error("getstate", "err", err, "payment_id", p.ID)
			continue
		}

		mapped := payment.MapStatus(status)
		switch mapped {
		case "confirmed":
			if err := w.store.ActivateSubscriptionTx(ctx, p.TBankOrderID, p.TBankPaymentID, status); err != nil {
				w.logger.Error("activate from getstate", "err", err, "order_id", p.TBankOrderID)
			}
			w.logger.Info("payment confirmed via getstate", "order_id", p.TBankOrderID)
		case "rejected":
			if err := w.store.RejectPaymentTx(ctx, p.TBankOrderID, p.TBankPaymentID, status, p.ErrorCode); err != nil {
				w.logger.Error("reject from getstate", "err", err, "order_id", p.TBankOrderID)
			}
		}
	}
}
