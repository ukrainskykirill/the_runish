package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"therunish/internal/domain"
)

type PageData struct {
	User            *domain.User
	Flash           string
	ActiveNav       string
	BotUsername     string
	IsAuthenticated bool
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, errCode string) {
	writeJSON(w, status, map[string]string{"error": errCode})
}
