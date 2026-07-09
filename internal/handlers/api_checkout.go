package handlers

import (
	"errors"
	"net/http"

	"therunish/internal/auth"
	"therunish/internal/observability"
	"therunish/internal/usecase"
)

func (a *App) APICheckout(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "login_required")
		return
	}

	result, err := a.checkout.Start(r.Context(), user)
	if err != nil {
		if status, code, ok := checkoutErrCode(err); ok {
			if usecase.IsPaymentInitFailed(err) {
				observability.Alert(r.Context(), a.logger, "Оплата не прошла: не удалось создать платёж", err, "user_id", user.ID)
			}
			writeJSONError(w, status, code)
			return
		}
		observability.Alert(r.Context(), a.logger, "Оформление оплаты: внутренняя ошибка", err, "user_id", user.ID)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func checkoutErrCode(err error) (int, string, bool) {
	switch {
	case errors.Is(err, usecase.ErrPhoneRequired):
		return http.StatusBadRequest, "phone_required", true
	case errors.Is(err, usecase.ErrCartEmpty):
		return http.StatusBadRequest, "cart_empty", true
	case errors.Is(err, usecase.ErrCartInvalid):
		return http.StatusBadRequest, "cart_invalid", true
	case errors.Is(err, usecase.ErrMultipleSubscriptions):
		return http.StatusBadRequest, "multiple_subscriptions", true
	case errors.Is(err, usecase.ErrEntryFeeRequired):
		return http.StatusBadRequest, "entry_fee_required", true
	case errors.Is(err, usecase.ErrAlreadyOwned):
		return http.StatusBadRequest, "already_owned", true
	case usecase.IsPaymentInitFailed(err):
		return http.StatusBadGateway, "payment_init_failed", true
	}
	return 0, "", false
}
