package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:TEST-BOT-TOKEN"

func signWebAppParams(t *testing.T, params map[string]string, botToken string) string {
	t.Helper()

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	dataCheckString := strings.Join(parts, "\n")

	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	secretMac.Write([]byte(botToken))
	secretKey := secretMac.Sum(nil)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dataCheckString))
	hash := hex.EncodeToString(mac.Sum(nil))

	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	vals.Set("hash", hash)
	return vals.Encode()
}

func TestValidateWebAppInitData_OK(t *testing.T) {
	params := map[string]string{
		"auth_date":   strconv.FormatInt(time.Now().Unix(), 10),
		"user":        `{"id":42,"first_name":"Alex","last_name":"Runner","username":"runner_alex"}`,
		"start_param": "abc123nonce",
	}
	initData := signWebAppParams(t, params, testBotToken)

	data, err := ValidateWebAppInitData(initData, testBotToken)
	if err != nil {
		t.Fatalf("ValidateWebAppInitData() error = %v", err)
	}
	if data.User.ID != 42 || data.User.Username != "runner_alex" {
		t.Errorf("unexpected user: %+v", data.User)
	}
	if data.StartParam != "abc123nonce" {
		t.Errorf("StartParam = %q, want abc123nonce", data.StartParam)
	}
}

func TestValidateWebAppInitData_WrongToken(t *testing.T) {
	params := map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"user":      `{"id":42,"first_name":"Alex"}`,
	}
	initData := signWebAppParams(t, params, testBotToken)

	if _, err := ValidateWebAppInitData(initData, "999999:OTHER-TOKEN"); err != ErrInvalidHash {
		t.Errorf("err = %v, want ErrInvalidHash", err)
	}
}

func TestValidateWebAppInitData_Expired(t *testing.T) {
	params := map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Add(-25*time.Hour).Unix(), 10),
		"user":      `{"id":42,"first_name":"Alex"}`,
	}
	initData := signWebAppParams(t, params, testBotToken)

	if _, err := ValidateWebAppInitData(initData, testBotToken); err != ErrExpired {
		t.Errorf("err = %v, want ErrExpired", err)
	}
}

func TestValidateWebAppInitData_Missing(t *testing.T) {
	if _, err := ValidateWebAppInitData("hash=deadbeef", testBotToken); err != ErrMissingFields {
		t.Errorf("err = %v, want ErrMissingFields", err)
	}
}

