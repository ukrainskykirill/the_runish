package handlers

import (
	"io"
	"net/http"

	"therunish/internal/observability"
	"therunish/internal/payment"
)

func (a *App) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.logger.Error("read webhook body", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

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
			observability.Alert(r.Context(), a.logger, "Оплата: не удалось активировать подписку по вебхуку", err, "order_id", notif.OrderID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "rejected":
		if err := a.store.RejectPaymentTx(r.Context(), notif.OrderID, notif.PaymentID.String(), notif.Status, notif.ErrorCode); err != nil {
			observability.Alert(r.Context(), a.logger, "Оплата: не удалось обработать отклонённый платёж", err, "order_id", notif.OrderID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "refunded":
		if err := a.store.RefundPaymentTx(r.Context(), notif.OrderID, notif.PaymentID.String(), notif.Status, notif.Amount); err != nil {
			observability.Alert(r.Context(), a.logger, "Оплата: не удалось обработать возврат", err, "order_id", notif.OrderID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
