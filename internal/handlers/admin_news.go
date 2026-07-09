package handlers

import (
	"errors"
	"net/http"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

func (a *App) AdminListNewsPage(w http.ResponseWriter, r *http.Request) {
	news, err := a.store.ListNews(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		News []domain.News
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		News:     news,
	}
	if err := a.renderer.Render(w, "admin_news", data); err != nil {
		a.logger.Error("render admin_news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreateNewsPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		PageData
		News  *domain.News
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
	}
	if err := a.renderer.Render(w, "admin_news_form", data); err != nil {
		a.logger.Error("render admin_news_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminEditNewsPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	n, err := a.store.GetNews(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		News  *domain.News
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		News:     &n,
	}
	if err := a.renderer.Render(w, "admin_news_form", data); err != nil {
		a.logger.Error("render admin_news_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreateNewsSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	n, errMsg := parseNewsForm(r)
	if errMsg != "" {
		data := struct {
			PageData
			News  *domain.News
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			News:     n,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_news_form", data)
		return
	}

	if _, err := a.store.CreateNews(r.Context(), n); err != nil {
		a.logger.Error("create news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
}

func (a *App) AdminUpdateNewsSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	n, errMsg := parseNewsForm(r)
	if errMsg != "" {
		n.ID = id
		data := struct {
			PageData
			News  *domain.News
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			News:     n,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_news_form", data)
		return
	}

	n.ID = id
	if err := a.store.UpdateNews(r.Context(), n); err != nil {
		a.logger.Error("update news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
}

func (a *App) AdminDeleteNewsSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.DeactivateNews(r.Context(), id); err != nil {
		a.logger.Error("delete news", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
}
