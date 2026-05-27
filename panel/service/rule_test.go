package service

import (
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
