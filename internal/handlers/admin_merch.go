package handlers

import (
	"errors"
	"net/http"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

func (a *App) AdminListMerchPage(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListMerch(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Merch []domain.Merch
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Merch:    items,
	}
	if err := a.renderer.Render(w, "admin_merch", data); err != nil {
		a.logger.Error("render admin_merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreateMerchPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		PageData
		Merch *domain.Merch
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
	}
	if err := a.renderer.Render(w, "admin_merch_form", data); err != nil {
		a.logger.Error("render admin_merch_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminEditMerchPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	m, err := a.store.GetMerch(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Merch *domain.Merch
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Merch:    &m,
	}
	if err := a.renderer.Render(w, "admin_merch_form", data); err != nil {
		a.logger.Error("render admin_merch_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreateMerchSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	m, errMsg := parseMerchForm(r)
	if errMsg != "" {
		data := struct {
			PageData
			Merch *domain.Merch
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			Merch:    m,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_merch_form", data)
		return
	}

	if _, err := a.store.CreateMerch(r.Context(), m); err != nil {
		a.logger.Error("create merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/merch", http.StatusSeeOther)
}

func (a *App) AdminUpdateMerchSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	m, errMsg := parseMerchForm(r)
	if errMsg != "" {
		m.ID = id
		data := struct {
			PageData
			Merch *domain.Merch
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			Merch:    m,
			Error:    errMsg,
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = a.renderer.Render(w, "admin_merch_form", data)
		return
	}

	m.ID = id
	if err := a.store.UpdateMerch(r.Context(), m); err != nil {
		a.logger.Error("update merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/merch", http.StatusSeeOther)
}

func (a *App) AdminDeleteMerchSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.DeactivateMerch(r.Context(), id); err != nil {
		a.logger.Error("delete merch", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/merch", http.StatusSeeOther)
}
