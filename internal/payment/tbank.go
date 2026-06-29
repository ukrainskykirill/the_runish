package payment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TBankProvider — реализация PaymentProvider для T-Bank эквайринга.
// Документация: developer.tbank.ru/eacq/api
type TBankProvider struct {
	TerminalKey string
	Password    string
	APIBase     string // https://securepay.tinkoff.ru/v2
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

// tbankInitResponse — ответ от /Init.
type tbankInitResponse struct {
	Success    bool   `json:"Success"`
	ErrorCode  string `json:"ErrorCode"`
	Message    string `json:"Message"`
	PaymentID  string `json:"PaymentId"`
	PaymentURL string `json:"PaymentURL"`
	Status     string `json:"Status"`
	OrderID    string `json:"OrderId"`
}

// tbankStateResponse — ответ от /GetState.
type tbankStateResponse struct {
	Success   bool   `json:"Success"`
	ErrorCode string `json:"ErrorCode"`
	Message   string `json:"Message"`
	PaymentID string `json:"PaymentId"`
	Status    string `json:"Status"`
	OrderID   string `json:"OrderId"`
}

// Init — POST /Init с подписью Token.
func (t *TBankProvider) Init(ctx context.Context, p InitParams) (InitResult, error) {
	params := map[string]any{
		"TerminalKey":     t.TerminalKey,
		"Amount":          p.AmountKop, // int64 → уйдёт как JSON-число, не строка
		"OrderId":         p.OrderID,
		"Description":     p.Description,
		"NotificationURL": p.NotificationURL,
		"SuccessURL":      p.SuccessURL,
		"FailURL":         p.FailURL,
	}

	// Фискальный чек 54-ФЗ (чек прихода): передаём, только если есть Email или Phone.
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

// GetState — POST /GetState для подстраховки (webhook мог потеряться).
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

// buildReceiptMap собирает объект Receipt для /Init и /Refund по правилам T-Bank.
// Возвращает nil, если чек не нужен (нет ни Email, ни Phone). Receipt исключается
// из расчёта Token (см. sign) как вложенный объект.
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

// Refund — POST /Refund для возврата платежа (полного или частичного).
// amountKop должен быть <= сумме исходного платежа. receipt — чек возврата (54-ФЗ):
// если исходная оплата фискализировалась, возврат тоже должен формировать чек.
// Возврат асинхронен: финальный статус приходит вебхуком REFUNDED/PARTIAL_REFUNDED.
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

// Cancel — POST /Cancel для отмены платежа.
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

// Notification — типизированные поля из webhook T-Bank.
// Т-Bank присылает и другие поля (Amount, CardId, Pan и т.д.),
// но для бизнес-логики нужны только эти.
type Notification struct {
	TerminalKey string `json:"TerminalKey"`
	OrderID     string `json:"OrderId"`
	Success     bool   `json:"Success"`
	Status      string `json:"Status"`
	PaymentID   flexID `json:"PaymentId"`
	ErrorCode   string `json:"ErrorCode"`
}

// flexID — идентификатор платежа, который T-Bank в ответе /Init отдаёт строкой
// ("8742344788"), а в webhook-уведомлении присылает числом (8742344788 без кавычек).
// Принимаем оба варианта и храним как строку.
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	*f = flexID(strings.Trim(string(b), `"`))
	return nil
}

// String возвращает идентификатор как строку.
func (f flexID) String() string { return string(f) }

// ValidateNotification парсит тело webhook'а и проверяет подпись Token.
// Token считается по ВСЕМ полям уведомления (кроме Token и вложенных объектов):
// добавляется Password, ключи сортируются, значения конкатенируются, SHA-256 → hex.
// Возвращает (notification, true, nil) при успехе, (notification, false, nil) при
// несовпадении подписи, или (_, false, err) при ошибке парсинга.
func ValidateNotification(rawBody []byte, password string) (Notification, bool, error) {
	var raw map[string]any
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return Notification{}, false, fmt.Errorf("decode notification: %w", err)
	}

	token, ok := raw["Token"].(string)
	if !ok || token == "" {
		return Notification{}, false, fmt.Errorf("missing Token in notification")
	}

	// Оставляем только скалярные поля (без Token, без вложенных объектов/массивов).
	filtered := make(map[string]any, len(raw))
	for k, v := range raw {
		if k == "Token" {
			continue
		}
		if isScalar(v) {
			filtered[k] = v
		}
	}

	expected := sign(filtered, password) // sign добавит Password автоматически
	if !safeEqual(expected, token) {
		return Notification{}, false, nil
	}

	var notif Notification
	if err := json.Unmarshal(rawBody, &notif); err != nil {
		return Notification{}, false, fmt.Errorf("unmarshal notification fields: %w", err)
	}
	return notif, true, nil
}

// isScalar возвращает true для примитивных JSON-значений (не object/array).
// Вложенные структуры исключаются из расчёта подписи по правилам T-Bank.
func isScalar(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return false
	default:
		return true
	}
}

// safeEqual — сравнение строк constant-time (защита от timing-атак).
func safeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var r byte
	for i := 0; i < len(a); i++ {
		r |= a[i] ^ b[i]
	}
	return r == 0
}

// do отправляет POST JSON и парсит ответ.
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

// sign считает Token по правилам T-Bank:
// берём все параметры (кроме Token и Password), добавляем Password,
// сортируем по ключу, конкатенируем значения, SHA-256 → hex lowercase.
func sign(params map[string]any, password string) string {
	cp := make(map[string]any, len(params)+1)
	for k, v := range params {
		if k == "Token" || k == "Password" {
			continue
		}
		cp[k] = v
	}
	cp["Password"] = password

	keys := make([]string, 0, len(cp))
	for k := range cp {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		v := cp[k]
		// Вложенные объекты и массивы (Receipt, Data, Items) исключаются из Token
		// по правилам T-Bank — подпись считается только по скалярным полям.
		if !isScalar(v) {
			continue
		}
		sb.WriteString(anyToSignString(v))
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// anyToSignString преобразует значение из JSON-декода в строку для подписи.
// JSON-числа приходят как float64, bool — как Go bool.
func anyToSignString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		// Целые числа без десятичной точки (T-Bank ожидает "100", не "100.00").
		return strconv.FormatFloat(val, 'f', -1, 64)
	case json.Number:
		return val.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// MapStatus переводит сырой статус T-Bank в наш domain.PaymentStatus.
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
