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

func TestCreateRuleRejectsOverlappingIngressPort(t *testing.T) {
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
	if _, err := svc.CreateRule(base); err != nil {
		t.Fatalf("create base rule: %v", err)
	}

	conflict := base
	conflict.Name = "Conflict"
	if _, err := svc.CreateRule(conflict); !errors.Is(err, ErrRulePortConflict) {
		t.Fatalf("expected ErrRulePortConflict, got %v", err)
	}

	udpSamePort := base
	udpSamePort.Name = "UDP Same Port"
	udpSamePort.Protocol = "udp"
	if _, err := svc.CreateRule(udpSamePort); err != nil {
		t.Fatalf("expected tcp and udp on same port to coexist, got %v", err)
	}

	bothConflict := base
	bothConflict.Name = "Both Conflict"
	bothConflict.Protocol = "both"
	if _, err := svc.CreateRule(bothConflict); !errors.Is(err, ErrRulePortConflict) {
		t.Fatalf("expected both protocol conflict, got %v", err)
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
	rule, err := svc.CreateRule(RuleInput{
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
