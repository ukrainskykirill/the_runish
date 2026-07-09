package handlers

import (
	"errors"
	"net/http"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

func (a *App) AdminListServicesPage(w http.ResponseWriter, r *http.Request) {
	services, err := a.store.ListServices(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list services", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Services []domain.Service
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Services: services,
	}
	if err := a.renderer.Render(w, "admin_services", data); err != nil {
		a.logger.Error("render admin_services", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) renderServiceForm(w http.ResponseWriter, r *http.Request, status int, svc *domain.Service, errMsg string, selectedIDs []int64) {
	trainings, err := a.store.ListTrainings(r.Context(), true)
	if err != nil {
		a.logger.Error("list trainings for service form", "err", err)
	}
	selected := make(map[int64]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[id] = true
	}
	data := serviceFormView{
		PageData:     PageData{BotUsername: a.cfg.BotUsername},
		Service:      svc,
		Error:        errMsg,
		AllTrainings: trainings,
		Selected:     selected,
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	if err := a.renderer.Render(w, "admin_service_form", data); err != nil {
		a.logger.Error("render admin_service_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreateServicePage(w http.ResponseWriter, r *http.Request) {
	a.renderServiceForm(w, r, http.StatusOK, nil, "", nil)
}

func (a *App) AdminEditServicePage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	svc, err := a.store.GetService(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	selected, err := a.store.GetServiceTrainingIDs(r.Context(), id)
	if err != nil {
		a.logger.Error("get service trainings", "err", err)
	}
	a.renderServiceForm(w, r, http.StatusOK, &svc, "", selected)
}

func (a *App) AdminCreateServiceSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	svc, errMsg := parseServiceForm(r)
	trainingIDs := parseServiceTrainingIDs(r)
	if errMsg != "" {
		a.renderServiceForm(w, r, http.StatusBadRequest, svc, errMsg, trainingIDs)
		return
	}

	_, err := a.store.CreateServiceWithTrainings(r.Context(), svc, trainingIDs)
	if err != nil {
		a.logger.Error("create service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/services", http.StatusSeeOther)
}

func (a *App) AdminUpdateServiceSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	svc, errMsg := parseServiceForm(r)
	trainingIDs := parseServiceTrainingIDs(r)
	if errMsg != "" {
		svc.ID = id
		a.renderServiceForm(w, r, http.StatusBadRequest, svc, errMsg, trainingIDs)
		return
	}

	svc.ID = id
	if err := a.store.UpdateServiceWithTrainings(r.Context(), svc, trainingIDs); err != nil {
		a.logger.Error("update service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/services", http.StatusSeeOther)
}

func (a *App) AdminDeleteServiceSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.DeactivateService(r.Context(), id); err != nil {
		a.logger.Error("delete service", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/services", http.StatusSeeOther)
}
