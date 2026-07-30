package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"therunish/internal/domain"
	"therunish/internal/session"
	"therunish/internal/storage"
)

const (
	AdminCookieName = "runish_admin"
	CoachCookieName = "runish_coach"

	RoleAdmin = "admin"
	RoleCoach = "coach"
)

type ctxKey string

const (
	userCtxKey      ctxKey = "user"
	panelRoleCtxKey ctxKey = "panel_role"
)

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

// PanelRoleFromContext — роль панельной сессии текущего запроса (admin | coach).
func PanelRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(panelRoleCtxKey).(string)
	return role
}

// LoginPathFor — куда отправлять неавторизованного посетителя панели.
func LoginPathFor(path string) string {
	if strings.HasPrefix(path, "/coach") {
		return "/coach/login"
	}
	return "/admin/login"
}

// RequirePanel пускает в панель сессии с одной из перечисленных ролей.
// Админская и тренерская куки живут раздельно, поэтому в одном браузере можно
// быть залогиненным и тем и другим одновременно.
func RequirePanel(store *storage.Store, allow ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allow))
	for _, role := range allow {
		allowed[role] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			login := LoginPathFor(r.URL.Path)

			var found bool
			for _, name := range []string{AdminCookieName, CoachCookieName} {
				c, err := r.Cookie(name)
				if err != nil || c.Value == "" {
					continue
				}
				role, err := store.GetAdminSession(r.Context(), c.Value)
				if errors.Is(err, storage.ErrNotFound) {
					continue
				}
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				if !allowed[role] {
					continue
				}
				r = r.WithContext(context.WithValue(r.Context(), panelRoleCtxKey, role))
				found = true
				break
			}
			if !found {
				http.Redirect(w, r, login, http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAdminToken(store *storage.Store) func(http.Handler) http.Handler {
	return RequirePanel(store, RoleAdmin)
}
