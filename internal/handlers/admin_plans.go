package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/storage"
)

const planDateLayout = "2006-01-02"

var weekdaysShort = [7]string{"пн", "вт", "ср", "чт", "пт", "сб", "вс"}

// panelBase — префикс панели, из которой пришёл запрос: раздел планов общий
// для тренера (/coach) и админа (/admin).
func panelBase(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/coach") {
		return "/coach"
	}
	return "/admin"
}

func (a *App) panelPage(r *http.Request) PageData {
	return PageData{
		BotUsername: a.cfg.BotUsername,
		Role:        auth.PanelRoleFromContext(r.Context()),
		PanelBase:   panelBase(r),
	}
}

// mondayOf — понедельник недели, в которую попадает дата.
func mondayOf(d time.Time) time.Time {
	return d.AddDate(0, 0, -(int(d.Weekday())+6)%7)
}

// weekLabel — «20.07 – 26.07» для заголовков и текста уведомления.
func weekLabel(weekStart string) string {
	d, err := time.Parse(planDateLayout, weekStart)
	if err != nil {
		return weekStart
	}
	return fmt.Sprintf("%s – %s", d.Format("02.01"), d.AddDate(0, 0, 6).Format("02.01"))
}

type planDayView struct {
	Date      string
	Label     string
	Weekday   string
	Kind      string
	Task      string
	LinkLabel string
	LinkURL   string
}

// Index — строка, потому что заготовка для «Добавить группу» рендерится
// с плейсхолдером, который JS заменяет на реальный номер.
type planGroupView struct {
	Index string
	Title string
	Days  []planDayView
}

const planGroupIndexPlaceholder = "__I__"

type planListItem struct {
	domain.TrainingPlan
	Label      string
	GroupCount int
}

// buildGroupViews раскладывает сохранённые группы на сетку недели: даты и дни
// всегда считаются от week_start, а не берутся из JSON.
func buildGroupViews(plan domain.TrainingPlan) []planGroupView {
	start, err := time.Parse(planDateLayout, plan.WeekStart)
	if err != nil {
		return nil
	}

	groups := plan.Groups
	if len(groups) == 0 {
		groups = []domain.PlanGroup{{}}
	}

	views := make([]planGroupView, 0, len(groups))
	for gi, g := range groups {
		view := planGroupView{Index: strconv.Itoa(gi), Title: g.Title, Days: make([]planDayView, 7)}
		for di := range view.Days {
			d := start.AddDate(0, 0, di)
			day := planDayView{
				Date:    d.Format(planDateLayout),
				Label:   d.Format("02.01"),
				Weekday: weekdaysShort[di],
			}
			if di < len(g.Days) {
				day.Kind = g.Days[di].Kind
				day.Task = g.Days[di].Task
				day.LinkLabel = g.Days[di].LinkLabel
				day.LinkURL = g.Days[di].LinkURL
			}
			view.Days[di] = day
		}
		views = append(views, view)
	}
	return views
}

// emptyGroupView — заготовка для кнопки «Добавить группу»: JS клонирует её
// и подставляет свой индекс вместо плейсхолдера.
func emptyGroupView(weekStart string) planGroupView {
	g := buildGroupViews(domain.TrainingPlan{WeekStart: weekStart, Groups: []domain.PlanGroup{{}}})[0]
	g.Index = planGroupIndexPlaceholder
	return g
}

func (a *App) AdminListPlansPage(w http.ResponseWriter, r *http.Request) {
	plans, err := a.store.ListPlans(r.Context(), false)
	if err != nil {
		a.logger.Error("list training plans", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	items := make([]planListItem, 0, len(plans))
	for _, p := range plans {
		items = append(items, planListItem{TrainingPlan: p, Label: weekLabel(p.WeekStart), GroupCount: len(p.Groups)})
	}

	data := struct {
		PageData
		Plans []planListItem
	}{
		PageData: a.panelPage(r),
		Plans:    items,
	}
	if err := a.renderer.Render(w, "admin_plans", data); err != nil {
		a.logger.Error("render admin_plans", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreatePlanPage(w http.ResponseWriter, r *http.Request) {
	a.renderPlanNew(w, r, mondayOf(time.Now()).AddDate(0, 0, 7).Format(planDateLayout), "")
}

func (a *App) renderPlanNew(w http.ResponseWriter, r *http.Request, weekStart, errMsg string) {
	data := struct {
		PageData
		WeekStart string
		Error     string
	}{
		PageData:  a.panelPage(r),
		WeekStart: weekStart,
		Error:     errMsg,
	}
	if errMsg != "" {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := a.renderer.Render(w, "admin_plan_new", data); err != nil {
		a.logger.Error("render admin_plan_new", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreatePlanSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	raw := strings.TrimSpace(r.FormValue("week_start"))
	d, err := time.Parse(planDateLayout, raw)
	if err != nil {
		a.renderPlanNew(w, r, raw, "Укажите дату недели")
		return
	}
	weekStart := mondayOf(d).Format(planDateLayout)

	if _, err := a.store.GetPlanByWeekAny(r.Context(), weekStart); err == nil {
		a.renderPlanNew(w, r, weekStart, "План на неделю "+weekLabel(weekStart)+" уже есть")
		return
	} else if !errors.Is(err, storage.ErrNotFound) {
		a.logger.Error("check existing plan", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var groups []domain.PlanGroup
	var materials []domain.PlanMaterial
	if r.FormValue("copy_previous") == "on" {
		if prev, err := a.store.GetLatestPlanBefore(r.Context(), weekStart); err == nil {
			groups = shiftGroups(prev.Groups, weekStart)
			materials = prev.Materials
		} else if !errors.Is(err, storage.ErrNotFound) {
			a.logger.Error("copy previous plan", "err", err)
		}
	}

	id, err := a.store.CreatePlan(r.Context(), weekStart, groups, materials)
	if err != nil {
		a.logger.Error("create training plan", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("%s/plans/%d/edit", panelBase(r), id), http.StatusSeeOther)
}

// shiftGroups переносит содержимое прошлой недели на новую, пересчитывая даты.
func shiftGroups(groups []domain.PlanGroup, weekStart string) []domain.PlanGroup {
	start, err := time.Parse(planDateLayout, weekStart)
	if err != nil {
		return nil
	}
	out := make([]domain.PlanGroup, 0, len(groups))
	for _, g := range groups {
		ng := domain.PlanGroup{Title: g.Title, Days: make([]domain.PlanDay, 0, 7)}
		for di := 0; di < 7; di++ {
			day := domain.PlanDay{Date: start.AddDate(0, 0, di).Format(planDateLayout), Weekday: di + 1}
			if di < len(g.Days) {
				day.Kind = g.Days[di].Kind
				day.Task = g.Days[di].Task
				day.LinkLabel = g.Days[di].LinkLabel
				day.LinkURL = g.Days[di].LinkURL
			}
			ng.Days = append(ng.Days, day)
		}
		out = append(out, ng)
	}
	return out
}

func (a *App) AdminEditPlanPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	plan, err := a.store.GetPlan(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get training plan", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.renderPlanForm(w, r, plan, "")
}

func (a *App) renderPlanForm(w http.ResponseWriter, r *http.Request, plan domain.TrainingPlan, errMsg string) {
	materials := plan.Materials
	if len(materials) == 0 {
		materials = []domain.PlanMaterial{{}}
	}

	data := struct {
		PageData
		Plan         domain.TrainingPlan
		Label        string
		Groups       []planGroupView
		EmptyGroup   planGroupView
		Materials    []domain.PlanMaterial
		BotConfigured bool
		Error        string
		NotifySent   string
		NotifyFailed string
		Saved        bool
	}{
		PageData:     a.panelPage(r),
		Plan:         plan,
		Label:        weekLabel(plan.WeekStart),
		Groups:       buildGroupViews(plan),
		EmptyGroup:   emptyGroupView(plan.WeekStart),
		Materials:    materials,
		BotConfigured: a.cfg.BotToken != "",
		Error:        errMsg,
		NotifySent:   r.URL.Query().Get("notify_sent"),
		NotifyFailed: r.URL.Query().Get("notify_failed"),
		Saved:        r.URL.Query().Get("saved") == "1",
	}
	if errMsg != "" {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := a.renderer.Render(w, "admin_plan_form", data); err != nil {
		a.logger.Error("render admin_plan_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminUpdatePlanSubmit(w http.ResponseWriter, r *http.Request) {
	id, ok := a.savePlanFromForm(w, r)
	if !ok {
		return
	}
	http.Redirect(w, r, fmt.Sprintf("%s/plans/%d/edit?saved=1", panelBase(r), id), http.StatusSeeOther)
}

// savePlanFromForm сохраняет сетку из редактора. Возвращает ok=false, если ответ
// уже записан (ошибка валидации перерисовывает форму).
func (a *App) savePlanFromForm(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return 0, false
	}

	plan, err := a.store.GetPlan(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return 0, false
		}
		a.logger.Error("get training plan", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return 0, false
	}

	groups, materials, errMsg := parsePlanForm(r, plan.WeekStart)
	if errMsg != "" {
		plan.Groups = groups
		plan.Materials = materials
		a.renderPlanForm(w, r, plan, errMsg)
		return 0, false
	}

	if err := a.store.UpdatePlanContent(r.Context(), id, groups, materials); err != nil {
		a.logger.Error("update training plan", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return 0, false
	}
	return id, true
}

func parsePlanForm(r *http.Request, weekStart string) ([]domain.PlanGroup, []domain.PlanMaterial, string) {
	start, err := time.Parse(planDateLayout, weekStart)
	if err != nil {
		return nil, nil, "Некорректная неделя плана"
	}

	count, _ := strconv.Atoi(r.FormValue("groups_count"))
	if count < 0 || count > 20 {
		return nil, nil, "Некорректное число групп"
	}

	var groups []domain.PlanGroup
	for gi := 0; gi < count; gi++ {
		prefix := fmt.Sprintf("g%d_", gi)
		// Пропускаем удалённые в браузере блоки: их поля не приходят.
		if _, ok := r.Form[prefix+"title"]; !ok {
			continue
		}

		g := domain.PlanGroup{
			Title: strings.TrimSpace(r.FormValue(prefix + "title")),
			Days:  make([]domain.PlanDay, 0, 7),
		}
		filled := g.Title != ""
		for di := 0; di < 7; di++ {
			dp := fmt.Sprintf("%sd%d_", prefix, di)
			day := domain.PlanDay{
				Date:      start.AddDate(0, 0, di).Format(planDateLayout),
				Weekday:   di + 1,
				Kind:      strings.TrimSpace(r.FormValue(dp + "kind")),
				Task:      strings.TrimSpace(r.FormValue(dp + "task")),
				LinkLabel: strings.TrimSpace(r.FormValue(dp + "link_label")),
				LinkURL:   strings.TrimSpace(r.FormValue(dp + "link_url")),
			}
			if day.Kind != "" || day.Task != "" || day.LinkURL != "" {
				filled = true
			}
			g.Days = append(g.Days, day)
		}
		if !filled {
			continue
		}
		if g.Title == "" {
			return nil, nil, "У каждой заполненной группы должно быть название"
		}
		groups = append(groups, g)
	}

	var materials []domain.PlanMaterial
	mCount, _ := strconv.Atoi(r.FormValue("materials_count"))
	for mi := 0; mi < mCount && mi < 20; mi++ {
		label := strings.TrimSpace(r.FormValue(fmt.Sprintf("m%d_label", mi)))
		link := strings.TrimSpace(r.FormValue(fmt.Sprintf("m%d_url", mi)))
		if label == "" && link == "" {
			continue
		}
		if label == "" || link == "" {
			return nil, nil, "У материала нужны и название, и ссылка"
		}
		materials = append(materials, domain.PlanMaterial{Label: label, URL: link})
	}

	return groups, materials, ""
}

// AdminPublishPlanSubmit — кнопка «Опубликовать» стоит в той же форме, что и
// «Сохранить», поэтому сначала сохраняем текущие правки, потом меняем статус.
func (a *App) AdminPublishPlanSubmit(w http.ResponseWriter, r *http.Request) {
	id, ok := a.savePlanFromForm(w, r)
	if !ok {
		return
	}
	a.setPlanStatus(w, r, id, domain.PlanStatusPublished)
}

func (a *App) AdminUnpublishPlanSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	a.setPlanStatus(w, r, id, domain.PlanStatusDraft)
}

func (a *App) setPlanStatus(w http.ResponseWriter, r *http.Request, id int64, status string) {
	if err := a.store.SetPlanStatus(r.Context(), id, status); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("set training plan status", "err", err, "status", status)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("%s/plans/%d/edit?saved=1", panelBase(r), id), http.StatusSeeOther)
}

func (a *App) AdminNotifyPlanSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if a.cfg.BotToken == "" {
		http.Error(w, "Telegram-бот не настроен", http.StatusBadRequest)
		return
	}

	plan, err := a.store.GetPlan(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get training plan", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !plan.IsPublished() {
		http.Error(w, "Сначала опубликуйте план", http.StatusBadRequest)
		return
	}

	// Аудитория «active» — те, у кого сейчас активна подписка и открыт диалог с ботом.
	list, err := a.store.ListNotificationUsers(r.Context(), "active")
	if err != nil {
		a.logger.Error("plan notify: list users", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	users := make([]notificationRecipient, 0, len(list))
	for _, u := range list {
		users = append(users, notificationRecipient{id: u.ID, telegramID: u.TelegramID})
	}

	text := strings.NewReplacer(
		"{week}", weekLabel(plan.WeekStart),
		"{url}", a.cfg.BaseURL+"/plan?week="+plan.WeekStart,
	).Replace(a.store.MessageTemplate(r.Context(), storage.SettingTmplPlanPublished))

	sent, failed := a.broadcast(r.Context(), users, text)
	if err := a.store.MarkPlanNotified(r.Context(), id, sent); err != nil {
		a.logger.Error("mark plan notified", "err", err)
	}

	q := url.Values{}
	q.Set("notify_sent", strconv.Itoa(sent))
	q.Set("notify_failed", strconv.Itoa(failed))
	http.Redirect(w, r, fmt.Sprintf("%s/plans/%d/edit?%s", panelBase(r), id, q.Encode()), http.StatusSeeOther)
}

func (a *App) AdminDeletePlanSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := a.store.DeletePlan(r.Context(), id); err != nil && !errors.Is(err, storage.ErrNotFound) {
		a.logger.Error("delete training plan", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, panelBase(r)+"/plans", http.StatusSeeOther)
}
