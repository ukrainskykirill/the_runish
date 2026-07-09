package payment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func ValidateNotification(rawBody []byte, password string) (Notification, bool, error) {
	var raw map[string]any
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return Notification{}, false, fmt.Errorf("decode notification: %w", err)
	}

	token, ok := raw["Token"].(string)
	if !ok || token == "" {
		return Notification{}, false, fmt.Errorf("missing Token in notification")
	}

	filtered := make(map[string]any, len(raw))
	for k, v := range raw {
		if k == "Token" {
			continue
		}
		if isScalar(v) {
			filtered[k] = v
		}
	}

	expected := sign(filtered, password)
	if !safeEqual(expected, token) {
		return Notification{}, false, nil
	}

	var notif Notification
	if err := json.Unmarshal(rawBody, &notif); err != nil {
		return Notification{}, false, fmt.Errorf("unmarshal notification fields: %w", err)
	}
	return notif, true, nil
}

func isScalar(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return false
	default:
		return true
	}
}

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
		if !isScalar(v) {
			continue
		}
		sb.WriteString(anyToSignString(v))
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

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
		return strconv.FormatFloat(val, 'f', -1, 64)
	case json.Number:
		return val.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}
