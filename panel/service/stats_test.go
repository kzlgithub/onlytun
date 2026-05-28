package service

import (
	"path/filepath"
	"testing"
	"time"

	paneldb "github.com/onlytun/panel/db"
)

func TestIngestStatsIgnoresUnknownRuleIDs(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	machine := paneldb.Machine{
		ID:    "machine-1",
		Name:  "Ingress",
		Role:  "ingress",
		Token: "token-1",
	}
	egress := paneldb.Machine{
		ID:    "machine-2",
		Name:  "Egress",
		Role:  "egress",
		Token: "token-2",
	}
	rule := paneldb.ForwardRule{
		ID:               "rule-1",
		Name:             "Rule",
		IngressMachineID: machine.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.com",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}
	if err := gdb.Create(&machine).Error; err != nil {
		t.Fatalf("create machine: %v", err)
	}
	if err := gdb.Create(&egress).Error; err != nil {
		t.Fatalf("create egress: %v", err)
	}
	if err := gdb.Create(&rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}

	svc := NewStatsService(gdb)
	err = svc.IngestStats(&machine, AgentStatsInput{
		MachineID: machine.ID,
		Stats: []AgentStatItem{
			{RuleID: "egress", BytesUp: 999, BytesDown: 999, ActiveConns: 9},
			{RuleID: rule.ID, BytesUp: 10, BytesDown: 20, ActiveConns: 2},
		},
	})
	if err != nil {
		t.Fatalf("ingest stats: %v", err)
	}

	var rows []paneldb.TrafficStat
	if err := gdb.Find(&rows).Error; err != nil {
		t.Fatalf("find stats: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one traffic stat row, got %d: %+v", len(rows), rows)
	}
	if rows[0].RuleID != rule.ID || rows[0].BytesUp != 10 || rows[0].BytesDown != 20 || rows[0].PeakConns != 2 {
		t.Fatalf("unexpected stat row: %+v", rows[0])
	}
}

func TestTodayTotalsForRulesAggregatesCurrentDayOnly(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	now := time.Date(2026, 5, 28, 15, 30, 0, 0, time.UTC)
	svc := NewStatsService(gdb)
	if err := gdb.Create(&[]paneldb.TrafficStat{
		{
			RuleID:    "rule-1",
			Hour:      now.Add(-2 * time.Hour).UTC().Truncate(time.Hour),
			BytesUp:   10,
			BytesDown: 20,
		},
		{
			RuleID:    "rule-1",
			Hour:      now.Add(-24 * time.Hour).UTC().Truncate(time.Hour),
			BytesUp:   100,
			BytesDown: 200,
		},
		{
			RuleID:    "rule-2",
			Hour:      now.UTC().Truncate(time.Hour),
			BytesUp:   7,
			BytesDown: 8,
		},
	}).Error; err != nil {
		t.Fatalf("create stats: %v", err)
	}

	totals, err := svc.TodayTotalsForRules([]string{"rule-1", "rule-2", "missing"}, now)
	if err != nil {
		t.Fatalf("today totals: %v", err)
	}
	if totals["rule-1"] != 30 {
		t.Fatalf("expected rule-1 total 30, got %d", totals["rule-1"])
	}
	if totals["rule-2"] != 15 {
		t.Fatalf("expected rule-2 total 15, got %d", totals["rule-2"])
	}
	if totals["missing"] != 0 {
		t.Fatalf("expected missing total 0, got %d", totals["missing"])
	}
}
