package handlers

import (
	"errors"
	"net/http"

	"therunish/internal/domain"
	"therunish/internal/payment"
	"therunish/internal/storage"
)

func (a *App) AdminRefundPaymentSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	p, err := a.store.GetPaymentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get payment for refund", "err", err, "id", id)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if p.Status != domain.PaymentStatusConfirmed {
		http.Error(w, "Можно вернуть только подтверждённый платёж", http.StatusBadRequest)
		return
	}
	if p.TBankPaymentID == "" {
		http.Error(w, "У платежа нет T-Bank PaymentId", http.StatusBadRequest)
		return
	}

	var refundReceipt *payment.Receipt
	if order, err := a.store.GetOrderByID(r.Context(), p.OrderID); err != nil {
		a.logger.Error("get order for refund receipt", "err", err, "order_id", p.OrderID)
	} else {
		refundReceipt = a.checkout.BuildReceipt(&order)
	}

	if err := a.provider.Refund(r.Context(), p.TBankPaymentID, p.AmountKop, refundReceipt); err != nil {
		a.logger.Error("provider refund", "err", err, "payment_id", id)
		http.Error(w, "Ошибка возврата: "+err.Error(), http.StatusBadGateway)
		return
	}

	a.logger.Info("refund initiated", "payment_id", id, "amount_kop", p.AmountKop, "admin", r.Header.Get("X-Admin-Token"))
	http.Redirect(w, r, "/admin/payments", http.StatusSeeOther)
}

func (a *App) AdminDashboardPage(w http.ResponseWriter, r *http.Request) {
	dash, err := a.store.GetPaymentDashboard(r.Context())
	if err != nil {
		a.logger.Error("payment dashboard", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	regs, err := a.store.GetRegistrationDashboard(r.Context())
	if err != nil {
		a.logger.Error("registration dashboard", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Dashboard     storage.PaymentDashboard
		Registrations storage.RegistrationDashboard
	}{
		PageData:      PageData{BotUsername: a.cfg.BotUsername},
		Dashboard:     dash,
		Registrations: regs,
	}
	if err := a.renderer.Render(w, "admin_dashboard", data); err != nil {
		a.logger.Error("render admin_dashboard", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminListPaymentsPage(w http.ResponseWriter, r *http.Request) {
	payments, err := a.store.ListRecentPayments(r.Context(), 50)
	if err != nil {
		a.logger.Error("list payments", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Payments []storage.PaymentWithDetails
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Payments: payments,
	}
	if err := a.renderer.Render(w, "admin_payments", data); err != nil {
		a.logger.Error("render admin_payments", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
