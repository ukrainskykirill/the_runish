package handlers

import (
	"errors"
	"net/http"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/storage"
)

// APIMe — GET /api/me
func (a *App) APIMe(w http.ResponseWriter, r *http.Request) {
	resp := struct {
		User                  *domain.User          `json:"user"`
		Subscriptions         []domain.Subscription `json:"subscriptions"`
		Orders                []domain.Order        `json:"orders"`
		CanBookFreeLesson     bool                  `json:"can_book_free_lesson"`
		CanChooseSubscription bool                  `json:"can_choose_subscription"`
		BotUsername           string                `json:"bot_username"`
	}{
		Subscriptions:         []domain.Subscription{},
		Orders:                []domain.Order{},
		CanBookFreeLesson:     true,
		CanChooseSubscription: true,
		BotUsername:           a.cfg.BotUsername,
	}

	user := auth.UserFromContext(r.Context())
	if user != nil {
		resp.User = user

		subs, err := a.store.ListActiveSubsByUser(r.Context(), user.ID)
		if err != nil {
			a.logger.Error("list subs", "err", err)
		} else if subs != nil {
			resp.Subscriptions = subs
		}

		orders, err := a.store.ListOrdersByUser(r.Context(), user.ID)
		if err != nil {
			a.logger.Error("list orders", "err", err)
		} else if orders != nil {
			resp.Orders = orders
		}

		resp.CanChooseSubscription = len(resp.Subscriptions) == 0

		hasPayments, err := a.store.HasConfirmedPaymentsByUser(r.Context(), user.ID)
		if err != nil {
			a.logger.Error("check confirmed payments", "err", err)
			hasPayments = true
		}
		hasSubscriptions, err := a.store.HasAnySubscriptionByUser(r.Context(), user.ID)
		if err != nil {
			a.logger.Error("check subscriptions", "err", err)
			hasSubscriptions = true
		}
		resp.CanBookFreeLesson = !hasPayments && !user.EntryFeePaid && !hasSubscriptions
	}

	writeJSON(w, http.StatusOK, resp)
}

// APIHome — GET /api/home
// Агрегирует данные для главной страницы, чтобы не делать 4 отдельных запроса.
func (a *App) APIHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	services, err := a.store.ListServices(ctx, false)
	if err != nil {
		a.logger.Error("home: list services", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if services == nil {
		services = []domain.Service{}
	}
	pc := a.pricingContext(ctx, auth.UserFromContext(ctx))
	for i := range services {
		pc.apply(&services[i])
	}

	news, err := a.store.ListNews(ctx, false)
	if err != nil {
		a.logger.Error("home: list news", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if news == nil {
		news = []domain.News{}
	}

	merch, err := a.store.ListMerch(ctx, false)
	if err != nil {
		a.logger.Error("home: list merch", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if merch == nil {
		merch = []domain.Merch{}
	}

	trainings, err := a.store.ListTrainings(ctx, false)
	if err != nil {
		a.logger.Error("home: list trainings", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if trainings == nil {
		trainings = []domain.Training{}
	}

	writeJSON(w, http.StatusOK, struct {
		Services    []domain.Service  `json:"services"`
		PromoActive bool              `json:"promo_active"`
		News        []domain.News     `json:"news"`
		Merch       []domain.Merch    `json:"merch"`
		Trainings   []domain.Training `json:"trainings"`
	}{
		Services:    limitSlice(services, 4),
		PromoActive: pc.promoActive,
		News:        limitSlice(news, 3),
		Merch:       limitSlice(merch, 3),
		Trainings:   trainings,
	})
}

// APICatalog — GET /api/catalog
func (a *App) APICatalog(w http.ResponseWriter, r *http.Request) {
	services, err := a.store.ListServices(r.Context(), false)
	if err != nil {
		a.logger.Error("list services", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if services == nil {
		services = []domain.Service{}
	}

	// Эффективные цены и доступность зависят от текущего юзера и акции «первых 30».
	pc := a.pricingContext(r.Context(), auth.UserFromContext(r.Context()))
	for i := range services {
		pc.apply(&services[i])
	}

	writeJSON(w, http.StatusOK, struct {
		Services    []domain.Service `json:"services"`
		PromoActive bool             `json:"promo_active"`
	}{Services: services, PromoActive: pc.promoActive})
}

// APINews — GET /api/news
func (a *App) APINews(w http.ResponseWriter, r *http.Request) {
	news, err := a.store.ListNews(r.Context(), false)
	if err != nil {
		a.logger.Error("list news", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if news == nil {
		news = []domain.News{}
	}
	writeJSON(w, http.StatusOK, struct {
		News []domain.News `json:"news"`
	}{News: news})
}

// APISchedule — GET /api/schedule. Активные тренировки недельного расписания.
func (a *App) APISchedule(w http.ResponseWriter, r *http.Request) {
	trainings, err := a.store.ListTrainings(r.Context(), false)
	if err != nil {
		a.logger.Error("list trainings", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if trainings == nil {
		trainings = []domain.Training{}
	}
	writeJSON(w, http.StatusOK, struct {
		Trainings []domain.Training `json:"trainings"`
	}{Trainings: trainings})
}

// APINewsItem — GET /api/news/{id}
func (a *App) APINewsItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	item, err := a.store.GetNews(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found")
			return
		}
		a.logger.Error("get news", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// APIMerch — GET /api/merch
func (a *App) APIMerch(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListMerch(r.Context(), false)
	if err != nil {
		a.logger.Error("list merch", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if items == nil {
		items = []domain.Merch{}
	}
	writeJSON(w, http.StatusOK, struct {
		Merch []domain.Merch `json:"merch"`
	}{Merch: items})
}

func limitSlice[T any](items []T, limit int) []T {
	if limit >= 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}
