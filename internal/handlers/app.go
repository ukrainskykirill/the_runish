package handlers

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"therunish/internal/auth"
	"therunish/internal/config"
	"therunish/internal/payment"
	"therunish/internal/render"
	"therunish/internal/session"
	"therunish/internal/storage"
)

// frontendDist — собранный React SPA (web/frontend/dist).
const frontendDist = "web/frontend/dist"

// App — контейнер зависимостей для HTTP-обработчиков.
type App struct {
	cfg      config.Config
	store    *storage.Store
	sessions *session.Manager
	mw       *auth.Middleware
	renderer *render.Renderer
	provider payment.PaymentProvider
	logger   *slog.Logger
}

type Deps struct {
	Cfg      config.Config
	Store    *storage.Store
	Sessions *session.Manager
	MW       *auth.Middleware
	Renderer *render.Renderer
	Provider payment.PaymentProvider
	Logger   *slog.Logger
}

func NewApp(d Deps) *App {
	return &App{
		cfg:      d.Cfg,
		store:    d.Store,
		sessions: d.Sessions,
		mw:       d.MW,
		renderer: d.Renderer,
		provider: d.Provider,
		logger:   d.Logger,
	}
}

// serveSPA отдаёт index.html собранного React-приложения (SPA shell).
// index.html НЕ кешируем: он ссылается на хешированные чанки, и при деплое браузер
// должен сразу получать свежие ссылки (иначе грузит старый JS из кеша).
func (a *App) serveSPA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.ServeFile(w, r, filepath.Join(frontendDist, "index.html"))
}

// Routes возвращает http.ServeMux с зарегистрированными маршрутами.
// Паттерны Go 1.22+: "GET /path", "POST /path", "{id}".
func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	// Статика админки.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// JSON API для публичного фронта.
	mux.HandleFunc("GET /api/me", a.mw.LoadUser(http.HandlerFunc(a.APIMe)).ServeHTTP)
	mux.HandleFunc("GET /api/home", a.mw.LoadUser(http.HandlerFunc(a.APIHome)).ServeHTTP)
	mux.HandleFunc("GET /api/catalog", a.mw.LoadUser(http.HandlerFunc(a.APICatalog)).ServeHTTP)
	mux.HandleFunc("GET /api/news", a.APINews)
	mux.HandleFunc("GET /api/news/{id}", a.APINewsItem)
	mux.HandleFunc("GET /api/merch", a.APIMerch)
	mux.HandleFunc("GET /api/schedule", a.APISchedule)

	mux.HandleFunc("GET /api/cart", a.mw.LoadUser(http.HandlerFunc(a.APICartView)).ServeHTTP)
	mux.HandleFunc("POST /api/cart", a.mw.LoadUser(http.HandlerFunc(a.APICartAdd)).ServeHTTP)
	mux.HandleFunc("POST /api/cart/remove", a.mw.LoadUser(http.HandlerFunc(a.APICartRemove)).ServeHTTP)

	mux.HandleFunc("POST /api/checkout", a.mw.RequireUser(http.HandlerFunc(a.APICheckout)).ServeHTTP)

	mux.HandleFunc("GET /api/auth/telegram/start", a.APIAuthTelegramStart)
	mux.HandleFunc("GET /api/auth/telegram/callback", a.APIAuthTelegramCallback)
	mux.HandleFunc("GET /api/auth/telegram/status", a.APIAuthTelegramStatus)
	mux.HandleFunc("GET /api/auth/telegram/complete", a.APIAuthTelegramComplete)
	mux.HandleFunc("POST /api/auth/logout", a.APIAuthLogout)

	// Платежи.
	mux.HandleFunc("POST /payment/webhook", a.PaymentWebhook)
	mux.HandleFunc("GET /payment/success", a.PaymentSuccess)
	mux.HandleFunc("GET /payment/fail", a.PaymentFail)

	// Эмулятор платёжной формы для PAYMENT_PROVIDER=mock.
	mux.HandleFunc("GET /payment/mock/{orderID}", a.PaymentMockPage)
	mux.HandleFunc("POST /payment/mock/{orderID}", a.PaymentMockConfirm)

	// Admin: авторизация (логин/пароль, без Telegram).
	mux.HandleFunc("GET /admin/login", a.AdminLoginPage)
	mux.HandleFunc("POST /admin/login", a.AdminLoginSubmit)
	mux.HandleFunc("POST /admin/logout", a.AdminLogout)

	// Admin: дашборд (главная страница)
	adminMw := auth.RequireAdminToken(a.store)
	mux.HandleFunc("GET /admin", adminMw(http.HandlerFunc(a.AdminDashboardPage)).ServeHTTP)

	// Admin: CRUD услуг (HTML-формы, защищены RequireAdminToken)
	mux.HandleFunc("GET /admin/services", adminMw(http.HandlerFunc(a.AdminListServicesPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/services/new", adminMw(http.HandlerFunc(a.AdminCreateServicePage)).ServeHTTP)
	mux.HandleFunc("POST /admin/services", adminMw(http.HandlerFunc(a.AdminCreateServiceSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/services/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditServicePage)).ServeHTTP)
	mux.HandleFunc("POST /admin/services/{id}", adminMw(http.HandlerFunc(a.AdminUpdateServiceSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/services/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteServiceSubmit)).ServeHTTP)

	// Admin: последние оплаты
	mux.HandleFunc("GET /admin/payments", adminMw(http.HandlerFunc(a.AdminListPaymentsPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/payments/{id}/refund", adminMw(http.HandlerFunc(a.AdminRefundPaymentSubmit)).ServeHTTP)

	// Admin: пользователи + ручное управление подписками
	mux.HandleFunc("GET /admin/users", adminMw(http.HandlerFunc(a.AdminListUsersPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/users/{id}", adminMw(http.HandlerFunc(a.AdminUserDetailPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/entry-fee", adminMw(http.HandlerFunc(a.AdminSetEntryFeeSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/subscriptions", adminMw(http.HandlerFunc(a.AdminAddSubscriptionSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/users/{id}/subscriptions/{subID}/edit", adminMw(http.HandlerFunc(a.AdminEditSubscriptionPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/subscriptions/{subID}", adminMw(http.HandlerFunc(a.AdminUpdateSubscriptionSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/subscriptions/{subID}/extend", adminMw(http.HandlerFunc(a.AdminExtendSubscriptionSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/subscriptions/{subID}/delete", adminMw(http.HandlerFunc(a.AdminDeleteSubscriptionSubmit)).ServeHTTP)

	// Admin: CRUD новостей
	mux.HandleFunc("GET /admin/news", adminMw(http.HandlerFunc(a.AdminListNewsPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/news/new", adminMw(http.HandlerFunc(a.AdminCreateNewsPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/news", adminMw(http.HandlerFunc(a.AdminCreateNewsSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/news/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditNewsPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/news/{id}", adminMw(http.HandlerFunc(a.AdminUpdateNewsSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/news/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteNewsSubmit)).ServeHTTP)

	// Admin: настройки клуба
	mux.HandleFunc("GET /admin/settings", adminMw(http.HandlerFunc(a.AdminSettingsPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/settings", adminMw(http.HandlerFunc(a.AdminSettingsSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/settings/reminders", adminMw(http.HandlerFunc(a.AdminReminderSettingsSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/notifications/send", adminMw(http.HandlerFunc(a.AdminSendNotificationSubmit)).ServeHTTP)

	// Admin: CRUD расписания тренировок
	mux.HandleFunc("GET /admin/trainings", adminMw(http.HandlerFunc(a.AdminListTrainingsPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/trainings/new", adminMw(http.HandlerFunc(a.AdminCreateTrainingPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/trainings", adminMw(http.HandlerFunc(a.AdminCreateTrainingSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/trainings/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditTrainingPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/trainings/{id}", adminMw(http.HandlerFunc(a.AdminUpdateTrainingSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/trainings/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteTrainingSubmit)).ServeHTTP)

	// Admin: CRUD мерча
	mux.HandleFunc("GET /admin/merch", adminMw(http.HandlerFunc(a.AdminListMerchPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/merch/new", adminMw(http.HandlerFunc(a.AdminCreateMerchPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/merch", adminMw(http.HandlerFunc(a.AdminCreateMerchSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/merch/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditMerchPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/merch/{id}", adminMw(http.HandlerFunc(a.AdminUpdateMerchSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/merch/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteMerchSubmit)).ServeHTTP)

	// Healthcheck
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// React SPA: статика сборки (/assets/...) + index.html для всех остальных путей.
	fileServer := http.FileServer(http.Dir(frontendDist))
	mux.Handle("GET /assets/", fileServer)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			if fi, err := os.Stat(filepath.Join(frontendDist, filepath.Clean(r.URL.Path))); err == nil && !fi.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		a.serveSPA(w, r)
	})

	return mux
}
