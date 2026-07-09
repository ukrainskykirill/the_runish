package handlers

import (
	"log/slog"

	"therunish/internal/auth"
	"therunish/internal/config"
	"therunish/internal/payment"
	"therunish/internal/render"
	"therunish/internal/session"
	"therunish/internal/storage"
	"therunish/internal/usecase"
)

const frontendDist = "web/frontend/dist"

type App struct {
	cfg        config.Config
	store      *storage.Store
	sessions   *session.Manager
	mw         *auth.Middleware
	renderer   *render.Renderer
	provider   payment.PaymentProvider
	pricing    *usecase.Pricing
	cart       *usecase.Cart
	checkout   *usecase.Checkout
	adminUsers *usecase.AdminUsers
	membership *usecase.AdminMembership
	trainings  *usecase.Training
	logger     *slog.Logger
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
	pricing := usecase.NewPricing(d.Store, d.Logger)
	cart := usecase.NewCart(d.Store, pricing)
	return &App{
		cfg:        d.Cfg,
		store:      d.Store,
		sessions:   d.Sessions,
		mw:         d.MW,
		renderer:   d.Renderer,
		provider:   d.Provider,
		pricing:    pricing,
		cart:       cart,
		checkout:   usecase.NewCheckout(d.Cfg, d.Store, cart, d.Provider, d.Logger),
		adminUsers: usecase.NewAdminUsers(d.Store),
		membership: usecase.NewAdminMembership(d.Store),
		trainings:  usecase.NewTraining(d.Store),
		logger:     d.Logger,
	}
}
