package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"therunish/internal/domain"
)

// PageData — базовые данные для (оставшихся) server-rendered страниц админки.
type PageData struct {
	User            *domain.User
	Flash           string
	ActiveNav       string
	BotUsername     string
	IsAuthenticated bool
}

// parseID — парсит {id} из пути (для admin).
func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// writeJSON сериализует v в JSON с заданным статусом.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSONError отправляет JSON {"error": "..."}.
func writeJSONError(w http.ResponseWriter, status int, errCode string) {
	writeJSON(w, status, map[string]string{"error": errCode})
}
