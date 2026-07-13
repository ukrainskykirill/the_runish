package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"therunish/internal/config"
	"therunish/internal/domain"
	"therunish/internal/payment"
	"therunish/internal/storage"
)

type Checkout struct {
	cfg      config.Config
	store    *storage.Store
	cart     *Cart
	provider payment.PaymentProvider
	logger   *slog.Logger
}

type CheckoutResult struct {
	OrderID    int64  `json:"order_id"`
	PaymentURL string `json:"payment_url"`
}

func NewCheckout(cfg config.Config, store *storage.Store, cart *Cart, provider payment.PaymentProvider, logger *slog.Logger) *Checkout {
	return &Checkout{
		cfg:      cfg,
		store:    store,
		cart:     cart,
		provider: provider,
		logger:   logger,
	}
}

func (c *Checkout) Start(ctx context.Context, user *domain.User) (CheckoutResult, error) {
	if user.Phone == "" {
		return CheckoutResult{}, ErrPhoneRequired
	}

	items, total, err := c.cart.checkoutItems(ctx, user)
	if err != nil {
		return CheckoutResult{}, err
	}

	staleTBankIDs, err := c.store.CancelStalePendingPayments(ctx, user.ID)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("cancel stale pending: %w", err)
	}
	for _, pid := range staleTBankIDs {
		if cerr := c.provider.Cancel(ctx, pid); cerr != nil {
			c.logger.Warn("cancel stale tbank payment", "err", cerr, "tbank_payment_id", pid)
		}
	}

	order := &domain.Order{
		UserID:       user.ID,
		AmountKop:    total,
		Status:       domain.OrderStatusCreated,
		ContactPhone: user.Phone,
		ContactName:  user.FullName,
		ContactTg:    user.Username,
		Items:        items,
	}
	orderID, err := c.store.CreateOrder(ctx, order)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("create order: %w", err)
	}

	tbankOrderID := fmt.Sprintf("o%d-%d", orderID, time.Now().UnixNano())
	pay := &domain.Payment{
		UserID:       user.ID,
		OrderID:      orderID,
		AmountKop:    total,
		Status:       domain.PaymentStatusNew,
		Provider:     c.cfg.PaymentProvider,
		TBankOrderID: tbankOrderID,
	}
	payID, err := c.store.CreatePayment(ctx, pay)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("create payment: %w", err)
	}

	initParams := payment.InitParams{
		OrderID:         tbankOrderID,
		AmountKop:       total,
		Description:     fmt.Sprintf("Заказ #%d — The Runish", orderID),
		NotificationURL: c.cfg.BaseURL + "/payment/webhook",
		SuccessURL:      c.cfg.BaseURL + "/payment/success?order=" + url.QueryEscape(tbankOrderID),
		FailURL:         c.cfg.BaseURL + "/payment/fail?order=" + url.QueryEscape(tbankOrderID),
	}

	if receipt := c.BuildReceipt(order); receipt != nil {
		initParams.Receipt = receipt
	} else {
		c.logger.Warn("receipt skipped: no contact phone", "order_id", orderID, "user_id", user.ID)
	}

	result, err := c.provider.Init(ctx, initParams)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("%w: %v", ErrPaymentInitFailed, err)
	}

	mappedStatus := payment.MapStatus(result.Status)
	switch mappedStatus {
	case "confirmed":
		if err := c.store.ActivateSubscriptionTx(ctx, tbankOrderID, result.PaymentID, result.Status); err != nil {
			return CheckoutResult{}, fmt.Errorf("activate confirmed payment: %w", err)
		}
	case "rejected":
		if err := c.store.RejectPaymentTx(ctx, tbankOrderID, result.PaymentID, result.Status, ""); err != nil {
			return CheckoutResult{}, fmt.Errorf("reject payment after init: %w", err)
		}
	default:
		if err := c.store.UpdatePaymentAfterInit(ctx, payID, domain.PaymentStatusPending, result.PaymentID, result.PaymentURL); err != nil {
			c.logger.Error("update payment after init", "err", err)
		}
	}

	paymentURL := result.PaymentURL
	if paymentURL == "" {
		if mappedStatus == "confirmed" {
			return CheckoutResult{OrderID: orderID, PaymentURL: c.cfg.BaseURL + "/payment/success"}, nil
		}
		return CheckoutResult{}, fmt.Errorf("%w: empty payment url", ErrPaymentInitFailed)
	}
	return CheckoutResult{OrderID: orderID, PaymentURL: paymentURL}, nil
}

func (c *Checkout) BuildReceipt(order *domain.Order) *payment.Receipt {
	if order.ContactPhone == "" {
		return nil
	}
	items := make([]payment.ReceiptItem, len(order.Items))
	for i, it := range order.Items {
		items[i] = payment.ReceiptItem{
			Name:          it.Title,
			Price:         it.PriceKop,
			Quantity:      float64(it.Qty),
			Amount:        it.PriceKop * int64(it.Qty),
			Tax:           c.cfg.TBankTax,
			PaymentMethod: c.cfg.TBankPaymentMethod,
			PaymentObject: c.cfg.TBankPaymentObject,
		}
	}
	return &payment.Receipt{
		Phone:    order.ContactPhone,
		Items:    items,
		Taxation: c.cfg.TBankTaxation,
	}
}

func IsPaymentInitFailed(err error) bool {
	return errors.Is(err, ErrPaymentInitFailed)
}
