package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"therunish/internal/domain"
	"therunish/internal/payment"
	"therunish/internal/storage"
)

// AdminRefundPaymentSubmit — инициировать возврат платежа (POST /admin/payments/{id}/refund).
// Вызывает provider.Refund; фактическое изменение статуса произойдёт по вебхуку REFUNDED.
func (a *App) AdminRefundPaymentSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	p, err := a.store.GetPaymentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get payment for refund", "err", err, "id", id)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Возврат возможен только для подтверждённого платежа.
	if p.Status != domain.PaymentStatusConfirmed {
		http.Error(w, "Можно вернуть только подтверждённый платёж", http.StatusBadRequest)
		return
	}
	// Нужен T-Bank PaymentId для вызова /Refund.
	if p.TBankPaymentID == "" {
		http.Error(w, "У платежа нет T-Bank PaymentId", http.StatusBadRequest)
		return
	}

	// Чек возврата (54-ФЗ): формируем по позициям исходного заказа, если был телефон.
	var refundReceipt *payment.Receipt
	if order, err := a.store.GetOrderByID(r.Context(), p.OrderID); err != nil {
		a.logger.Error("get order for refund receipt", "err", err, "order_id", p.OrderID)
	} else {
		refundReceipt = a.buildReceipt(&order)
	}

	// Инициируем возврат на полную сумму.
	if err := a.provider.Refund(r.Context(), p.TBankPaymentID, p.AmountKop, refundReceipt); err != nil {
		a.logger.Error("provider refund", "err", err, "payment_id", id)
		http.Error(w, "Ошибка возврата: "+err.Error(), http.StatusBadGateway)
		return
	}

	a.logger.Info("refund initiated", "payment_id", id, "amount_kop", p.AmountKop, "admin", r.Header.Get("X-Admin-Token"))
	http.Redirect(w, r, "/admin/payments", http.StatusSeeOther)
}

// AdminDashboardPage — главная страница админки со статистикой оплат (GET /admin).
func (a *App) AdminDashboardPage(w http.ResponseWriter, r *http.Request) {
	dash, err := a.store.GetPaymentDashboard(r.Context())
	if err != nil {
		a.logger.Error("payment dashboard", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Dashboard storage.PaymentDashboard
	}{
		PageData:  PageData{BotUsername: a.cfg.BotUsername},
		Dashboard: dash,
	}
	if err := a.renderer.Render(w, "admin_dashboard", data); err != nil {
		a.logger.Error("render admin_dashboard", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminListServicesPage — список услуг в HTML-таблице (GET /admin/services).
func (a *App) AdminListServicesPage(w http.ResponseWriter, r *http.Request) {
	services, err := a.store.ListServices(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list services", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Services []domain.Service
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Services: services,
	}
	if err := a.renderer.Render(w, "admin_services", data); err != nil {
		a.logger.Error("render admin_services", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminCreateServicePage — форма создания (GET /admin/services/new).
func (a *App) AdminCreateServicePage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		PageData
		Service *domain.Service
		Error   string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
	}
	if err := a.renderer.Render(w, "admin_service_form", data); err != nil {
		a.logger.Error("render admin_service_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminEditServicePage — форма редактирования (GET /admin/services/{id}/edit).
func (a *App) AdminEditServicePage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	svc, err := a.store.GetService(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Service *domain.Service
		Error   string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Service:  &svc,
	}
	if err := a.renderer.Render(w, "admin_service_form", data); err != nil {
		a.logger.Error("render admin_service_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminCreateServiceSubmit — создать услугу (POST /admin/services).
func (a *App) AdminCreateServiceSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	svc, errMsg := parseServiceForm(r)
	if errMsg != "" {
		data := struct {
			PageData
			Service *domain.Service
			Error   string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			Service:  svc,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_service_form", data)
		return
	}

	if _, err := a.store.CreateService(r.Context(), svc); err != nil {
		a.logger.Error("create service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/services", http.StatusSeeOther)
}

// AdminUpdateServiceSubmit — обновить услугу (POST /admin/services/{id}).
func (a *App) AdminUpdateServiceSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	svc, errMsg := parseServiceForm(r)
	if errMsg != "" {
		svc.ID = id
		data := struct {
			PageData
			Service *domain.Service
			Error   string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			Service:  svc,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_service_form", data)
		return
	}

	svc.ID = id
	if err := a.store.UpdateService(r.Context(), svc); err != nil {
		a.logger.Error("update service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/services", http.StatusSeeOther)
}

// AdminDeleteServiceSubmit — мягкое удаление (POST /admin/services/{id}/delete).
func (a *App) AdminDeleteServiceSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.DeactivateService(r.Context(), id); err != nil {
		a.logger.Error("delete service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/services", http.StatusSeeOther)
}

// AdminListPaymentsPage — последние оплаты (GET /admin/payments).
func (a *App) AdminListPaymentsPage(w http.ResponseWriter, r *http.Request) {
	payments, err := a.store.ListRecentPayments(r.Context(), 50)
	if err != nil {
		a.logger.Error("list payments", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Payments []storage.PaymentWithDetails
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Payments: payments,
	}
	if err := a.renderer.Render(w, "admin_payments", data); err != nil {
		a.logger.Error("render admin_payments", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminListNewsPage — список новостей (GET /admin/news).
func (a *App) AdminListNewsPage(w http.ResponseWriter, r *http.Request) {
	news, err := a.store.ListNews(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		News []domain.News
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		News:     news,
	}
	if err := a.renderer.Render(w, "admin_news", data); err != nil {
		a.logger.Error("render admin_news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminCreateNewsPage — форма создания новости (GET /admin/news/new).
func (a *App) AdminCreateNewsPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		PageData
		News  *domain.News
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
	}
	if err := a.renderer.Render(w, "admin_news_form", data); err != nil {
		a.logger.Error("render admin_news_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminEditNewsPage — форма редактирования новости (GET /admin/news/{id}/edit).
func (a *App) AdminEditNewsPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	n, err := a.store.GetNews(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		News  *domain.News
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		News:     &n,
	}
	if err := a.renderer.Render(w, "admin_news_form", data); err != nil {
		a.logger.Error("render admin_news_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminCreateNewsSubmit — создать новость (POST /admin/news).
func (a *App) AdminCreateNewsSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	n, errMsg := parseNewsForm(r)
	if errMsg != "" {
		data := struct {
			PageData
			News  *domain.News
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			News:     n,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_news_form", data)
		return
	}

	if _, err := a.store.CreateNews(r.Context(), n); err != nil {
		a.logger.Error("create news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
}

// AdminUpdateNewsSubmit — обновить новость (POST /admin/news/{id}).
func (a *App) AdminUpdateNewsSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	n, errMsg := parseNewsForm(r)
	if errMsg != "" {
		n.ID = id
		data := struct {
			PageData
			News  *domain.News
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			News:     n,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_news_form", data)
		return
	}

	n.ID = id
	if err := a.store.UpdateNews(r.Context(), n); err != nil {
		a.logger.Error("update news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
}

// AdminDeleteNewsSubmit — мягкое удаление новости (POST /admin/news/{id}/delete).
func (a *App) AdminDeleteNewsSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.DeactivateNews(r.Context(), id); err != nil {
		a.logger.Error("delete news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
}

// AdminListMerchPage — список мерча (GET /admin/merch).
func (a *App) AdminListMerchPage(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListMerch(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Merch []domain.Merch
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Merch:    items,
	}
	if err := a.renderer.Render(w, "admin_merch", data); err != nil {
		a.logger.Error("render admin_merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminCreateMerchPage — форма создания мерча (GET /admin/merch/new).
func (a *App) AdminCreateMerchPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		PageData
		Merch *domain.Merch
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
	}
	if err := a.renderer.Render(w, "admin_merch_form", data); err != nil {
		a.logger.Error("render admin_merch_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminEditMerchPage — форма редактирования мерча (GET /admin/merch/{id}/edit).
func (a *App) AdminEditMerchPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	m, err := a.store.GetMerch(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Merch *domain.Merch
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Merch:    &m,
	}
	if err := a.renderer.Render(w, "admin_merch_form", data); err != nil {
		a.logger.Error("render admin_merch_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminCreateMerchSubmit — создать мерч (POST /admin/merch).
func (a *App) AdminCreateMerchSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	m, errMsg := parseMerchForm(r)
	if errMsg != "" {
		data := struct {
			PageData
			Merch *domain.Merch
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			Merch:    m,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_merch_form", data)
		return
	}

	if _, err := a.store.CreateMerch(r.Context(), m); err != nil {
		a.logger.Error("create merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/merch", http.StatusSeeOther)
}

// AdminUpdateMerchSubmit — обновить мерч (POST /admin/merch/{id}).
func (a *App) AdminUpdateMerchSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	m, errMsg := parseMerchForm(r)
	if errMsg != "" {
		m.ID = id
		data := struct {
			PageData
			Merch *domain.Merch
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			Merch:    m,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_merch_form", data)
		return
	}

	m.ID = id
	if err := a.store.UpdateMerch(r.Context(), m); err != nil {
		a.logger.Error("update merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/merch", http.StatusSeeOther)
}

// AdminDeleteMerchSubmit — мягкое удаление мерча (POST /admin/merch/{id}/delete).
func (a *App) AdminDeleteMerchSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.DeactivateMerch(r.Context(), id); err != nil {
		a.logger.Error("delete merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/merch", http.StatusSeeOther)
}

// parseServiceForm — извлекает Service из form-данных.
func parseServiceForm(r *http.Request) (*domain.Service, string) {
	kind := r.FormValue("kind")
	title := r.FormValue("title")
	desc := r.FormValue("description")
	priceStr := r.FormValue("price_rub")
	durationStr := r.FormValue("duration_days")
	sortStr := r.FormValue("sort_order")
	isActive := r.FormValue("is_active") == "on"

	svcKind := domain.ServiceKind(kind)
	if !svcKind.IsValid() {
		return nil, "Неверный тип услуги"
	}

	priceKop, errMsg := parseRubles(priceStr)
	if errMsg != "" {
		return nil, errMsg
	}

	if svcKind == domain.KindFree && priceKop != 0 {
		return nil, "Бесплатная услуга должна иметь цену 0"
	}

	var duration *int
	if durationStr != "" {
		d, err := strconv.Atoi(durationStr)
		if err != nil || d <= 0 {
			return nil, "Срок должен быть положительным числом"
		}
		duration = &d
	}

	if svcKind.GrantsSubscription() && (duration == nil || *duration <= 0) {
		return nil, "Подписка/бандл требуют срок (дни)"
	}

	// Скидочные цены — опциональные.
	withSub, errMsg := parseOptionalRubles(r.FormValue("price_with_sub_rub"))
	if errMsg != "" {
		return nil, "Цена при подписке: " + errMsg
	}
	promo, errMsg := parseOptionalRubles(r.FormValue("promo_price_rub"))
	if errMsg != "" {
		return nil, "Льготная цена: " + errMsg
	}

	sortOrder, _ := strconv.Atoi(sortStr)

	return &domain.Service{
		Kind:            svcKind,
		Title:           title,
		Description:     desc,
		PriceKop:        priceKop,
		DurationDays:    duration,
		SortOrder:       sortOrder,
		IsActive:        isActive,
		PriceWithSubKop: withSub,
		PromoPriceKop:   promo,
	}, ""
}

// parseOptionalRubles парсит необязательное рублёвое поле в *int64 (пусто → nil).
func parseOptionalRubles(str string) (*int64, string) {
	if str == "" {
		return nil, ""
	}
	v, errMsg := parseRubles(str)
	if errMsg != "" {
		return nil, errMsg
	}
	return &v, ""
}

// parseNewsForm — извлекает News из form-данных.
func parseNewsForm(r *http.Request) (*domain.News, string) {
	title := r.FormValue("title")
	content := r.FormValue("content")
	sortStr := r.FormValue("sort_order")
	isActive := r.FormValue("is_active") == "on"

	if title == "" {
		return nil, "Заголовок обязателен"
	}
	if content == "" {
		return nil, "Текст новости обязателен"
	}

	sortOrder, _ := strconv.Atoi(sortStr)

	return &domain.News{
		Title:     title,
		Content:   content,
		SortOrder: sortOrder,
		IsActive:  isActive,
	}, ""
}

// parseMerchForm — извлекает Merch из form-данных.
func parseMerchForm(r *http.Request) (*domain.Merch, string) {
	title := r.FormValue("title")
	desc := r.FormValue("description")
	priceStr := r.FormValue("price_rub")
	sortStr := r.FormValue("sort_order")
	isActive := r.FormValue("is_active") == "on"

	if title == "" {
		return nil, "Название обязательно"
	}

	priceKop, errMsg := parseRubles(priceStr)
	if errMsg != "" {
		return nil, errMsg
	}

	sortOrder, _ := strconv.Atoi(sortStr)

	return &domain.Merch{
		Title:       title,
		Description: desc,
		PriceKop:    priceKop,
		SortOrder:   sortOrder,
		IsActive:    isActive,
	}, ""
}

// parseRubles парсит строку рублей ("8000" или "800.50") в копейки (int64).
// Возвращает (kopecks, "") при успехе или (0, "сообщение об ошибке").
func parseRubles(s string) (int64, string) {
	if s == "" {
		return 0, "Цена обязательна"
	}
	rub, err := strconv.ParseFloat(s, 64)
	if err != nil || rub < 0 {
		return 0, "Цена должна быть неотрицательным числом"
	}
	// Умножаем на 100 и округляем до целого, чтобы избежать float-погрешностей.
	kop := int64(rub*100 + 0.5)
	return kop, ""
}
