package reporter

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/onlytun/agent/tunnel"
)

// Stats re-exports tunnel statistics for reporter registrations.
type Stats = tunnel.Stats

type statsItem struct {
	RuleID      string `json:"rule_id"`
	BytesUp     int64  `json:"bytes_up"`
	BytesDown   int64  `json:"bytes_down"`
	ActiveConns int64  `json:"active_conns"`
}

type statsPayload struct {
	MachineID string      `json:"machine_id"`
	Stats     []statsItem `json:"stats"`
}

// RegisterStatsSource 注册一条转发规则的统计数据来源。
func (r *Reporter) RegisterStatsSource(ruleID string, getter func() Stats) {
	r.mu.Lock()
	r.sources[ruleID] = getter
	r.mu.Unlock()
}

// UnregisterStatsSource 注销一条规则的统计来源（规则删除时调用）。
func (r *Reporter) UnregisterStatsSource(ruleID string) {
	r.mu.Lock()
	delete(r.sources, ruleID)
	delete(r.last, ruleID)
	r.mu.Unlock()
}

func (r *Reporter) runStatsLoop(ctx context.Context) {
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	r.sendStats(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.sendStats(ctx)
		}
	}
}

func (r *Reporter) sendStats(ctx context.Context) {
	payload := statsPayload{
		MachineID: r.machineID,
		Stats:     r.collectStats(),
	}
	if err := r.postJSON(ctx, "/api/agent/stats", payload); err != nil {
		log.Printf("[WARN] reporter stats post failed: %v", err)
	}
}

func (r *Reporter) collectStats() []statsItem {
	r.mu.RLock()
	keys := make([]string, 0, len(r.sources))
	for ruleID := range r.sources {
		keys = append(keys, ruleID)
	}
	sort.Strings(keys)

	getters := make([]func() Stats, 0, len(keys))
	for _, ruleID := range keys {
		getters = append(getters, r.sources[ruleID])
	}
	r.mu.RUnlock()

	items := make([]statsItem, 0, len(keys))
	for i, ruleID := range keys {
		stats := getters[i]()
		r.mu.Lock()
		previous, seen := r.last[ruleID]
		deltaUp, deltaDown := stats.BytesUp, stats.BytesDown
		if seen {
			deltaUp = nonNegativeDelta(stats.BytesUp, previous.BytesUp)
			deltaDown = nonNegativeDelta(stats.BytesDown, previous.BytesDown)
		}
		r.last[ruleID] = stats
		r.mu.Unlock()
		items = append(items, statsItem{
			RuleID:      ruleID,
			BytesUp:     deltaUp,
			BytesDown:   deltaDown,
			ActiveConns: stats.ActiveConns,
		})
	}

	return items
}

func nonNegativeDelta(current, previous int64) int64 {
	if current < previous {
		return current
	}
	return current - previous
}
