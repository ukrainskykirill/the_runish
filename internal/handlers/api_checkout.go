package handlers

import (
	"fmt"
	"net/http"
	"time"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/payment"
)

// APICheckout — POST /api/checkout. Создаёт order + payment, возвращает payment_url.
func (a *App) APICheckout(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "login_required")
		return
	}

	// Без телефона оплатить нельзя — он нужен для фискального чека (54-ФЗ).
	if user.Phone == "" {
		writeJSONError(w, http.StatusBadRequest, "phone_required")
		return
	}

	sessionID := a.sessions.SessionID(r)
	cart, _ := a.store.GetCart(r.Context(), sessionID)
	if len(cart) == 0 {
		writeJSONError(w, http.StatusBadRequest, "cart_empty")
		return
	}

	// Резолвим услуги из БД.
	ids := make([]int64, len(cart))
	for i, it := range cart {
		ids[i] = it.ServiceID
	}
	services, err := a.store.ListServicesByIDs(r.Context(), ids)
	if err != nil {
		a.logger.Error("resolve services", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	// Собираем позиции, считаем сумму по эффективной цене и проверяем доступность.
	pc := a.pricingContext(r.Context(), user)
	var total int64
	var subCount int // подписочных позиций (подписка/бандл) — допустимо не больше одной
	items := make([]domain.OrderItem, 0, len(cart))
	for _, it := range cart {
		svc, ok := services[it.ServiceID]
		if !ok {
			continue
		}
		// Каждый товар — максимум 1 шт.
		qty := it.Qty
		if qty > 1 {
			qty = 1
		}
		if svc.Kind.GrantsSubscription() {
			subCount += qty
			if subCount > 1 {
				writeJSONError(w, http.StatusBadRequest, "multiple_subscriptions")
				return
			}
		}
		pc.apply(&svc)
		// Гейтинг: подписку без взноса (locked) и повторный взнос/бандл (owned) не пропускаем.
		if svc.Locked {
			writeJSONError(w, http.StatusBadRequest, "entry_fee_required")
			return
		}
		if svc.Owned {
			writeJSONError(w, http.StatusBadRequest, "already_owned")
			return
		}
		items = append(items, domain.OrderItem{
			ServiceID: svc.ID,
			Title:     svc.Title,
			PriceKop:  svc.EffectivePriceKop,
			Qty:       qty,
		})
		total += svc.EffectivePriceKop * int64(qty)
	}
	if len(items) == 0 {
		writeJSONError(w, http.StatusBadRequest, "cart_invalid")
		return
	}

	// Создаём заказ.
	order := &domain.Order{
		UserID:       user.ID,
		AmountKop:    total,
		Status:       domain.OrderStatusCreated,
		ContactPhone: user.Phone,
		ContactName:  user.FullName,
		ContactTg:    user.Username,
		Items:        items,
	}
	orderID, err := a.store.CreateOrder(r.Context(), order)
	if err != nil {
		a.logger.Error("create order", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	// Генерируем уникальный tbank_order_id.
	tbankOrderID := fmt.Sprintf("o%d-%d", orderID, time.Now().UnixNano())

	// Создаём платёж.
	pay := &domain.Payment{
		UserID:       user.ID,
		OrderID:      orderID,
		AmountKop:    total,
		Status:       domain.PaymentStatusNew,
		Provider:     a.cfg.PaymentProvider,
		TBankOrderID: tbankOrderID,
	}
	payID, err := a.store.CreatePayment(r.Context(), pay)
	if err != nil {
		a.logger.Error("create payment", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	// Вызываем Init у провайдера.
	notifURL := a.cfg.BaseURL + "/payment/webhook"
	successURL := a.cfg.BaseURL + "/payment/success"
	failURL := a.cfg.BaseURL + "/payment/fail"

	initParams := payment.InitParams{
		OrderID:         tbankOrderID,
		AmountKop:       total,
		Description:     fmt.Sprintf("Заказ #%d — The Runish", orderID),
		NotificationURL: notifURL,
		SuccessURL:      successURL,
		FailURL:         failURL,
	}

	// Фискальный чек 54-ФЗ: формируем, если есть телефон покупателя.
	if rcpt := a.buildReceipt(order); rcpt != nil {
		initParams.Receipt = rcpt
	} else {
		// Без контакта чек не сформировать — логируем, но не блокируем оплату.
		a.logger.Warn("receipt skipped: no contact phone",
			"order_id", orderID, "user_id", user.ID)
	}

	result, err := a.provider.Init(r.Context(), initParams)
	if err != nil {
		a.logger.Error("payment init", "err", err)
		writeJSONError(w, http.StatusBadGateway, "payment_init_failed")
		return
	}

	// Сохраняем PaymentId и PaymentURL, обновляем статус.
	var status domain.PaymentStatus
	switch payment.MapStatus(result.Status) {
	case "confirmed":
		status = domain.PaymentStatusConfirmed
	case "rejected":
		status = domain.PaymentStatusRejected
	default:
		status = domain.PaymentStatusPending
	}
	if err := a.store.UpdatePaymentAfterInit(r.Context(), payID, status, result.PaymentID, result.PaymentURL); err != nil {
		a.logger.Error("update payment after init", "err", err)
	}

	// NB: корзину НЕ очищаем здесь — только после подтверждения оплаты.

	paymentURL := result.PaymentURL
	if paymentURL == "" {
		paymentURL = a.cfg.BaseURL + "/payment/success"
	}

	writeJSON(w, http.StatusOK, struct {
		OrderID    int64  `json:"order_id"`
		PaymentURL string `json:"payment_url"`
	}{OrderID: orderID, PaymentURL: paymentURL})
}

// buildReceipt формирует фискальный чек 54-ФЗ по заказу. Возвращает nil, если у
// заказа нет контактного телефона (без него чек не сформировать). Наименование
// позиции — снимок названия услуги (order_items.title). Используется и при оплате,
// и при возврате (чек возврата).
func (a *App) buildReceipt(order *domain.Order) *payment.Receipt {
	if order.ContactPhone == "" {
		return nil
	}
	items := make([]payment.ReceiptItem, len(order.Items))
	for i, it := range order.Items {
		items[i] = payment.ReceiptItem{
			Name:          it.Title,
			Price:         it.PriceKop,
			Quantity:      float64(it.Qty),
			Amount:        it.PriceKop * int64(it.Qty),
			Tax:           a.cfg.TBankTax,
			PaymentMethod: a.cfg.TBankPaymentMethod,
			PaymentObject: a.cfg.TBankPaymentObject,
		}
	}
	return &payment.Receipt{
		Phone:    order.ContactPhone,
		Items:    items,
		Taxation: a.cfg.TBankTaxation,
	}
}
