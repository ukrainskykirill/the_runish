package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"therunish/internal/domain"
	"therunish/internal/observability"
)

const (
	vkOAuthCookie  = "vk_oauth"
	vkOAuthTTL     = 10 * time.Minute
	vkAuthorizeURL = "https://id.vk.com/authorize"
	vkTokenURL     = "https://id.vk.com/oauth2/auth"
	vkUserInfoURL  = "https://id.vk.com/oauth2/user_info"
)

func (a *App) vkCookieSecure() bool {
	return strings.HasPrefix(a.cfg.BaseURL, "https://")
}

func (a *App) APIAuthVKStart(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.VKEnabled() {
		http.Error(w, "VK login disabled", http.StatusNotFound)
		return
	}

	state, err := randomURLToken(16)
	if err != nil {
		a.logger.Error("vk: generate state", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	verifier, err := randomURLToken(48)
	if err != nil {
		a.logger.Error("vk: generate verifier", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	http.SetCookie(w, &http.Cookie{
		Name:     vkOAuthCookie,
		Value:    state + ":" + verifier,
		Path:     "/",
		Expires:  time.Now().Add(vkOAuthTTL),
		HttpOnly: true,
		Secure:   a.vkCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", a.cfg.VKClientID)
	q.Set("redirect_uri", a.cfg.VKRedirectURI())
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("scope", "")

	http.Redirect(w, r, vkAuthorizeURL+"?"+q.Encode(), http.StatusFound)
}

func (a *App) APIAuthVKCallback(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.VKEnabled() {
		http.Error(w, "VK login disabled", http.StatusNotFound)
		return
	}

	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")
	deviceID := query.Get("device_id")
	if code == "" || state == "" {
		a.logger.Warn("vk callback: missing code/state", "err", query.Get("error"))
		http.Error(w, "Invalid VK response", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie(vkOAuthCookie)
	if err != nil {
		a.logger.Warn("vk callback: missing oauth cookie")
		http.Error(w, "Auth session expired", http.StatusBadRequest)
		return
	}
	savedState, verifier, ok := strings.Cut(cookie.Value, ":")
	if !ok || savedState != state || verifier == "" {
		a.logger.Warn("vk callback: state mismatch")
		http.Error(w, "Auth validation failed", http.StatusForbidden)
		return
	}
	a.clearVKCookie(w)

	token, err := a.vkExchangeCode(r.Context(), code, verifier, deviceID, state)
	if err != nil {
		observability.Alert(r.Context(), a.logger, "Вход через ВК: не удалось обменять код на токен", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	info, err := a.vkUserInfo(r.Context(), token)
	if err != nil {
		observability.Alert(r.Context(), a.logger, "Вход через ВК: не удалось получить данные пользователя", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	u := &domain.User{
		VKID:     info.ID,
		FullName: strings.TrimSpace(info.FirstName + " " + info.LastName),
	}
	saved, err := a.store.UpsertVKUser(r.Context(), u)
	if err != nil {
		observability.Alert(r.Context(), a.logger, "Регистрация не удалась (вход через ВК на сайте)", err, "vk_id", info.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if _, err := a.sessions.Create(w, saved.ID); err != nil {
		a.logger.Error("vk: create session", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logger.Info("vk login", "user_id", saved.ID, "vk_id", saved.VKID)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) clearVKCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     vkOAuthCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.vkCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) vkExchangeCode(ctx context.Context, code, verifier, deviceID, state string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("client_id", a.cfg.VKClientID)
	form.Set("client_secret", a.cfg.VKClientSecret)
	form.Set("redirect_uri", a.cfg.VKRedirectURI())
	form.Set("state", state)
	if deviceID != "" {
		form.Set("device_id", deviceID)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := a.vkPostForm(ctx, vkTokenURL, form, &body); err != nil {
		return "", err
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("vk token error: %s %s", body.Error, body.ErrorDesc)
	}
	return body.AccessToken, nil
}

type vkUser struct {
	ID        int64
	FirstName string
	LastName  string
}

func (a *App) vkUserInfo(ctx context.Context, accessToken string) (vkUser, error) {
	form := url.Values{}
	form.Set("access_token", accessToken)
	form.Set("client_id", a.cfg.VKClientID)

	var body struct {
		User struct {
			UserID    string `json:"user_id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"user"`
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	if err := a.vkPostForm(ctx, vkUserInfoURL, form, &body); err != nil {
		return vkUser{}, err
	}
	if body.User.UserID == "" {
		return vkUser{}, fmt.Errorf("vk user_info error: %s %s", body.Error, body.ErrorDesc)
	}
	id, err := strconv.ParseInt(body.User.UserID, 10, 64)
	if err != nil {
		return vkUser{}, fmt.Errorf("parse vk user_id %q: %w", body.User.UserID, err)
	}
	return vkUser{ID: id, FirstName: body.User.FirstName, LastName: body.User.LastName}, nil
}

func (a *App) vkPostForm(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode vk response (%s): %w", endpoint, err)
	}
	return nil
}

func randomURLToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
