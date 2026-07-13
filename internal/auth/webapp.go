package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const WebAppInitDataMaxAge = 24 * time.Hour

type WebAppUser struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
}

type WebAppData struct {
	User       WebAppUser
	StartParam string
	AuthDate   time.Time
}

// ValidateWebAppInitData validates Telegram Mini App initData per
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app.
// Note: the secret derivation differs from the Login Widget (ValidateTelegramAuth):
// here secret = HMAC_SHA256(botToken, "WebAppData"), not sha256(botToken).
func ValidateWebAppInitData(initData string, botToken string) (WebAppData, error) {
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return WebAppData{}, ErrMissingFields
	}

	hash := vals.Get("hash")
	userRaw := vals.Get("user")
	authDateStr := vals.Get("auth_date")
	if hash == "" || userRaw == "" || authDateStr == "" {
		return WebAppData{}, ErrMissingFields
	}

	if err := verifyWebAppHash(vals, hash, botToken); err != nil {
		return WebAppData{}, err
	}

	authDateUnix, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return WebAppData{}, ErrMissingFields
	}
	authDate := time.Unix(authDateUnix, 0)
	if time.Since(authDate) > WebAppInitDataMaxAge {
		return WebAppData{}, ErrExpired
	}

	var u struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	}
	if err := json.Unmarshal([]byte(userRaw), &u); err != nil {
		return WebAppData{}, ErrMissingFields
	}
	if u.ID == 0 {
		return WebAppData{}, ErrMissingFields
	}

	return WebAppData{
		User: WebAppUser{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Username:  u.Username,
		},
		StartParam: vals.Get("start_param"),
		AuthDate:   authDate,
	}, nil
}

// Note on phone collection: Telegram.WebApp.requestContact() does NOT return the
// phone number to the browser — its callback only reports a boolean (shared or not).
// The contact itself is delivered to the bot as a regular Bot API update (a Message
// with a Contact), the same way as the existing reply-keyboard "share phone" button.
// It is handled by the existing internal/botworker/telegram.go handleContact — no
// separate WebApp-side validation/endpoint is needed for phone numbers.

func verifyWebAppHash(vals url.Values, hash, botToken string) error {
	keys := make([]string, 0, len(vals))
	for k := range vals {
		if k == "hash" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+vals.Get(k))
	}
	dataCheckString := strings.Join(parts, "\n")

	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	secretMac.Write([]byte(botToken))
	secretKey := secretMac.Sum(nil)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dataCheckString))
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedHash), []byte(hash)) {
		return ErrInvalidHash
	}
	return nil
}
