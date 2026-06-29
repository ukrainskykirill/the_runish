package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client — минимальный клиент Telegram Bot API на голом http.Client.
type Client struct {
	token   string
	apiBase string
	http    *http.Client
}

func New(token string) *Client {
	return &Client{
		token:   token,
		apiBase: "https://api.telegram.org",
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// SendMessage отправляет текстовое сообщение.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	return c.do(ctx, "/sendMessage", payload, nil)
}

// SendMessageWithContactButton отправляет сообщение с reply-кнопкой «Поделиться телефоном»
// (request_contact). По нажатию Telegram присылает контакт пользователя в message.contact.
func (c *Client) SendMessageWithContactButton(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]any{
			"keyboard": [][]map[string]any{
				{{"text": "📱 Поделиться телефоном", "request_contact": true}},
			},
			"resize_keyboard":   true,
			"one_time_keyboard": true,
		},
	}
	return c.do(ctx, "/sendMessage", payload, nil)
}

// SendMessageRemoveKeyboard отправляет сообщение и убирает reply-клавиатуру.
func (c *Client) SendMessageRemoveKeyboard(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": map[string]any{"remove_keyboard": true},
	}
	return c.do(ctx, "/sendMessage", payload, nil)
}

// InlineButton — кнопка инлайн-клавиатуры.
type InlineButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

// InlineKeyboard — раскладка инлайн-кнопок (строки × кнопки).
type InlineKeyboard [][]InlineButton

// SendMessageWithKeyboard отправляет сообщение с инлайн-клавиатурой и возвращает message_id.
func (c *Client) SendMessageWithKeyboard(ctx context.Context, chatID int64, text string, kb InlineKeyboard) (int64, error) {
	payload := map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": map[string]any{"inline_keyboard": kb},
	}
	var out struct {
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := c.do(ctx, "/sendMessage", payload, &out); err != nil {
		return 0, err
	}
	return out.Result.MessageID, nil
}

// EditMessageText редактирует текст и клавиатуру ранее отправленного сообщения.
// Пустой kb убирает кнопки (например, после выбора ответа).
func (c *Client) EditMessageText(ctx context.Context, chatID, messageID int64, text string, kb InlineKeyboard) error {
	if kb == nil {
		kb = InlineKeyboard{}
	}
	payload := map[string]any{
		"chat_id":      chatID,
		"message_id":   messageID,
		"text":         text,
		"reply_markup": map[string]any{"inline_keyboard": kb},
	}
	return c.do(ctx, "/editMessageText", payload, nil)
}

// AnswerCallbackQuery подтверждает нажатие инлайн-кнопки (убирает «часики»).
func (c *Client) AnswerCallbackQuery(ctx context.Context, callbackID string) error {
	return c.do(ctx, "/answerCallbackQuery", map[string]any{"callback_query_id": callbackID}, nil)
}

// Update — входящее обновление от getUpdates (сообщения и нажатия инлайн-кнопок).
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

// CallbackQuery — нажатие на инлайн-кнопку.
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    From     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

// Message — сообщение от пользователя боту.
type Message struct {
	From    From     `json:"from"`
	Chat    Chat     `json:"chat"`
	Text    string   `json:"text"`
	Contact *Contact `json:"contact"`
}

// Contact — контакт, которым поделился пользователь (кнопка request_contact).
type Contact struct {
	PhoneNumber string `json:"phone_number"`
	UserID      int64  `json:"user_id"` // id владельца контакта (для проверки «свой ли номер»)
}

// From — отправитель сообщения.
type From struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Chat — чат, в котором получено сообщение.
type Chat struct {
	ID int64 `json:"id"`
}

// GetUpdates — короткий поллинг новых обновлений начиная с offset (без long-polling
// задержки на стороне Telegram, чтобы не упираться в таймаут http.Client).
func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	payload := map[string]any{
		"offset":          offset,
		"timeout":         0,
		"allowed_updates": []string{"message", "callback_query"},
	}
	var out struct {
		Result []Update `json:"result"`
	}
	if err := c.do(ctx, "/getUpdates", payload, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

// DeleteWebhook снимает webhook, если он был установлен — иначе getUpdates вернёт ошибку 409.
func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.do(ctx, "/deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}

// BotCommand — пункт меню команд бота (кнопка «/» в клиенте Telegram).
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// SetMyCommands регистрирует список команд, который Telegram показывает в меню бота.
func (c *Client) SetMyCommands(ctx context.Context, cmds []BotCommand) error {
	return c.do(ctx, "/setMyCommands", map[string]any{"commands": cmds}, nil)
}

// apiResponse — базовый ответ Telegram API.
type apiResponse struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

func (c *Client) do(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s%s", c.apiBase, c.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	if !apiResp.OK {
		return &APIError{Code: apiResp.ErrorCode, Description: apiResp.Description}
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// APIError — ошибка Telegram API (например 403 Forbidden — юзер заблокировал бота).
type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram api error %d: %s", e.Code, e.Description)
}

// IsForbidden возвращает true, если юзер заблокировал бота (403).
func (e *APIError) IsForbidden() bool {
	return e.Code == 403
}
