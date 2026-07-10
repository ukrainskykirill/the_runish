package handlers

import (
	"encoding/json"
	"net/http"

	"therunish/internal/auth"
)

func (a *App) APIMeSetPhone(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "login_required")
		return
	}

	var body struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	phone, ok := auth.NormalizeRuPhone(body.Phone)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_phone")
		return
	}

	if err := a.store.SetUserPhone(r.Context(), user.ID, phone); err != nil {
		a.logger.Error("set user phone", "err", err, "user_id", user.ID)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"phone": phone})
}
