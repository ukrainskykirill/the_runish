package auth

import (
	"context"
	"errors"
	"net/http"

	"therunish/internal/domain"
	"therunish/internal/session"
	"therunish/internal/storage"
)

const AdminCookieName = "runish_admin"

type ctxKey string

const userCtxKey ctxKey = "user"

type Middleware struct {
	sessions *session.Manager
	store    *storage.Store
}

func NewMiddleware(sessions *session.Manager, store *storage.Store) *Middleware {
	return &Middleware{sessions: sessions, store: store}
}

func (mw *Middleware) LoadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := mw.sessions.UserFromRequest(r)
		if err == nil && userID > 0 {
			user, err := mw.store.GetUserByID(r.Context(), userID)
			if err == nil {
				ctx := context.WithValue(r.Context(), userCtxKey, &user)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (mw *Middleware) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := mw.sessions.UserFromRequest(r)
		if err != nil || userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		user, err := mw.store.GetUserByID(r.Context(), userID)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (mw *Middleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := mw.sessions.UserFromRequest(r)
		if err != nil || userID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		user, err := mw.store.GetUserByID(r.Context(), userID)
		if err != nil || !user.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserFromContext(ctx context.Context) *domain.User {
	u, _ := ctx.Value(userCtxKey).(*domain.User)
	return u
}

func RequireAdminToken(store *storage.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(AdminCookieName)
			if err != nil || c.Value == "" {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}
			if err := store.GetAdminSession(r.Context(), c.Value); err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
					return
				}
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
