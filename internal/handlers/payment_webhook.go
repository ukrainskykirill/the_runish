package handlers

import (
	"io"
	"net/http"

	"therunish/internal/payment"
)

// PaymentWebhook — приём Notification от T-Bank.
// Без авторизации — валидация по Token.
func (a *App) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.logger.Error("read webhook body", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Парсим и валидируем подпись по всем полям уведомления.
	notif, ok, err := payment.ValidateNotification(body, a.cfg.TBankPassword)
	if err != nil {
		a.logger.Error("validate notification", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !ok {
		a.logger.Warn("invalid webhook token", "order_id", notif.OrderID)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	mapped := payment.MapStatus(notif.Status)
	a.logger.Info("webhook received",
		"order_id", notif.OrderID,
		"tbank_status", notif.Status,
		"mapped", mapped,
	)

	switch mapped {
	case "confirmed":
		if err := a.store.ActivateSubscriptionTx(r.Context(), notif.OrderID, notif.PaymentID.String(), notif.Status); err != nil {
			a.logger.Error("activate subscription", "err", err, "order_id", notif.OrderID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "rejected":
		if err := a.store.RejectPaymentTx(r.Context(), notif.OrderID, notif.PaymentID.String(), notif.Status, notif.ErrorCode); err != nil {
			a.logger.Error("reject payment", "err", err, "order_id", notif.OrderID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "refunded":
		// REFUNDED (полный) → отмена платежа/заказа/подписок.
		// PARTIAL_REFUNDED (частичный) → только фиксируем tbank_status.
		if err := a.store.RefundPaymentTx(r.Context(), notif.OrderID, notif.PaymentID.String(), notif.Status); err != nil {
			a.logger.Error("refund payment", "err", err, "order_id", notif.OrderID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// T-Bank требует ровно HTTP 200 с телом "OK".
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
