package handlers

import (
	"net/http"
)

func (a *App) PaymentSuccess(w http.ResponseWriter, r *http.Request) {
	a.serveSPA(w, r)
}

func (a *App) PaymentFail(w http.ResponseWriter, r *http.Request) {
	a.serveSPA(w, r)
}
