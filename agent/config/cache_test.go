package config

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
)

func TestSaveLoadConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nested", "cache.json")

	cfg := &Config{
		MachineID:        "machine-1",
		Role:             "egress",
		PSK:              "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
		PanelURL:         "http://127.0.0.1:8080",
		Token:            "token-1",
		TunnelListenAddr: "0.0.0.0:19999",
		Rules: []RuleConfig{
			{
				RuleID:     "rule-1",
				ListenAddr: "127.0.0.1:41001",
				Protocol:   "tcp",
				EgressAddr: "127.0.0.1:19999",
				TargetAddr: "example.com:443",
			},
		},
	}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if loaded.MachineID != cfg.MachineID ||
		loaded.Role != cfg.Role ||
		loaded.PSK != cfg.PSK ||
		loaded.PanelURL != cfg.PanelURL ||
		loaded.Token != cfg.Token ||
		loaded.TunnelListenAddr != cfg.TunnelListenAddr {
		t.Fatalf("loaded config mismatch: %+v", loaded)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0] != cfg.Rules[0] {
		t.Fatalf("loaded rules mismatch: %+v", loaded.Rules)
	}
}

func TestGetPSKBytesDecodesHex(t *testing.T) {
	cfg := &Config{
		PSK: "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
	}

	psk, err := cfg.GetPSKBytes()
	if err != nil {
		t.Fatalf("GetPSKBytes failed: %v", err)
	}
	if len(psk) != 32 {
		t.Fatalf("expected 32-byte psk, got %d", len(psk))
	}

	want := []byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,
	}
	if !bytes.Equal(psk, want) {
		t.Fatal("decoded psk mismatch")
	}
}

func TestGetPSKBytesRejectsInvalidLength(t *testing.T) {
	cfg := &Config{
		PSK: "00112233",
	}

	_, err := cfg.GetPSKBytes()
	if !errors.Is(err, errInvalidPSKLength) {
		t.Fatalf("expected invalid psk length error, got %v", err)
	}
}
