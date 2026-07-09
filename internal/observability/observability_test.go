package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRecover(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))

	h := Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(buf.String(), "Паника в обработчике") {
		t.Errorf("log missing panic message: %s", buf.String())
	}
}

func TestAlertLevel(t *testing.T) {
	var rec slog.Record
	logger := slog.New(&captureHandler{rec: &rec})

	Alert(context.Background(), logger, "Оплата не прошла", errors.New("boom"), "order_id", 7)

	if rec.Level != LevelCritical {
		t.Fatalf("level = %v, want %v", rec.Level, LevelCritical)
	}
	if LevelCritical <= slog.LevelError {
		t.Fatal("LevelCritical must be above LevelError so plain Error logs do not alert")
	}

	_, attrs := extract(rec)
	if _, ok := attrs["origin"]; !ok {
		t.Errorf("alert missing origin attr: %v", attrs)
	}
	if attrs["order_id"] == nil {
		t.Errorf("alert missing order_id attr: %v", attrs)
	}
}

type captureHandler struct{ rec *slog.Record }

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	*h.rec = r
	return nil
}
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func TestRecoverPassesThrough(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	h := Recover(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

func TestSinkThrottle(t *testing.T) {
	cur := time.Now()
	s := &sink{lastSent: map[string]time.Time{}, now: func() time.Time { return cur }}
	err := errors.New("db down")

	if !s.allow("query failed", err) {
		t.Fatal("first occurrence should pass")
	}
	if s.allow("query failed", err) {
		t.Fatal("repeat within window should be throttled")
	}
	if !s.allow("another error", err) {
		t.Fatal("different message should pass")
	}

	cur = cur.Add(tgThrottleWindow + time.Second)
	if !s.allow("query failed", err) {
		t.Fatal("after window should pass again")
	}
}

func TestExtract(t *testing.T) {
	want := errors.New("boom")
	r := slog.NewRecord(time.Now(), slog.LevelError, "create news", 0)
	r.Add("err", want, "id", 42)

	gotErr, attrs := extract(r)
	if !errors.Is(gotErr, want) {
		t.Errorf("err = %v, want %v", gotErr, want)
	}
	if _, ok := attrs["id"]; !ok {
		t.Errorf("attrs missing id: %v", attrs)
	}
	if attrs["err"] != "boom" {
		t.Errorf("attrs[err] = %v, want boom", attrs["err"])
	}
}

func TestFormatAlertTruncates(t *testing.T) {
	long := strings.Repeat("я", tgMaxRunes+500)
	out := formatAlert("production", long, nil, nil)
	if n := len([]rune(out)); n > tgMaxRunes+1 {
		t.Errorf("alert length = %d runes, want <= %d", n, tgMaxRunes+1)
	}
}
