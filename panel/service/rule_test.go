package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	paneldb "github.com/onlytun/panel/db"
)

func TestEnabledRulesForMachineExcludesExceededTrafficLimit(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", Token: "token-egress", IP: "127.0.0.1"}
	limited := paneldb.ForwardRule{
		ID:                "rule-limited",
		Name:              "Limited",
		IngressMachineID:  ingress.ID,
		IngressPort:       41001,
		EgressMachineID:   egress.ID,
		TargetAddr:        "example.com",
		TargetPort:        443,
		Protocol:          "tcp",
		Enabled:           true,
		TrafficLimitBytes: 100,
	}
	unlimited := paneldb.ForwardRule{
		ID:               "rule-unlimited",
		Name:             "Unlimited",
		IngressMachineID: ingress.ID,
		IngressPort:      41002,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.org",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}

	if err := gdb.Create(&ingress).Error; err != nil {
		t.Fatalf("create ingress: %v", err)
	}
	if err := gdb.Create(&egress).Error; err != nil {
		t.Fatalf("create egress: %v", err)
	}
	if err := gdb.Create(&limited).Error; err != nil {
		t.Fatalf("create limited rule: %v", err)
	}
	if err := gdb.Create(&unlimited).Error; err != nil {
		t.Fatalf("create unlimited rule: %v", err)
	}
	if err := gdb.Create(&paneldb.TrafficStat{
		RuleID:    limited.ID,
		Hour:      time.Now().UTC().Truncate(time.Hour),
		BytesUp:   60,
		BytesDown: 40,
	}).Error; err != nil {
		t.Fatalf("create stat: %v", err)
	}

	svc := NewRuleService(gdb, 19999)
	configs, err := svc.EnabledRulesForMachine(&ingress)
	if err != nil {
		t.Fatalf("enabled rules: %v", err)
	}
	if len(configs) != 1 || configs[0].RuleID != unlimited.ID {
		t.Fatalf("expected only unlimited rule, got %+v", configs)
	}
}

func TestEnabledRulesForMachineUsesIXAdvertiseAddress(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", Token: "token-ingress"}
	egress := paneldb.Machine{
		ID:                  "egress-ix",
		Name:                "IX",
		Role:                "egress",
		Token:               "token-egress",
		IP:                  "89.43.141.163",
		IsIX:                true,
		TunnelAdvertiseAddr: "103.177.162.211:19999",
	}
	rule := paneldb.ForwardRule{
		ID:               "rule-ix",
		Name:             "IX Rule",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.com",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}
	for _, record := range []any{&ingress, &egress, &rule} {
		if err := gdb.Create(record).Error; err != nil {
			t.Fatalf("create record: %v", err)
		}
	}

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

func TestCreateRuleDisablesOverlappingIngressPort(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", Token: "token-egress", IP: "127.0.0.1"}
	if err := gdb.Create(&ingress).Error; err != nil {
		t.Fatalf("create ingress: %v", err)
	}
	if err := gdb.Create(&egress).Error; err != nil {
		t.Fatalf("create egress: %v", err)
	}

	svc := NewRuleService(gdb, 19999)
	base := RuleInput{
		Name:             "Base",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.com",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}
	if _, _, err := svc.CreateRule(base); err != nil {
		t.Fatalf("create base rule: %v", err)
	}

	conflict := base
	conflict.Name = "Conflict"
	conflictRule, occupiedBy, err := svc.CreateRule(conflict)
	if err != nil {
		t.Fatalf("create conflicting rule should succeed as disabled: %v", err)
	}
	if conflictRule.Enabled {
		t.Fatal("conflicting rule should be saved as disabled")
	}
	if occupiedBy == nil || occupiedBy.Name != "Base" {
		t.Fatalf("expected conflict rule Base, got %#v", occupiedBy)
	}

	udpSamePort := base
	udpSamePort.Name = "UDP Same Port"
	udpSamePort.Protocol = "udp"
	if udpRule, occupiedBy, err := svc.CreateRule(udpSamePort); err != nil {
		t.Fatalf("expected tcp and udp on same port to coexist, got %v", err)
	} else if !udpRule.Enabled || occupiedBy != nil {
		t.Fatalf("expected udp rule enabled without conflict, got rule=%+v conflict=%+v", udpRule, occupiedBy)
	}

	bothConflict := base
	bothConflict.Name = "Both Conflict"
	bothConflict.Protocol = "both"
	if bothRule, occupiedBy, err := svc.CreateRule(bothConflict); err != nil {
		t.Fatalf("create both conflict should succeed as disabled: %v", err)
	} else if bothRule.Enabled || occupiedBy == nil {
		t.Fatalf("expected both conflict disabled with conflict rule, got rule=%+v conflict=%+v", bothRule, occupiedBy)
	}
}

func TestCreateRuleAllowsDisabledOverlappingIngressPort(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", Token: "token-egress", IP: "127.0.0.1"}
	if err := gdb.Create(&ingress).Error; err != nil {
		t.Fatalf("create ingress: %v", err)
	}
	if err := gdb.Create(&egress).Error; err != nil {
		t.Fatalf("create egress: %v", err)
	}

	svc := NewRuleService(gdb, 19999)
	base := RuleInput{
		Name:             "Base",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.com",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}
	if _, _, err := svc.CreateRule(base); err != nil {
		t.Fatalf("create base rule: %v", err)
	}

	disabled := base
	disabled.Name = "Disabled Conflict"
	disabled.Enabled = false
	if _, conflict, err := svc.CreateRule(disabled); err != nil {
		t.Fatalf("disabled conflict should be saved: %v", err)
	} else if conflict != nil {
		t.Fatalf("explicitly disabled rule should not report conflict, got %+v", conflict)
	}
}

func TestToggleRuleRejectsOverlappingIngressPort(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", Token: "token-egress", IP: "127.0.0.1"}
	if err := gdb.Create(&ingress).Error; err != nil {
		t.Fatalf("create ingress: %v", err)
	}
	if err := gdb.Create(&egress).Error; err != nil {
		t.Fatalf("create egress: %v", err)
	}

	svc := NewRuleService(gdb, 19999)
	if _, _, err := svc.CreateRule(RuleInput{
		Name:             "Enabled",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.com",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	}); err != nil {
		t.Fatalf("create enabled rule: %v", err)
	}
	disabled, _, err := svc.CreateRule(RuleInput{
		Name:             "Disabled",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.org",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          false,
	})
	if err != nil {
		t.Fatalf("create disabled rule: %v", err)
	}
	savedDisabled, err := svc.GetRule(disabled.ID)
	if err != nil {
		t.Fatalf("get disabled rule: %v", err)
	}
	if savedDisabled.Enabled {
		t.Fatal("disabled rule should be persisted as disabled")
	}

	if _, err := svc.ToggleRule(disabled.ID); !errors.Is(err, ErrRulePortConflict) {
		t.Fatalf("expected toggle conflict, got %v", err)
	}
}

func TestUpdateRuleAllowsOwnIngressPort(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ingress := paneldb.Machine{ID: "ingress-1", Name: "Ingress", Role: "ingress", Token: "token-ingress"}
	egress := paneldb.Machine{ID: "egress-1", Name: "Egress", Role: "egress", Token: "token-egress", IP: "127.0.0.1"}
	if err := gdb.Create(&ingress).Error; err != nil {
		t.Fatalf("create ingress: %v", err)
	}
	if err := gdb.Create(&egress).Error; err != nil {
		t.Fatalf("create egress: %v", err)
	}

	svc := NewRuleService(gdb, 19999)
	rule, _, err := svc.CreateRule(RuleInput{
		Name:             "Base",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.com",
		TargetPort:       443,
		Protocol:         "tcp",
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	if _, err := svc.UpdateRule(rule.ID, RuleInput{
		Name:             "Renamed",
		IngressMachineID: ingress.ID,
		IngressPort:      41001,
		EgressMachineID:  egress.ID,
		TargetAddr:       "example.org",
		TargetPort:       8443,
		Protocol:         "tcp",
		Enabled:          true,
	}); err != nil {
		t.Fatalf("update own rule port: %v", err)
	}
}
