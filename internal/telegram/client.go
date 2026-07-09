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

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	return c.do(ctx, "/sendMessage", payload, nil)
}

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

func (c *Client) SendMessageRemoveKeyboard(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": map[string]any{"remove_keyboard": true},
	}
	return c.do(ctx, "/sendMessage", payload, nil)
}

type InlineButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}

type InlineKeyboard [][]InlineButton

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

func (c *Client) AnswerCallbackQuery(ctx context.Context, callbackID string) error {
	return c.do(ctx, "/answerCallbackQuery", map[string]any{"callback_query_id": callbackID}, nil)
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    From     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Message struct {
	From    From     `json:"from"`
	Chat    Chat     `json:"chat"`
	Text    string   `json:"text"`
	Contact *Contact `json:"contact"`
}

type Contact struct {
	PhoneNumber string `json:"phone_number"`
	UserID      int64  `json:"user_id"`
}

type From struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID int64 `json:"id"`
}

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

func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.do(ctx, "/deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

func (c *Client) SetMyCommands(ctx context.Context, cmds []BotCommand) error {
	return c.do(ctx, "/setMyCommands", map[string]any{"commands": cmds}, nil)
}

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

type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram api error %d: %s", e.Code, e.Description)
}

func (e *APIError) IsForbidden() bool {
	return e.Code == 403
}
