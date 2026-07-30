package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"therunish/internal/auth"
	"therunish/internal/buildinfo"
)

func (a *App) serveSPA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.ServeFile(w, r, filepath.Join(frontendDist, "index.html"))
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	mux.HandleFunc("GET /api/me", a.mw.LoadUser(http.HandlerFunc(a.APIMe)).ServeHTTP)
	mux.HandleFunc("POST /api/me/phone", a.mw.RequireUser(http.HandlerFunc(a.APIMeSetPhone)).ServeHTTP)
	mux.HandleFunc("GET /api/home", a.mw.LoadUser(http.HandlerFunc(a.APIHome)).ServeHTTP)
	mux.HandleFunc("GET /api/catalog", a.mw.LoadUser(http.HandlerFunc(a.APICatalog)).ServeHTTP)
	mux.HandleFunc("GET /api/news", a.APINews)
	mux.HandleFunc("GET /api/news/{id}", a.APINewsItem)
	mux.HandleFunc("GET /api/merch", a.APIMerch)
	mux.HandleFunc("GET /api/schedule", a.APISchedule)
	mux.HandleFunc("GET /api/trainings/upcoming", a.mw.LoadUser(http.HandlerFunc(a.APITrainingsUpcoming)).ServeHTTP)
	mux.HandleFunc("POST /api/trainings/register", a.mw.RequireUser(http.HandlerFunc(a.APITrainingRegister)).ServeHTTP)
	mux.HandleFunc("POST /api/trainings/cancel", a.mw.RequireUser(http.HandlerFunc(a.APITrainingCancel)).ServeHTTP)
	mux.HandleFunc("GET /api/plan", a.mw.RequireUser(http.HandlerFunc(a.APIPlan)).ServeHTTP)
	mux.HandleFunc("GET /api/cart", a.mw.LoadUser(http.HandlerFunc(a.APICartView)).ServeHTTP)
	mux.HandleFunc("POST /api/cart", a.mw.LoadUser(http.HandlerFunc(a.APICartAdd)).ServeHTTP)
	mux.HandleFunc("POST /api/cart/remove", a.mw.LoadUser(http.HandlerFunc(a.APICartRemove)).ServeHTTP)
	mux.HandleFunc("POST /api/checkout", a.mw.RequireUser(http.HandlerFunc(a.APICheckout)).ServeHTTP)
	mux.HandleFunc("GET /api/survey", a.mw.RequireUser(http.HandlerFunc(a.APISurveyGet)).ServeHTTP)
	mux.HandleFunc("POST /api/survey", a.mw.RequireUser(http.HandlerFunc(a.APISurveySubmit)).ServeHTTP)
	mux.HandleFunc("POST /api/survey/progress", a.mw.RequireUser(http.HandlerFunc(a.APISurveyProgress)).ServeHTTP)
	mux.HandleFunc("GET /api/auth/telegram/start", a.APIAuthTelegramStart)
	mux.HandleFunc("GET /api/auth/telegram/callback", a.APIAuthTelegramCallback)
	mux.HandleFunc("GET /api/auth/telegram/status", a.APIAuthTelegramStatus)
	mux.HandleFunc("GET /api/auth/telegram/complete", a.APIAuthTelegramComplete)
	mux.HandleFunc("GET /api/auth/vk/start", a.APIAuthVKStart)
	mux.HandleFunc("GET /api/auth/vk/callback", a.APIAuthVKCallback)
	mux.HandleFunc("POST /api/auth/telegram/webapp", a.APIAuthTelegramWebApp)
	mux.HandleFunc("POST /api/auth/dev", a.APIAuthDev)
	mux.HandleFunc("POST /api/auth/logout", a.APIAuthLogout)
	mux.HandleFunc("POST /api/me/notifications/enable", a.mw.RequireUser(http.HandlerFunc(a.APIMeEnableNotifications)).ServeHTTP)

	mux.HandleFunc("GET /api/payment/status", a.APIPaymentStatus)

	mux.HandleFunc("POST /payment/webhook", a.PaymentWebhook)
	mux.HandleFunc("GET /payment/success", a.PaymentSuccess)
	mux.HandleFunc("GET /payment/fail", a.PaymentFail)
	mux.HandleFunc("GET /payment/mock/{orderID}", a.PaymentMockPage)
	mux.HandleFunc("POST /payment/mock/{orderID}", a.PaymentMockConfirm)

	mux.HandleFunc("GET /admin/login", a.AdminLoginPage)
	mux.HandleFunc("POST /admin/login", a.AdminLoginSubmit)
	mux.HandleFunc("POST /admin/logout", a.AdminLogout)

	mux.HandleFunc("GET /coach/login", a.CoachLoginPage)
	mux.HandleFunc("POST /coach/login", a.CoachLoginSubmit)
	mux.HandleFunc("POST /coach/logout", a.CoachLogout)

	// Раздел планов общий для тренера и админа, поэтому одни и те же хендлеры
	// монтируются под обоими префиксами (редиректы считает panelBase).
	planMw := auth.RequirePanel(a.store, auth.RoleAdmin, auth.RoleCoach)
	for _, base := range []string{"/coach", "/admin"} {
		mux.HandleFunc("GET "+base+"/plans", planMw(http.HandlerFunc(a.AdminListPlansPage)).ServeHTTP)
		mux.HandleFunc("GET "+base+"/plans/new", planMw(http.HandlerFunc(a.AdminCreatePlanPage)).ServeHTTP)
		mux.HandleFunc("POST "+base+"/plans", planMw(http.HandlerFunc(a.AdminCreatePlanSubmit)).ServeHTTP)
		mux.HandleFunc("GET "+base+"/plans/{id}/edit", planMw(http.HandlerFunc(a.AdminEditPlanPage)).ServeHTTP)
		mux.HandleFunc("POST "+base+"/plans/{id}", planMw(http.HandlerFunc(a.AdminUpdatePlanSubmit)).ServeHTTP)
		mux.HandleFunc("POST "+base+"/plans/{id}/publish", planMw(http.HandlerFunc(a.AdminPublishPlanSubmit)).ServeHTTP)
		mux.HandleFunc("POST "+base+"/plans/{id}/unpublish", planMw(http.HandlerFunc(a.AdminUnpublishPlanSubmit)).ServeHTTP)
		mux.HandleFunc("POST "+base+"/plans/{id}/notify", planMw(http.HandlerFunc(a.AdminNotifyPlanSubmit)).ServeHTTP)
		mux.HandleFunc("POST "+base+"/plans/{id}/delete", planMw(http.HandlerFunc(a.AdminDeletePlanSubmit)).ServeHTTP)
	}
	mux.HandleFunc("GET /coach", planMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/coach/plans", http.StatusSeeOther)
	})).ServeHTTP)

	adminMw := auth.RequireAdminToken(a.store)
	mux.HandleFunc("GET /admin", adminMw(http.HandlerFunc(a.AdminDashboardPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/services", adminMw(http.HandlerFunc(a.AdminListServicesPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/services/new", adminMw(http.HandlerFunc(a.AdminCreateServicePage)).ServeHTTP)
	mux.HandleFunc("POST /admin/services", adminMw(http.HandlerFunc(a.AdminCreateServiceSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/services/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditServicePage)).ServeHTTP)
	mux.HandleFunc("POST /admin/services/{id}", adminMw(http.HandlerFunc(a.AdminUpdateServiceSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/services/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteServiceSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/payments", adminMw(http.HandlerFunc(a.AdminListPaymentsPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/payments/{id}/refund", adminMw(http.HandlerFunc(a.AdminRefundPaymentSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/users", adminMw(http.HandlerFunc(a.AdminListUsersPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/users/{id}", adminMw(http.HandlerFunc(a.AdminUserDetailPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/entry-fee", adminMw(http.HandlerFunc(a.AdminSetEntryFeeSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/subscriptions", adminMw(http.HandlerFunc(a.AdminAddSubscriptionSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/users/{id}/subscriptions/{subID}/edit", adminMw(http.HandlerFunc(a.AdminEditSubscriptionPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/subscriptions/{subID}", adminMw(http.HandlerFunc(a.AdminUpdateSubscriptionSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/subscriptions/{subID}/extend", adminMw(http.HandlerFunc(a.AdminExtendSubscriptionSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/users/{id}/subscriptions/{subID}/delete", adminMw(http.HandlerFunc(a.AdminDeleteSubscriptionSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/news", adminMw(http.HandlerFunc(a.AdminListNewsPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/news/new", adminMw(http.HandlerFunc(a.AdminCreateNewsPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/news", adminMw(http.HandlerFunc(a.AdminCreateNewsSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/news/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditNewsPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/news/{id}", adminMw(http.HandlerFunc(a.AdminUpdateNewsSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/news/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteNewsSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/settings", adminMw(http.HandlerFunc(a.AdminSettingsPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/settings", adminMw(http.HandlerFunc(a.AdminSettingsSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/settings/reminders", adminMw(http.HandlerFunc(a.AdminReminderSettingsSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/settings/templates", adminMw(http.HandlerFunc(a.AdminTemplatesSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/notifications/send", adminMw(http.HandlerFunc(a.AdminSendNotificationSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/trainings", adminMw(http.HandlerFunc(a.AdminListTrainingsPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/trainings/{id}/registrations", adminMw(http.HandlerFunc(a.AdminTrainingRegistrationsPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/trainings/new", adminMw(http.HandlerFunc(a.AdminCreateTrainingPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/trainings", adminMw(http.HandlerFunc(a.AdminCreateTrainingSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/trainings/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditTrainingPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/trainings/{id}", adminMw(http.HandlerFunc(a.AdminUpdateTrainingSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/trainings/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteTrainingSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/merch", adminMw(http.HandlerFunc(a.AdminListMerchPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/merch/new", adminMw(http.HandlerFunc(a.AdminCreateMerchPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/merch", adminMw(http.HandlerFunc(a.AdminCreateMerchSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/merch/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditMerchPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/merch/{id}", adminMw(http.HandlerFunc(a.AdminUpdateMerchSubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/merch/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteMerchSubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/survey", adminMw(http.HandlerFunc(a.AdminListSurveyPage)).ServeHTTP)
	mux.HandleFunc("GET /admin/survey/new", adminMw(http.HandlerFunc(a.AdminCreateSurveyPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/survey", adminMw(http.HandlerFunc(a.AdminCreateSurveySubmit)).ServeHTTP)
	mux.HandleFunc("GET /admin/survey/{id}/edit", adminMw(http.HandlerFunc(a.AdminEditSurveyPage)).ServeHTTP)
	mux.HandleFunc("POST /admin/survey/{id}", adminMw(http.HandlerFunc(a.AdminUpdateSurveySubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/survey/{id}/delete", adminMw(http.HandlerFunc(a.AdminDeleteSurveySubmit)).ServeHTTP)
	mux.HandleFunc("POST /admin/survey/{id}/move", adminMw(http.HandlerFunc(a.AdminMoveSurveySubmit)).ServeHTTP)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK build=" + buildinfo.Time))
	})

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
