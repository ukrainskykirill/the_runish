package handlers

import (
	"net/http"
)

// PaymentSuccess — возврат при успешной оплате (SuccessURL).
// Очищаем корзину — заказ уже создан в APICheckout. Затем отдаём SPA shell,
// React Router отрисует экран результата по пути /payment/success.
func (a *App) PaymentSuccess(w http.ResponseWriter, r *http.Request) {
	sessionID := a.sessions.SessionID(r)
	_ = a.store.ClearCart(r.Context(), sessionID)
	a.serveSPA(w, r)
}

// PaymentFail — возврат при отказе (FailURL).
func (a *App) PaymentFail(w http.ResponseWriter, r *http.Request) {
	a.serveSPA(w, r)
}
