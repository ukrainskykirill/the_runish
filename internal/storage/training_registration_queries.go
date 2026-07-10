package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"therunish/internal/domain"
)

type usableEntitlement struct {
	serviceID int64
	quota     sql.NullInt64
	used      int
}

func (s *Store) ListUpcomingSessions(ctx context.Context, userID int64) ([]domain.TrainingOccurrence, error) {
	loc := clubLocation()
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	horizonDays := 7 - isoWeekday(today) + 1
	horizonEnd := today.AddDate(0, 0, horizonDays)
	todayStr := today.Format("2006-01-02")
	horizonStr := horizonEnd.Format("2006-01-02")

	trainings, err := s.ListTrainings(ctx, false)
	if err != nil {
		return nil, err
	}
	byWeekday := make(map[int][]domain.Training)
	for _, t := range trainings {
		byWeekday[t.Weekday] = append(byWeekday[t.Weekday], t)
	}

	type sessionInfo struct {
		status string
		count  int
	}
	sessions := make(map[string]sessionInfo)
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts.training_id, to_char(ts.session_date,'YYYY-MM-DD'), ts.status,
		       count(r.id) FILTER (WHERE r.status = 'active')
		FROM training_sessions ts
		LEFT JOIN training_registrations r ON r.session_id = ts.id
		WHERE ts.session_date >= $1 AND ts.session_date < $2
		GROUP BY ts.id`, todayStr, horizonStr)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	for rows.Next() {
		var tid int64
		var d, st string
		var cnt int
		if err := rows.Scan(&tid, &d, &st, &cnt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions[occKey(tid, d)] = sessionInfo{status: st, count: cnt}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	myRegs := make(map[string]int64)
	var ents []usableEntitlement
	entCovers := make(map[int64]map[int64]bool)
	entHasList := make(map[int64]bool)
	entryFeePaid := false
	if userID != 0 {
		if err := s.db.QueryRowContext(ctx, `SELECT entry_fee_paid FROM users WHERE id = $1`, userID).Scan(&entryFeePaid); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get user: %w", err)
		}

		mr, err := s.db.QueryContext(ctx, `
			SELECT ts.training_id, to_char(ts.session_date,'YYYY-MM-DD'), r.id
			FROM training_registrations r
			JOIN training_sessions ts ON ts.id = r.session_id
			WHERE r.user_id = $1 AND r.status = 'active' AND ts.session_date >= $2`, userID, todayStr)
		if err != nil {
			return nil, fmt.Errorf("list my regs: %w", err)
		}
		for mr.Next() {
			var tid, rid int64
			var d string
			if err := mr.Scan(&tid, &d, &rid); err != nil {
				_ = mr.Close()
				return nil, fmt.Errorf("scan my reg: %w", err)
			}
			myRegs[occKey(tid, d)] = rid
		}
		_ = mr.Close()

		er, err := s.db.QueryContext(ctx, `
			SELECT e.service_id, e.quota, e.used
			FROM training_entitlements e
			WHERE e.user_id = $1 AND e.status = 'active'
			  AND (e.valid_until IS NULL OR e.valid_until > now())
			  AND (e.quota IS NULL OR e.used < e.quota)`, userID)
		if err != nil {
			return nil, fmt.Errorf("list entitlements: %w", err)
		}
		var svcIDs []int64
		for er.Next() {
			var e usableEntitlement
			if err := er.Scan(&e.serviceID, &e.quota, &e.used); err != nil {
				_ = er.Close()
				return nil, fmt.Errorf("scan entitlement: %w", err)
			}
			ents = append(ents, e)
			svcIDs = append(svcIDs, e.serviceID)
		}
		_ = er.Close()

		if len(svcIDs) > 0 {
			cr, err := s.db.QueryContext(ctx,
				`SELECT service_id, training_id FROM service_trainings WHERE service_id = ANY($1)`, svcIDs)
			if err != nil {
				return nil, fmt.Errorf("list service_trainings: %w", err)
			}
			for cr.Next() {
				var sid, tid int64
				if err := cr.Scan(&sid, &tid); err != nil {
					_ = cr.Close()
					return nil, fmt.Errorf("scan service_training: %w", err)
				}
				entHasList[sid] = true
				if entCovers[sid] == nil {
					entCovers[sid] = make(map[int64]bool)
				}
				entCovers[sid][tid] = true
			}
			_ = cr.Close()
		}
	}

	accessFor := func(trainingID int64) (bool, *int) {
		if !entryFeePaid {
			return false, nil
		}
		inAccess := false
		var best *int
		for _, e := range ents {
			covers := !entHasList[e.serviceID] || entCovers[e.serviceID][trainingID]
			if !covers {
				continue
			}
			inAccess = true
			if !e.quota.Valid {
				return true, nil
			}
			rem := int(e.quota.Int64) - e.used
			if best == nil || rem > *best {
				r := rem
				best = &r
			}
		}
		return inAccess, best
	}

	var result []domain.TrainingOccurrence
	for d := 0; d < horizonDays; d++ {
		date := today.AddDate(0, 0, d)
		iso := isoWeekday(date)
		for _, t := range byWeekday[iso] {
			start, err := occurrenceStart(date, t.StartTime, loc)
			if err != nil || !start.After(now) {
				continue
			}
			ds := date.Format("2006-01-02")
			key := occKey(t.ID, ds)
			info, hasSession := sessions[key]

			occ := domain.TrainingOccurrence{
				TrainingID:  t.ID,
				Title:       t.Title,
				Weekday:     t.Weekday,
				StartTime:   t.StartTime,
				Place:       t.Place,
				PlaceURL:    t.PlaceURL,
				SessionDate: ds,
				Capacity:    t.Capacity,
			}
			if hasSession {
				occ.RegisteredCount = info.count
				occ.Cancelled = info.status == "cancelled"
			}
			occ.Available = t.Capacity == nil || occ.RegisteredCount < *t.Capacity
			if userID != 0 {
				if rid, ok := myRegs[key]; ok {
					occ.Registered = true
					id := rid
					occ.MyRegistrationID = &id
				}
				occ.InMyAccess, occ.QuotaLeft = accessFor(t.ID)
			}
			result = append(result, occ)
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].SessionDate != result[j].SessionDate {
			return result[i].SessionDate < result[j].SessionDate
		}
		return result[i].StartTime < result[j].StartTime
	})
	return result, nil
}

func occKey(trainingID int64, date string) string {
	return strconv.FormatInt(trainingID, 10) + "|" + date
}

func (s *Store) ListMyTrainingRegistrations(ctx context.Context, userID int64) ([]domain.TrainingRegistration, error) {
	loc := clubLocation()
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).Format("2006-01-02")

	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, t.id, t.title, to_char(ts.session_date,'YYYY-MM-DD'),
		       to_char(t.start_time,'HH24:MI'), t.place, r.created_at
		FROM training_registrations r
		JOIN training_sessions ts ON ts.id = r.session_id
		JOIN trainings t ON t.id = ts.training_id
		WHERE r.user_id = $1 AND r.status = 'active'
		  AND ts.status = 'scheduled' AND ts.session_date >= $2
		ORDER BY ts.session_date, t.start_time`, userID, today)
	if err != nil {
		return nil, fmt.Errorf("list my registrations: %w", err)
	}
	defer rows.Close()

	var regs []domain.TrainingRegistration
	for rows.Next() {
		var reg domain.TrainingRegistration
		if err := rows.Scan(&reg.ID, &reg.TrainingID, &reg.Title, &reg.SessionDate,
			&reg.StartTime, &reg.Place, &reg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan registration: %w", err)
		}
		regs = append(regs, reg)
	}
	return regs, rows.Err()
}
