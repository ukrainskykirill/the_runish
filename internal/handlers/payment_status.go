package handlers

import (
	"errors"
	"net/http"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

// APIPaymentStatus reports the coarse status of a payment by its opaque T-Bank order id.
// It is intentionally session-less: after a Mini App checkout, Telegram.WebApp.openLink opens
// the T-Bank page (and its success/fail redirect) in an EXTERNAL browser that has no
// runish_session cookie, so the payment-result page there cannot use /api/me. The order id is
// carried back in the success/fail URL (see checkout Start), and the real status is set by the
// webhook regardless of session. The order id (o<orderID>-<UnixNano>) is effectively unguessable,
// and only a coarse status is exposed.
func (a *App) APIPaymentStatus(w http.ResponseWriter, r *http.Request) {
	order := r.URL.Query().Get("order")
	if order == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_order")
		return
	}

	pay, err := a.store.GetPaymentByTBankOrderID(r.Context(), order)
	if errors.Is(err, storage.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		a.logger.Error("payment status lookup", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": paymentResultStatus(pay.Status)})
}

func paymentResultStatus(s domain.PaymentStatus) string {
	switch s {
	case domain.PaymentStatusConfirmed:
		return "paid"
	case domain.PaymentStatusRejected, domain.PaymentStatusCancelled:
		return "failed"
	case domain.PaymentStatusRefunded:
		return "refunded"
	default:
		return "pending"
	}
}
