package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"

	"therunish/internal/config"
	"therunish/internal/telegram"
)

// LevelCritical — уровень суперкритичных событий. Только записи этого уровня
// уходят в Sentry и Telegram-алерт-бот; обычные Error-логи пишутся лишь в stdout.
const LevelCritical = slog.LevelError + 4

const (
	tgThrottleWindow = 5 * time.Minute
	tgMaxRunes       = 3500
	flushTimeout     = 2 * time.Second
	sourceContext    = 6
)

// Alert фиксирует суперкритичное событие: структурный лог в stdout + Sentry + Telegram.
func Alert(ctx context.Context, logger *slog.Logger, msg string, err error, attrs ...any) {
	args := make([]any, 0, len(attrs)+4)
	args = append(args, attrs...)
	if err != nil {
		args = append(args, "err", err)
	}
	if _, file, line, ok := runtime.Caller(1); ok {
		args = append(args, "origin", fmt.Sprintf("%s:%d", shortPath(file), line))
	}
	logger.Log(ctx, LevelCritical, msg, args...)
}

func Setup(cfg config.Config) (*slog.Logger, func()) {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       cfg.LogLevel,
		ReplaceAttr: renameCritical,
	})
	flush := func() {}

	s := &sink{
		env:      cfg.SentryEnvironment,
		lastSent: map[string]time.Time{},
		now:      time.Now,
		fallback: slog.New(base),
	}

	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              cfg.SentryDSN,
			Environment:      cfg.SentryEnvironment,
			AttachStacktrace: true,
			TracesSampleRate: 0,
			BeforeSend:       contextify,
		}); err != nil {
			s.fallback.Error("sentry init failed", "err", err)
		} else {
			s.useSentry = true
			flush = func() { sentry.Flush(flushTimeout) }
		}
	}

	if cfg.AlertBotToken != "" && cfg.AlertChatID != 0 {
		s.tg = telegram.New(cfg.AlertBotToken)
		s.chatID = cfg.AlertChatID
	}

	return slog.New(&forwardHandler{Handler: base, sink: s}), flush
}

func renameCritical(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		if lv, ok := a.Value.Any().(slog.Level); ok && lv == LevelCritical {
			a.Value = slog.StringValue("CRITICAL")
		}
	}
	return a
}

type forwardHandler struct {
	slog.Handler
	sink *sink
}

func (h *forwardHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= LevelCritical && h.sink != nil {
		h.sink.capture(r)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *forwardHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &forwardHandler{Handler: h.Handler.WithAttrs(attrs), sink: h.sink}
}

func (h *forwardHandler) WithGroup(name string) slog.Handler {
	return &forwardHandler{Handler: h.Handler.WithGroup(name), sink: h.sink}
}

type tgSender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

type sink struct {
	useSentry bool
	tg        tgSender
	chatID    int64
	env       string

	mu       sync.Mutex
	lastSent map[string]time.Time
	now      func() time.Time

	fallback *slog.Logger
}

func (s *sink) capture(r slog.Record) {
	err, attrs := extract(r)

	if s.useSentry {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelError)
			if len(attrs) > 0 {
				scope.SetContext("attributes", attrs)
			}
			if err != nil {
				sentry.CaptureException(fmt.Errorf("%s: %w", r.Message, err))
			} else {
				sentry.CaptureMessage(r.Message)
			}
		})
	}

	if s.tg != nil && s.allow(r.Message, err) {
		go s.send(formatAlert(s.env, r.Message, err, attrs))
	}
}

func (s *sink) allow(msg string, err error) bool {
	key := msg
	if err != nil {
		key += "|" + err.Error()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if last, ok := s.lastSent[key]; ok && now.Sub(last) < tgThrottleWindow {
		return false
	}
	s.lastSent[key] = now

	if len(s.lastSent) > 500 {
		for k, t := range s.lastSent {
			if now.Sub(t) > tgThrottleWindow {
				delete(s.lastSent, k)
			}
		}
	}
	return true
}

func (s *sink) send(text string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.tg.SendMessage(ctx, s.chatID, text); err != nil {
		s.fallback.Error("alert telegram send failed", "err", err)
	}
}

func extract(r slog.Record) (error, map[string]any) {
	attrs := make(map[string]any, r.NumAttrs())
	var found error
	r.Attrs(func(a slog.Attr) bool {
		v := a.Value.Any()
		if e, ok := v.(error); ok {
			attrs[a.Key] = e.Error()
			if found == nil || a.Key == "err" || a.Key == "error" {
				found = e
			}
		} else {
			attrs[a.Key] = v
		}
		return true
	})
	return found, attrs
}

func formatAlert(env, msg string, err error, attrs map[string]any) string {
	var b strings.Builder
	b.WriteString("🔴 ")
	if env != "" {
		b.WriteString("[" + env + "] ")
	}
	b.WriteString(msg + "\n")
	if err != nil {
		b.WriteString("\nОшибка: " + err.Error() + "\n")
	}
	if o, ok := attrs["origin"]; ok {
		fmt.Fprintf(&b, "📍 %v\n", o)
	}

	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		switch k {
		case "err", "error", "stack", "origin":
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		b.WriteString("\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "%s: %v\n", k, attrs[k])
		}
	}

	out := strings.TrimRight(b.String(), "\n")
	if rs := []rune(out); len(rs) > tgMaxRunes {
		out = string(rs[:tgMaxRunes]) + "…"
	}
	return out
}

// contextify дочитывает строки исходного кода к фреймам стектрейса, чтобы в
// Sentry был виден кусок кода. Работает, если исходники доступны по тем же путям,
// что и при сборке (см. Dockerfile — исходники копируются в рантайм-образ).
func contextify(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	cache := map[string][]string{}
	for i := range event.Exception {
		if event.Exception[i].Stacktrace == nil {
			continue
		}
		frames := event.Exception[i].Stacktrace.Frames
		for j := range frames {
			addSource(&frames[j], cache)
		}
	}
	return event
}

func addSource(f *sentry.Frame, cache map[string][]string) {
	path := f.AbsPath
	if path == "" {
		path = f.Filename
	}
	if path == "" || f.Lineno <= 0 {
		return
	}

	lines, ok := cache[path]
	if !ok {
		data, err := os.ReadFile(path)
		if err != nil {
			cache[path] = nil
			return
		}
		lines = strings.Split(string(data), "\n")
		cache[path] = lines
	}
	if lines == nil {
		return
	}

	idx := f.Lineno - 1
	if idx < 0 || idx >= len(lines) {
		return
	}
	f.ContextLine = lines[idx]

	pre := idx - sourceContext
	if pre < 0 {
		pre = 0
	}
	f.PreContext = append([]string(nil), lines[pre:idx]...)

	post := idx + 1 + sourceContext
	if post > len(lines) {
		post = len(lines)
	}
	f.PostContext = append([]string(nil), lines[idx+1:post]...)
}

func shortPath(file string) string {
	parts := strings.Split(file, "/")
	if len(parts) <= 2 {
		return file
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

// Recover — паника в HTTP-хендлере становится суперкритичным алертом и ответом 500.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.Log(r.Context(), LevelCritical, "Паника в обработчике запроса",
					"err", fmt.Errorf("panic: %v", rec),
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal_error"}`))
			}()
			next.ServeHTTP(w, r)
		})
	}
}
