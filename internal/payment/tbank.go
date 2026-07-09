package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TBankProvider struct {
	TerminalKey string
	Password    string
	APIBase     string
	HTTPClient  *http.Client
}

func NewTBank(terminalKey, password, apiBase string) *TBankProvider {
	return &TBankProvider{
		TerminalKey: terminalKey,
		Password:    password,
		APIBase:     apiBase,
		HTTPClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TBankProvider) Init(ctx context.Context, p InitParams) (InitResult, error) {
	params := map[string]any{
		"TerminalKey":     t.TerminalKey,
		"Amount":          p.AmountKop,
		"OrderId":         p.OrderID,
		"Description":     p.Description,
		"NotificationURL": p.NotificationURL,
		"SuccessURL":      p.SuccessURL,
		"FailURL":         p.FailURL,
	}

	if rm := buildReceiptMap(p.Receipt); rm != nil {
		params["Receipt"] = rm
	}

	params["Token"] = sign(params, t.Password)

	var resp tbankInitResponse
	if err := t.do(ctx, "/Init", params, &resp); err != nil {
		return InitResult{}, fmt.Errorf("tbank init: %w", err)
	}
	if !resp.Success {
		return InitResult{}, fmt.Errorf("tbank init failed: code=%s msg=%s", resp.ErrorCode, resp.Message)
	}
	return InitResult{
		PaymentID:  resp.PaymentID,
		PaymentURL: resp.PaymentURL,
		Status:     resp.Status,
	}, nil
}

func (t *TBankProvider) GetState(ctx context.Context, paymentID string) (string, error) {
	params := map[string]any{
		"TerminalKey": t.TerminalKey,
		"PaymentId":   paymentID,
	}
	params["Token"] = sign(params, t.Password)

	var resp tbankStateResponse
	if err := t.do(ctx, "/GetState", params, &resp); err != nil {
		return "", fmt.Errorf("tbank getstate: %w", err)
	}
	if !resp.Success {
		return "", fmt.Errorf("tbank getstate failed: code=%s msg=%s", resp.ErrorCode, resp.Message)
	}
	return resp.Status, nil
}

func buildReceiptMap(r *Receipt) map[string]any {
	if r == nil || (r.Email == "" && r.Phone == "") {
		return nil
	}
	receipt := map[string]any{"Taxation": r.Taxation}
	if r.Email != "" {
		receipt["Email"] = r.Email
	}
	if r.Phone != "" {
		receipt["Phone"] = r.Phone
	}
	items := make([]map[string]any, len(r.Items))
	for i, it := range r.Items {
		items[i] = map[string]any{
			"Name":          it.Name,
			"Price":         it.Price,
			"Quantity":      it.Quantity,
			"Amount":        it.Amount,
			"Tax":           it.Tax,
			"PaymentMethod": it.PaymentMethod,
			"PaymentObject": it.PaymentObject,
		}
	}
	receipt["Items"] = items
	return receipt
}

func (t *TBankProvider) Refund(ctx context.Context, paymentID string, amountKop int64, receipt *Receipt) error {
	params := map[string]any{
		"TerminalKey": t.TerminalKey,
		"PaymentId":   paymentID,
		"Amount":      amountKop,
	}
	if rm := buildReceiptMap(receipt); rm != nil {
		params["Receipt"] = rm
	}
	params["Token"] = sign(params, t.Password)

	var resp struct {
		Success   bool   `json:"Success"`
		ErrorCode string `json:"ErrorCode"`
		Message   string `json:"Message"`
	}
	if err := t.do(ctx, "/Refund", params, &resp); err != nil {
		return fmt.Errorf("tbank refund: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("tbank refund failed: code=%s msg=%s", resp.ErrorCode, resp.Message)
	}
	return nil
}

func (t *TBankProvider) Cancel(ctx context.Context, paymentID string) error {
	params := map[string]any{
		"TerminalKey": t.TerminalKey,
		"PaymentId":   paymentID,
	}
	params["Token"] = sign(params, t.Password)

	var resp struct {
		Success   bool   `json:"Success"`
		ErrorCode string `json:"ErrorCode"`
		Message   string `json:"Message"`
	}
	if err := t.do(ctx, "/Cancel", params, &resp); err != nil {
		return fmt.Errorf("tbank cancel: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("tbank cancel failed: code=%s msg=%s", resp.ErrorCode, resp.Message)
	}
	return nil
}

func (t *TBankProvider) do(ctx context.Context, path string, params map[string]any, out any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params: %w", err)
	}

	reqURL := strings.TrimRight(t.APIBase, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

func MapStatus(tbankStatus string) string {
	switch strings.ToUpper(tbankStatus) {
	case "NEW", "FORM_SHOWED", "AUTHORIZING", "3DS_CHECKING", "AUTHORIZED":
		return "pending"
	case "CONFIRMED":
		return "confirmed"
	case "REJECTED", "AUTH_FAIL", "DEADLINE_EXPIRED":
		return "rejected"
	case "REFUNDED", "PARTIAL_REFUNDED":
		return "refunded"
	default:
		return "pending"
	}
}
