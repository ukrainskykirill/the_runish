package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"therunish/internal/auth"
	"therunish/internal/storage"
)

// cartLine — позиция корзины с резолвленными ценой/названием.
type cartLine struct {
	ServiceID int64  `json:"service_id"`
	Title     string `json:"title"`
	PriceKop  int64  `json:"price_kop"`
	Qty       int    `json:"qty"`
	Subtotal  int64  `json:"subtotal"`
}

type cartResponse struct {
	Lines []cartLine `json:"lines"`
	Total int64      `json:"total"`
	Count int        `json:"count"`
}

// buildCartResponse резолвит корзину текущей сессии в cartResponse.
func (a *App) buildCartResponse(r *http.Request) (cartResponse, error) {
	resp := cartResponse{Lines: []cartLine{}}

	sessionID := a.sessions.SessionID(r)
	if sessionID == "" {
		return resp, nil
	}

	cart, err := a.store.GetCart(r.Context(), sessionID)
	if err != nil {
		return resp, err
	}
	if len(cart) == 0 {
		return resp, nil
	}

	ids := make([]int64, len(cart))
	for i, it := range cart {
		ids[i] = it.ServiceID
	}
	services, err := a.store.ListServicesByIDs(r.Context(), ids)
	if err != nil {
		return resp, err
	}

	// Эффективная цена (скидка «при подписке» / акция) — как в каталоге и checkout.
	pc := a.pricingContext(r.Context(), auth.UserFromContext(r.Context()))

	for _, it := range cart {
		svc, ok := services[it.ServiceID]
		if !ok {
			continue
		}
		pc.apply(&svc)
		sub := svc.EffectivePriceKop * int64(it.Qty)
		resp.Lines = append(resp.Lines, cartLine{
			ServiceID: svc.ID,
			Title:     svc.Title,
			PriceKop:  svc.EffectivePriceKop,
			Qty:       it.Qty,
			Subtotal:  sub,
		})
		resp.Total += sub
		resp.Count += it.Qty
	}
	return resp, nil
}

// APICartView — GET /api/cart
func (a *App) APICartView(w http.ResponseWriter, r *http.Request) {
	resp, err := a.buildCartResponse(r)
	if err != nil {
		a.logger.Error("get cart", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// APICartAdd — POST /api/cart {"service_id": 1}
func (a *App) APICartAdd(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "login_required")
		return
	}

	sessionID := a.sessions.SessionID(r)
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "no_session")
		return
	}

	var body struct {
		ServiceID int64 `json:"service_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ServiceID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_service_id")
		return
	}

	// Резолвим добавляемую услугу.
	newSvc, err := a.store.GetService(r.Context(), body.ServiceID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSONError(w, http.StatusBadRequest, "invalid_service_id")
			return
		}
		a.logger.Error("get service for cart", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if !newSvc.IsActive {
		writeJSONError(w, http.StatusBadRequest, "service_inactive")
		return
	}

	// Подписочный товар (подписка/бандл) — в корзине допустим только один:
	// нельзя две подписки и нельзя бандл + подписку одновременно.
	if newSvc.Kind.GrantsSubscription() {
		cart, err := a.store.GetCart(r.Context(), sessionID)
		if err != nil {
			a.logger.Error("get cart", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		for _, it := range cart {
			if it.ServiceID == newSvc.ID {
				continue // тот же товар — qty всё равно останется 1
			}
			existing, err := a.store.GetService(r.Context(), it.ServiceID)
			if err != nil {
				continue
			}
			if existing.Kind.GrantsSubscription() {
				writeJSONError(w, http.StatusBadRequest, "subscription_already_in_cart")
				return
			}
		}
	}

	if err := a.store.AddToCart(r.Context(), sessionID, body.ServiceID); err != nil {
		a.logger.Error("add to cart", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	resp, err := a.buildCartResponse(r)
	if err != nil {
		a.logger.Error("get cart", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// APICartRemove — POST /api/cart/remove {"service_id": 1}
func (a *App) APICartRemove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServiceID int64 `json:"service_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ServiceID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_service_id")
		return
	}

	sessionID := a.sessions.SessionID(r)
	if sessionID != "" {
		if err := a.store.RemoveFromCart(r.Context(), sessionID, body.ServiceID); err != nil {
			a.logger.Error("remove from cart", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal_error")
			return
		}
	}

	resp, err := a.buildCartResponse(r)
	if err != nil {
		a.logger.Error("get cart", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
