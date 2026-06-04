package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	paneldb "github.com/onlytun/panel/db"
)

func TestNormalizeTunnelAdvertiseAddr(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "ipv4", input: "103.177.162.211:19999", want: "103.177.162.211:19999"},
		{name: "domain", input: "ix.example.com:19999", want: "ix.example.com:19999"},
		{name: "ipv6", input: "[2001:db8::1]:19999", want: "[2001:db8::1]:19999"},
		{name: "missing port", input: "103.177.162.211", wantErr: true},
		{name: "empty host", input: ":19999", wantErr: true},
		{name: "bad port", input: "103.177.162.211:abc", wantErr: true},
		{name: "port out of range", input: "103.177.162.211:70000", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeTunnelAdvertiseAddr(tc.input)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidTunnelAddr) {
					t.Fatalf("expected ErrInvalidTunnelAddr, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestMachineUpdateCompletesWhenHeartbeatReportsNewVersion(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	machine := paneldb.Machine{
		ID:           "machine-1",
		Name:         "Ingress",
		Role:         "ingress",
		Token:        "token-1",
		Online:       true,
		AgentVersion: "v1.5.0",
	}
	if err := gdb.Create(&machine).Error; err != nil {
		t.Fatalf("create machine: %v", err)
	}

	svc := NewMachineService(gdb, 19999)
	task, err := svc.RequestMachineUpdate(machine.ID)
	if err != nil {
		t.Fatalf("request update: %v", err)
	}
	if task.FromVersion != "v1.5.0" {
		t.Fatalf("expected from version v1.5.0, got %q", task.FromVersion)
	}
	if _, err := svc.ClaimMachineUpdate(machine.ID, task.ID); err != nil {
		t.Fatalf("claim update: %v", err)
	}
	if err := svc.UpdateHeartbeat(&machine, "ingress", "127.0.0.1", "v1.5.1", nil, nil, 1, 2, 3, 4, 5, 6); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	var stored paneldb.MachineUpdateTask
	if err := gdb.Take(&stored, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != "success" {
		t.Fatalf("expected task success after version change, got %q", stored.Status)
	}
	if stored.FinishedAt == nil {
		t.Fatalf("expected task finished_at to be set")
	}
}

func TestMachineUpdateTimesOutWithoutResult(t *testing.T) {
	gdb, err := paneldb.OpenDatabase(filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	startedAt := time.Now().Add(-(updateTaskTimeout + time.Minute))
	task := paneldb.MachineUpdateTask{
		ID:          "task-1",
		MachineID:   "machine-1",
		Kind:        "agent",
		Status:      "running",
		FromVersion: "v1.5.0",
		StartedAt:   &startedAt,
	}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc := NewMachineService(gdb, 19999)
	if err := svc.reconcileUpdateTaskFromHeartbeat(task.MachineID, "v1.5.0", time.Now()); err != nil {
		t.Fatalf("reconcile task: %v", err)
	}

	var stored paneldb.MachineUpdateTask
	if err := gdb.Take(&stored, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != "failed" {
		t.Fatalf("expected timeout task failed, got %q", stored.Status)
	}
	if stored.Error == "" {
		t.Fatalf("expected timeout error")
	}
}
