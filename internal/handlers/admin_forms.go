package handlers

import (
	"net/http"
	"strconv"

	"therunish/internal/domain"
)

type serviceFormView struct {
	PageData
	Service      *domain.Service
	Error        string
	AllTrainings []domain.Training
	Selected     map[int64]bool
}

func parseServiceForm(r *http.Request) (*domain.Service, string) {
	kind := r.FormValue("kind")
	title := r.FormValue("title")
	desc := r.FormValue("description")
	priceStr := r.FormValue("price_rub")
	durationStr := r.FormValue("duration_days")
	sortStr := r.FormValue("sort_order")
	isActive := r.FormValue("is_active") == "on"

	svcKind := domain.ServiceKind(kind)
	if !svcKind.IsValid() {
		return nil, "Неверный тип услуги"
	}

	priceKop, errMsg := parseRubles(priceStr)
	if errMsg != "" {
		return nil, errMsg
	}
	if svcKind == domain.KindFree && priceKop != 0 {
		return nil, "Бесплатная услуга должна иметь цену 0"
	}

	var duration *int
	if durationStr != "" {
		d, err := strconv.Atoi(durationStr)
		if err != nil || d <= 0 {
			return nil, "Срок должен быть положительным числом"
		}
		duration = &d
	}
	if svcKind.GrantsSubscription() && (duration == nil || *duration <= 0) {
		return nil, "Подписка/бандл требуют срок (дни)"
	}

	withSub, errMsg := parseOptionalRubles(r.FormValue("price_with_sub_rub"))
	if errMsg != "" {
		return nil, "Цена при подписке: " + errMsg
	}
	promo, errMsg := parseOptionalRubles(r.FormValue("promo_price_rub"))
	if errMsg != "" {
		return nil, "Льготная цена: " + errMsg
	}

	var quota *int
	if qs := r.FormValue("trainings_quota"); qs != "" {
		q, err := strconv.Atoi(qs)
		if err != nil || q < 0 {
			return nil, "Квота занятий должна быть неотрицательным числом"
		}
		quota = &q
	}

	sortOrder, _ := strconv.Atoi(sortStr)
	return &domain.Service{
		Kind:            svcKind,
		Title:           title,
		Description:     desc,
		PriceKop:        priceKop,
		DurationDays:    duration,
		SortOrder:       sortOrder,
		IsActive:        isActive,
		PriceWithSubKop: withSub,
		PromoPriceKop:   promo,
		TrainingsQuota:  quota,
	}, ""
}

func parseServiceTrainingIDs(r *http.Request) []int64 {
	raw := r.Form["training_ids"]
	ids := make([]int64, 0, len(raw))
	for _, s := range raw {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func parseOptionalRubles(str string) (*int64, string) {
	if str == "" {
		return nil, ""
	}
	v, errMsg := parseRubles(str)
	if errMsg != "" {
		return nil, errMsg
	}
	return &v, ""
}

func parseNewsForm(r *http.Request) (*domain.News, string) {
	title := r.FormValue("title")
	content := r.FormValue("content")
	sortStr := r.FormValue("sort_order")
	isActive := r.FormValue("is_active") == "on"

	if title == "" {
		return nil, "Заголовок обязателен"
	}
	if content == "" {
		return nil, "Текст новости обязателен"
	}

	sortOrder, _ := strconv.Atoi(sortStr)
	return &domain.News{
		Title:     title,
		Content:   content,
		SortOrder: sortOrder,
		IsActive:  isActive,
	}, ""
}

func parseMerchForm(r *http.Request) (*domain.Merch, string) {
	title := r.FormValue("title")
	desc := r.FormValue("description")
	priceStr := r.FormValue("price_rub")
	sortStr := r.FormValue("sort_order")
	isActive := r.FormValue("is_active") == "on"

	if title == "" {
		return nil, "Название обязательно"
	}
	priceKop, errMsg := parseRubles(priceStr)
	if errMsg != "" {
		return nil, errMsg
	}

	sortOrder, _ := strconv.Atoi(sortStr)
	return &domain.Merch{
		Title:       title,
		Description: desc,
		PriceKop:    priceKop,
		SortOrder:   sortOrder,
		IsActive:    isActive,
	}, ""
}

func parseRubles(s string) (int64, string) {
	if s == "" {
		return 0, "Цена обязательна"
	}
	rub, err := strconv.ParseFloat(s, 64)
	if err != nil || rub < 0 {
		return 0, "Цена должна быть неотрицательным числом"
	}
	return int64(rub*100 + 0.5), ""
}
