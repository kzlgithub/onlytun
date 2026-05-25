package service

import (
	"errors"
	"sort"
	"time"

	paneldb "github.com/onlytun/panel/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StatsService struct {
	db *gorm.DB
}

type AgentStatItem struct {
	RuleID      string `json:"rule_id"`
	BytesUp     int64  `json:"bytes_up"`
	BytesDown   int64  `json:"bytes_down"`
	ActiveConns int64  `json:"active_conns"`
}

type AgentStatsInput struct {
	MachineID string          `json:"machine_id"`
	Stats     []AgentStatItem `json:"stats"`
}

type StatsPoint struct {
	Time      time.Time `json:"time"`
	BytesUp   int64     `json:"bytes_up"`
	BytesDown int64     `json:"bytes_down"`
}

type StatsSeries struct {
	Points    []StatsPoint `json:"points"`
	TotalUp   int64        `json:"total_up"`
	TotalDown int64        `json:"total_down"`
}

func NewStatsService(gdb *gorm.DB) *StatsService {
	return &StatsService{db: gdb}
}

func (s *StatsService) IngestStats(machine *paneldb.Machine, input AgentStatsInput) error {
	if machine == nil {
		return ErrMachineNotFound
	}
	if input.MachineID != "" && input.MachineID != machine.ID {
		return errors.New("service: machine id mismatch")
	}

	hour := time.Now().UTC().Truncate(time.Hour)
	for _, item := range input.Stats {
		var rule paneldb.ForwardRule
		if err := s.db.Take(&rule, "id = ?", item.RuleID).Error; err != nil {
			continue
		}
		if rule.IngressMachineID != machine.ID && rule.EgressMachineID != machine.ID {
			continue
		}

		record := paneldb.TrafficStat{
			RuleID:    item.RuleID,
			Hour:      hour,
			BytesUp:   item.BytesUp,
			BytesDown: item.BytesDown,
			PeakConns: item.ActiveConns,
		}
		if err := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "rule_id"}, {Name: "hour"}},
			DoUpdates: clause.Assignments(map[string]any{
				"bytes_up":   gorm.Expr("traffic_stats.bytes_up + excluded.bytes_up"),
				"bytes_down": gorm.Expr("traffic_stats.bytes_down + excluded.bytes_down"),
				"peak_conns": gorm.Expr("CASE WHEN excluded.peak_conns > traffic_stats.peak_conns THEN excluded.peak_conns ELSE traffic_stats.peak_conns END"),
			}),
		}).Create(&record).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *StatsService) LatestStatsForRules(ruleIDs []string) (map[string]RuleRealtimeStat, error) {
	statsByRule := make(map[string]RuleRealtimeStat, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return statsByRule, nil
	}

	var rows []paneldb.TrafficStat
	if err := s.db.
		Where("rule_id IN ?", ruleIDs).
		Order("rule_id ASC, hour DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		if _, ok := statsByRule[row.RuleID]; ok {
			continue
		}
		statsByRule[row.RuleID] = RuleRealtimeStat{
			BytesUp:   row.BytesUp,
			BytesDown: row.BytesDown,
			PeakConns: row.PeakConns,
		}
	}

	return statsByRule, nil
}

func (s *StatsService) GetSeries(ruleID, rangeKey string) (*StatsSeries, error) {
	now := time.Now()
	switch rangeKey {
	case "", "day":
		return s.buildDaySeries(ruleID, now)
	case "week":
		return s.buildWeekSeries(ruleID, now)
	default:
		return nil, errors.New("service: invalid range")
	}
}

func (s *StatsService) buildDaySeries(ruleID string, now time.Time) (*StatsSeries, error) {
	location := now.Location()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := start.Add(24 * time.Hour)

	var rows []paneldb.TrafficStat
	if err := s.db.Where("rule_id = ? AND hour >= ? AND hour < ?", ruleID, start.UTC(), end.UTC()).Find(&rows).Error; err != nil {
		return nil, err
	}

	byHour := make(map[time.Time]paneldb.TrafficStat, len(rows))
	for _, row := range rows {
		localHour := row.Hour.In(location)
		key := time.Date(localHour.Year(), localHour.Month(), localHour.Day(), localHour.Hour(), 0, 0, 0, location)
		byHour[key] = row
	}

	points := make([]StatsPoint, 0, 24)
	var totalUp, totalDown int64
	for i := 0; i < 24; i++ {
		slot := start.Add(time.Duration(i) * time.Hour)
		stat := byHour[slot]
		points = append(points, StatsPoint{
			Time:      slot,
			BytesUp:   stat.BytesUp,
			BytesDown: stat.BytesDown,
		})
		totalUp += stat.BytesUp
		totalDown += stat.BytesDown
	}

	return &StatsSeries{Points: points, TotalUp: totalUp, TotalDown: totalDown}, nil
}

func (s *StatsService) buildWeekSeries(ruleID string, now time.Time) (*StatsSeries, error) {
	location := now.Location()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	start := startOfToday.AddDate(0, 0, -6)
	end := startOfToday.AddDate(0, 0, 1)

	var rows []paneldb.TrafficStat
	if err := s.db.Where("rule_id = ? AND hour >= ? AND hour < ?", ruleID, start.UTC(), end.UTC()).Find(&rows).Error; err != nil {
		return nil, err
	}

	byDay := make(map[time.Time]StatsPoint)
	for _, row := range rows {
		localHour := row.Hour.In(location)
		day := time.Date(localHour.Year(), localHour.Month(), localHour.Day(), 0, 0, 0, 0, location)
		point := byDay[day]
		point.Time = day
		point.BytesUp += row.BytesUp
		point.BytesDown += row.BytesDown
		byDay[day] = point
	}

	points := make([]StatsPoint, 0, 7)
	var totalUp, totalDown int64
	for i := 0; i < 7; i++ {
		day := start.AddDate(0, 0, i)
		point := byDay[day]
		if point.Time.IsZero() {
			point.Time = day
		}
		points = append(points, point)
		totalUp += point.BytesUp
		totalDown += point.BytesDown
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Time.Before(points[j].Time)
	})

	return &StatsSeries{Points: points, TotalUp: totalUp, TotalDown: totalDown}, nil
}
