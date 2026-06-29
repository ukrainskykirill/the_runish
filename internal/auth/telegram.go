package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// TelegramLoginMaxAge — окно валидности данных Login Widget (защита от replay).
	TelegramLoginMaxAge = 24 * time.Hour
)

var (
	ErrInvalidHash   = errors.New("invalid telegram hash")
	ErrExpired       = errors.New("telegram auth data expired")
	ErrMissingFields = errors.New("missing required fields")
)

// TelegramAuthData — распарсенные данные от Login Widget.
type TelegramAuthData struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	PhotoURL  string
	AuthDate  time.Time
	Hash      string
	Raw       url.Values // все оригинальные параметры
}

// ParseTelegramAuth парсит query-параметры от Login Widget.
func ParseTelegramAuth(vals url.Values) (TelegramAuthData, error) {
	data := TelegramAuthData{Raw: vals}

	idStr := vals.Get("id")
	if idStr == "" || vals.Get("hash") == "" || vals.Get("auth_date") == "" {
		return data, ErrMissingFields
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return data, fmt.Errorf("parse id: %w", err)
	}
	data.ID = id

	authDateUnix, err := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	if err != nil {
		return data, fmt.Errorf("parse auth_date: %w", err)
	}
	data.AuthDate = time.Unix(authDateUnix, 0)

	data.FirstName = vals.Get("first_name")
	data.LastName = vals.Get("last_name")
	data.Username = vals.Get("username")
	data.PhotoURL = vals.Get("photo_url")
	data.Hash = vals.Get("hash")

	return data, nil
}

// ValidateTelegramAuth проверяет подпись HMAC-SHA256 и актуальность auth_date.
// Секрет — это SHA256(BOT_TOKEN).
func ValidateTelegramAuth(data TelegramAuthData, botToken string) error {
	// Проверяем, что данные не протухли.
	if time.Since(data.AuthDate) > TelegramLoginMaxAge {
		return ErrExpired
	}

	secret := sha256.Sum256([]byte(botToken))

	// data_check_string: key=value (кроме hash), отсортированные, через \n.
	keys := make([]string, 0, len(data.Raw))
	for k := range data.Raw {
		if k == "hash" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+data.Raw.Get(k))
	}
	dataCheckString := strings.Join(parts, "\n")

	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(dataCheckString))
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedHash), []byte(data.Hash)) {
		return ErrInvalidHash
	}
	return nil
}
