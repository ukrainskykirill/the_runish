package handlers

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"therunish/internal/storage"
)

// paymentMockTmpl — простая страница-эмулятор платёжной формы для PAYMENT_PROVIDER=mock.
var paymentMockTmpl = template.Must(template.New("payment_mock").Parse(`<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Тестовая оплата — The Runish</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f6f1e7; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
  .card { background: #fff; border-radius: 16px; padding: 32px; max-width: 360px; width: 100%; box-shadow: 0 10px 30px rgba(0,0,0,.08); text-align: center; }
  h1 { font-size: 20px; margin: 0 0 8px; }
  .order { color: #888; font-size: 13px; margin-bottom: 16px; }
  .amount { font-size: 32px; font-weight: 700; margin: 0 0 24px; }
  button { width: 100%; padding: 14px; border-radius: 10px; border: none; font-size: 16px; font-weight: 600; cursor: pointer; margin-bottom: 10px; }
  .pay { background: #e8453c; color: #fff; }
  .cancel { background: #eee; color: #333; }
  .note { color: #aaa; font-size: 12px; margin-top: 8px; }
</style>
</head>
<body>
  <div class="card">
    <h1>Тестовая оплата</h1>
    <div class="order">Заказ {{.OrderID}}</div>
    <div class="amount">{{.Amount}}</div>
    <form method="post">
      <button class="pay" type="submit" name="action" value="confirm">Оплатить (эмуляция)</button>
      <button class="cancel" type="submit" name="action" value="cancel">Отменить</button>
    </form>
    <div class="note">PAYMENT_PROVIDER=mock — настоящая оплата не выполняется.</div>
  </div>
</body>
</html>`))

// PaymentMockPage — GET /payment/mock/{orderID}. Эмулятор страницы платёжного провайдера.
func (a *App) PaymentMockPage(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderID")

	pay, err := a.store.GetPaymentByTBankOrderID(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("mock payment: get payment", "err", err, "order_id", orderID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = paymentMockTmpl.Execute(w, struct {
		OrderID string
		Amount  string
	}{
		OrderID: orderID,
		Amount:  fmt.Sprintf("%.2f ₽", float64(pay.AmountKop)/100),
	})
}

// PaymentMockConfirm — POST /payment/mock/{orderID}. Подтверждает или отменяет тестовую оплату
// и редиректит на success/fail, как настоящий провайдер.
func (a *App) PaymentMockConfirm(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderID")

	switch r.FormValue("action") {
	case "confirm":
		if err := a.store.ActivateSubscriptionTx(r.Context(), orderID, "mock-"+orderID, "CONFIRMED"); err != nil {
			a.logger.Error("mock payment: activate subscription", "err", err, "order_id", orderID)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, a.cfg.BaseURL+"/payment/success", http.StatusSeeOther)
	case "cancel":
		if err := a.store.RejectPaymentTx(r.Context(), orderID, "mock-"+orderID, "REJECTED", "mock_cancel"); err != nil {
			a.logger.Error("mock payment: reject payment", "err", err, "order_id", orderID)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, a.cfg.BaseURL+"/payment/fail", http.StatusSeeOther)
	default:
		http.Error(w, "invalid action", http.StatusBadRequest)
	}
}
