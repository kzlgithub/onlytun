package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	paneldb "github.com/onlytun/panel/db"
	"gorm.io/gorm"
)

func TestEnabledRulesForMachineExpandsDeviceGroupRule(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingressGroup := paneldb.MachineGroup{ID: "ingress-group", Name: "Ingress Group", Role: "ingress"}
	egressGroup := paneldb.MachineGroup{ID: "egress-group", Name: "Egress Group", Role: "egress"}
	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", GroupID: ingressGroup.ID, Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", GroupID: egressGroup.ID, Token: "token-egress", IP: "127.0.0.1", Online: true}
	rule := paneldb.DeviceGroupRule{
		ID:             "group-rule",
		Name:           "Group Rule",
		IngressGroupID: ingressGroup.ID,
		EgressGroupID:  egressGroup.ID,
		IngressPort:    41001,
		TargetAddr:     "example.com",
		TargetPort:     443,
		Protocol:       "tcp",
		Enabled:        true,
	}

	for _, record := range []any{&ingressGroup, &egressGroup, &ingress, &egress, &rule} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}
	enableDeviceGroupMode(t, gdb)

	configs, err := NewRuleService(gdb, 19999).EnabledRulesForMachine(&ingress)
	if err != nil {
		t.Fatalf("enabled rules: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected one config, got %+v", configs)
	}
	if configs[0].RuleID != rule.ID || configs[0].EgressAddr != "127.0.0.1:19999" {
		t.Fatalf("unexpected config: %+v", configs[0])
	}
}

func TestEnabledDeviceGroupRulesUseIXAdvertiseAddress(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingressGroup := paneldb.MachineGroup{ID: "ingress-group", Name: "Ingress Group", Role: "ingress"}
	egressGroup := paneldb.MachineGroup{ID: "egress-group", Name: "Egress Group", Role: "egress"}
	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", GroupID: ingressGroup.ID, Token: "token-ingress"}
	egress := paneldb.Machine{
		ID:                  "egress-ix",
		Name:                "IX",
		Role:                "egress",
		GroupID:             egressGroup.ID,
		Token:               "token-egress",
		IP:                  "89.43.141.163",
		IsIX:                true,
		TunnelAdvertiseAddr: "103.177.162.211:19999",
		Online:              true,
	}
	rule := paneldb.DeviceGroupRule{
		ID:             "group-rule",
		Name:           "Group Rule",
		IngressGroupID: ingressGroup.ID,
		EgressGroupID:  egressGroup.ID,
		IngressPort:    41001,
		TargetAddr:     "example.com",
		TargetPort:     443,
		Protocol:       "tcp",
		Enabled:        true,
	}

	for _, record := range []any{&ingressGroup, &egressGroup, &ingress, &egress, &rule} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}
	enableDeviceGroupMode(t, gdb)

	configs, err := NewRuleService(gdb, 19999).EnabledRulesForMachine(&ingress)
	if err != nil {
		t.Fatalf("enabled rules: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected one config, got %+v", configs)
	}
	if configs[0].EgressAddr != "103.177.162.211:19999" {
		t.Fatalf("expected IX advertise address, got %q", configs[0].EgressAddr)
	}
}

func TestDeviceGroupRuleSkipsIngressWhenSingleRuleOwnsPort(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingressGroup := paneldb.MachineGroup{ID: "ingress-group", Name: "Ingress Group", Role: "ingress"}
	egressGroup := paneldb.MachineGroup{ID: "egress-group", Name: "Egress Group", Role: "egress"}
	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", GroupID: ingressGroup.ID, Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", GroupID: egressGroup.ID, Token: "token-egress", IP: "127.0.0.1", Online: true}
	single := paneldb.ForwardRule{
		ID:               "single-rule",
		Name:             "Single Rule",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.org",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}
	groupRule := paneldb.DeviceGroupRule{
		ID:             "group-rule",
		Name:           "Group Rule",
		IngressGroupID: ingressGroup.ID,
		EgressGroupID:  egressGroup.ID,
		IngressPort:    41001,
		TargetAddr:     "example.com",
		TargetPort:     443,
		Protocol:       "tcp",
		Enabled:        true,
	}

	for _, record := range []any{&ingressGroup, &egressGroup, &ingress, &egress, &single, &groupRule} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}
	enableDeviceGroupMode(t, gdb)

	configs, err := NewRuleService(gdb, 19999).EnabledRulesForMachine(&ingress)
	if err != nil {
		t.Fatalf("enabled rules: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected group rule skipped because single rule owns port, got %+v", configs)
	}
}

func TestCreateDeviceGroupRuleDisablesOverlappingIngressPort(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingressGroup := paneldb.MachineGroup{ID: "ingress-group", Name: "Ingress Group", Role: "ingress"}
	egressGroup := paneldb.MachineGroup{ID: "egress-group", Name: "Egress Group", Role: "egress"}
	for _, record := range []any{&ingressGroup, &egressGroup} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}

	svc := NewGroupService(gdb, 19999)
	base := DeviceGroupRuleInput{
		Name:           "Base",
		IngressGroupID: ingressGroup.ID,
		EgressGroupID:  egressGroup.ID,
		IngressPort:    41001,
		TargetAddr:     "example.com",
		TargetPort:     443,
		Protocol:       "tcp",
		Enabled:        true,
	}
	if _, _, err := svc.CreateDeviceGroupRule(base); err != nil {
		t.Fatalf("create base rule: %v", err)
	}

	conflict := base
	conflict.Name = "Conflict"
	conflictRule, occupiedBy, err := svc.CreateDeviceGroupRule(conflict)
	if err != nil {
		t.Fatalf("create conflicting rule: %v", err)
	}
	if occupiedBy == nil || occupiedBy.Name != "Base" {
		t.Fatalf("expected conflict with base rule, got %+v", occupiedBy)
	}
	if conflictRule.Enabled {
		t.Fatalf("expected conflicting rule disabled in response, got %+v", conflictRule)
	}

	saved, err := svc.GetDeviceGroupRule(conflictRule.ID)
	if err != nil {
		t.Fatalf("load saved conflicting rule: %v", err)
	}
	if saved.Enabled {
		t.Fatalf("expected conflicting rule disabled in database, got %+v", saved)
	}

	if _, err := svc.ToggleDeviceGroupRule(saved.ID); !errors.Is(err, ErrGroupRulePortConflict) {
		t.Fatalf("expected toggle conflict error, got %v", err)
	}
}

func TestDeviceGroupModeDisabledUsesOnlySingleRules(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingressGroup := paneldb.MachineGroup{ID: "ingress-group", Name: "Ingress Group", Role: "ingress"}
	egressGroup := paneldb.MachineGroup{ID: "egress-group", Name: "Egress Group", Role: "egress"}
	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", GroupID: ingressGroup.ID, Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", GroupID: egressGroup.ID, Token: "token-egress", IP: "127.0.0.1", Online: true}
	single := paneldb.ForwardRule{
		ID:               "single-rule",
		Name:             "Single Rule",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.org",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}
	groupRule := paneldb.DeviceGroupRule{
		ID:             "group-rule",
		Name:           "Group Rule",
		IngressGroupID: ingressGroup.ID,
		EgressGroupID:  egressGroup.ID,
		IngressPort:    41002,
		TargetAddr:     "example.com",
		TargetPort:     443,
		Protocol:       "tcp",
		Enabled:        true,
	}

	for _, record := range []any{&ingressGroup, &egressGroup, &ingress, &egress, &single, &groupRule} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}

	configs, err := NewRuleService(gdb, 19999).EnabledRulesForMachine(&ingress)
	if err != nil {
		t.Fatalf("enabled rules: %v", err)
	}
	if len(configs) != 1 || configs[0].RuleID != single.ID {
		t.Fatalf("expected only single rule when device group mode is off, got %+v", configs)
	}
}

func TestDeviceGroupModeEnabledSuppressesSingleRules(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingressGroup := paneldb.MachineGroup{ID: "ingress-group", Name: "Ingress Group", Role: "ingress"}
	egressGroup := paneldb.MachineGroup{ID: "egress-group", Name: "Egress Group", Role: "egress"}
	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", GroupID: ingressGroup.ID, Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", GroupID: egressGroup.ID, Token: "token-egress", IP: "127.0.0.1", Online: true}
	single := paneldb.ForwardRule{
		ID:               "single-rule",
		Name:             "Single Rule",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.org",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}
	groupRule := paneldb.DeviceGroupRule{
		ID:             "group-rule",
		Name:           "Group Rule",
		IngressGroupID: ingressGroup.ID,
		EgressGroupID:  egressGroup.ID,
		IngressPort:    41002,
		TargetAddr:     "example.com",
		TargetPort:     443,
		Protocol:       "tcp",
		Enabled:        true,
	}

	for _, record := range []any{&ingressGroup, &egressGroup, &ingress, &egress, &single, &groupRule} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}
	enableDeviceGroupMode(t, gdb)

	configs, err := NewRuleService(gdb, 19999).EnabledRulesForMachine(&ingress)
	if err != nil {
		t.Fatalf("enabled rules: %v", err)
	}
	if len(configs) != 1 || configs[0].RuleID != groupRule.ID {
		t.Fatalf("expected only group rule when device group mode is on, got %+v", configs)
	}
}

func TestStatsAuthorizesDeviceGroupRuleByMachineGroup(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingressGroup := paneldb.MachineGroup{ID: "ingress-group", Name: "Ingress Group", Role: "ingress"}
	egressGroup := paneldb.MachineGroup{ID: "egress-group", Name: "Egress Group", Role: "egress"}
	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", GroupID: ingressGroup.ID, Token: "token-ingress"}
	rule := paneldb.DeviceGroupRule{
		ID:             "group-rule",
		Name:           "Group Rule",
		IngressGroupID: ingressGroup.ID,
		EgressGroupID:  egressGroup.ID,
		IngressPort:    41001,
		TargetAddr:     "example.com",
		TargetPort:     443,
		Protocol:       "tcp",
		Enabled:        true,
	}
	for _, record := range []any{&ingressGroup, &egressGroup, &ingress, &rule} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}

	err = NewStatsService(gdb).IngestStats(&ingress, AgentStatsInput{
		MachineID: ingress.ID,
		Stats: []AgentStatItem{{
			RuleID:      rule.ID,
			BytesUp:     10,
			BytesDown:   20,
			ActiveConns: 2,
		}},
	})
	if err != nil {
		t.Fatalf("ingest stats: %v", err)
	}

	var stat paneldb.TrafficStat
	if err := gdb.Take(&stat, "rule_id = ? AND hour = ?", rule.ID, time.Now().UTC().Truncate(time.Hour)).Error; err != nil {
		t.Fatalf("load stat: %v", err)
	}
	if stat.BytesUp != 10 || stat.BytesDown != 20 || stat.PeakConns != 2 {
		t.Fatalf("unexpected stat: %+v", stat)
	}
}

func enableDeviceGroupMode(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	if err := gdb.Create(&paneldb.SystemSetting{Key: DeviceGroupModeSettingKey, Value: "true"}).Error; err != nil {
		t.Fatalf("enable device group mode: %v", err)
	}
}
