package storage

import (
	"context"
	"fmt"
	"time"
)

type DailyRegistrationStat struct {
	Date      time.Time
	Count     int64
	UserNames []string
}

type RegistrationPeriodStats struct {
	Total    int
	MaxDaily int64
	Daily    []DailyRegistrationStat
}

type RegistrationDashboard struct {
	Month RegistrationPeriodStats
	Week  RegistrationPeriodStats
}

func (s *Store) GetRegistrationDashboard(ctx context.Context) (RegistrationDashboard, error) {
	now := time.Now()

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	weekStart := now.AddDate(0, 0, -6)
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())

	monthStats, err := s.getRegistrationStats(ctx, monthStart, now)
	if err != nil {
		return RegistrationDashboard{}, fmt.Errorf("get month registrations: %w", err)
	}
	weekStats, err := s.getRegistrationStats(ctx, weekStart, now)
	if err != nil {
		return RegistrationDashboard{}, fmt.Errorf("get week registrations: %w", err)
	}
	return RegistrationDashboard{Month: monthStats, Week: weekStats}, nil
}

func (s *Store) getRegistrationStats(ctx context.Context, from, to time.Time) (RegistrationPeriodStats, error) {
	const q = `
		SELECT DATE(created_at) AS day, COALESCE(full_name, ''), COALESCE(username, '')
		FROM users
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY day DESC, created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, from, to)
	if err != nil {
		return RegistrationPeriodStats{}, fmt.Errorf("query registration stats: %w", err)
	}
	defer rows.Close()

	dayOrder := []string{}
	dayMap := map[string]*DailyRegistrationStat{}

	for rows.Next() {
		var day time.Time
		var fullName, username string
		if err := rows.Scan(&day, &fullName, &username); err != nil {
			return RegistrationPeriodStats{}, fmt.Errorf("scan registration stat: %w", err)
		}

		key := day.Format("2006-01-02")
		stat, ok := dayMap[key]
		if !ok {
			stat = &DailyRegistrationStat{Date: day}
			dayMap[key] = stat
			dayOrder = append(dayOrder, key)
		}

		stat.Count++
		name := fullName
		if name == "" && username != "" {
			name = "@" + username
		}
		if name != "" {
			stat.UserNames = append(stat.UserNames, name)
		}
	}
	if err := rows.Err(); err != nil {
		return RegistrationPeriodStats{}, fmt.Errorf("rows err: %w", err)
	}

	var result RegistrationPeriodStats
	for _, key := range dayOrder {
		st := dayMap[key]
		result.Total += int(st.Count)
		if st.Count > result.MaxDaily {
			result.MaxDaily = st.Count
		}
		result.Daily = append(result.Daily, *st)
	}
	return result, nil
}
