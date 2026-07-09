package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"therunish/internal/auth"
	"therunish/internal/usecase"
)

func (a *App) APICartView(w http.ResponseWriter, r *http.Request) {
	resp, err := a.cart.View(r.Context(), auth.UserFromContext(r.Context()))
	if err != nil {
		a.logger.Error("get cart", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) APICartAdd(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "login_required")
		return
	}

	var body struct {
		ServiceID int64 `json:"service_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ServiceID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_service_id")
		return
	}

	resp, err := a.cart.Add(r.Context(), body.ServiceID, user)
	if err != nil {
		if status, code, ok := cartErrCode(err); ok {
			writeJSONError(w, status, code)
			return
		}
		a.logger.Error("add to cart", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) APICartRemove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServiceID int64 `json:"service_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ServiceID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_service_id")
		return
	}

	resp, err := a.cart.Remove(r.Context(), body.ServiceID, auth.UserFromContext(r.Context()))
	if err != nil {
		a.logger.Error("remove from cart", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func cartErrCode(err error) (int, string, bool) {
	switch {
	case errors.Is(err, usecase.ErrNoSession):
		return http.StatusBadRequest, "no_session", true
	case errors.Is(err, usecase.ErrInvalidService):
		return http.StatusBadRequest, "invalid_service_id", true
	case errors.Is(err, usecase.ErrServiceInactive):
		return http.StatusBadRequest, "service_inactive", true
	case errors.Is(err, usecase.ErrSubscriptionAlreadyInCart):
		return http.StatusBadRequest, "subscription_already_in_cart", true
	}
	return 0, "", false
}
