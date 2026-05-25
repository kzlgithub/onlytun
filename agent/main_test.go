package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/onlytun/agent/config"
	"github.com/onlytun/agent/reporter"
)

func TestAgentRuntimeSyncConfigStartsAndStopsIngressRules(t *testing.T) {
	pskHex := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	psk, err := (&config.Config{PSK: pskHex}).GetPSKBytes()
	if err != nil {
		t.Fatalf("decode psk: %v", err)
	}

	targetAddr, stopTarget := startMainTCPEchoServer(t)
	defer stopTarget()

	egressListenAddr := reserveMainTCPAddr(t)
	egressRuntime := &agentRuntime{
		cfg: &config.Config{
			Role:             "egress",
			PSK:              pskHex,
			TunnelListenAddr: egressListenAddr,
		},
		psk:      psk,
		reporter: reporter.NewReporter("http://127.0.0.1", "egress-machine", "egress", "token"),
		ingress:  make(map[string]*ingressRuntime),
	}
	if err := egressRuntime.applyEgressRules(); err != nil {
		t.Fatalf("start egress runtime: %v", err)
	}
	defer egressRuntime.shutdown()

	ingressListenAddr := reserveMainTCPAddr(t)

	var (
		panelMu sync.Mutex
		rules   []config.RuleConfig
	)
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panelMu.Lock()
		defer panelMu.Unlock()
		_ = json.NewEncoder(w).Encode(panelConfigResponse{Rules: rules})
	}))
	defer panel.Close()

	cfgPath := filepath.Join(t.TempDir(), "cache.json")
	initialCfg := &config.Config{
		MachineID: "ingress-machine",
		Role:      "ingress",
		PSK:       pskHex,
		PanelURL:  panel.URL,
		Token:     "panel-token",
		Rules:     nil,
	}
	if err := config.SaveConfig(cfgPath, initialCfg); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	runtime := &agentRuntime{
		cfgPath:    cfgPath,
		cfg:        initialCfg,
		psk:        psk,
		reporter:   reporter.NewReporter(panel.URL, initialCfg.MachineID, initialCfg.Role, initialCfg.Token),
		httpClient: panel.Client(),
		ingress:    make(map[string]*ingressRuntime),
	}
	defer runtime.shutdown()

	panelMu.Lock()
	rules = []config.RuleConfig{
		{
			RuleID:     "rule-1",
			ListenAddr: ingressListenAddr,
			Protocol:   "tcp",
			EgressAddr: egressListenAddr,
			TargetAddr: targetAddr,
		},
	}
	panelMu.Unlock()

	if err := runtime.syncConfig(context.Background()); err != nil {
		t.Fatalf("sync config start: %v", err)
	}
	if len(runtime.ingress) != 1 {
		t.Fatalf("expected one ingress runtime, got %d", len(runtime.ingress))
	}

	loaded, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load synced config: %v", err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].RuleID != "rule-1" {
		t.Fatalf("unexpected saved rules: %+v", loaded.Rules)
	}

	clientConn, err := net.Dial("tcp", ingressListenAddr)
	if err != nil {
		t.Fatalf("dial ingress runtime: %v", err)
	}
	payload := []byte("sync-config-e2e")
	if _, err := clientConn.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(clientConn, reply); err != nil {
		t.Fatalf("read reply: %v", err)
	}
	_ = clientConn.Close()
	if string(reply) != string(payload) {
		t.Fatalf("reply mismatch: got %q want %q", string(reply), string(payload))
	}

	panelMu.Lock()
	rules = nil
	panelMu.Unlock()

	if err := runtime.syncConfig(context.Background()); err != nil {
		t.Fatalf("sync config stop: %v", err)
	}
	if len(runtime.ingress) != 0 {
		t.Fatalf("expected ingress runtimes to be removed, got %d", len(runtime.ingress))
	}

	loaded, err = config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("reload synced config: %v", err)
	}
	if len(loaded.Rules) != 0 {
		t.Fatalf("expected saved rules to be empty, got %+v", loaded.Rules)
	}
}

func TestAgentRuntimeApplyEgressRulesUsesTunnelListenAddr(t *testing.T) {
	pskHex := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	psk, err := (&config.Config{PSK: pskHex}).GetPSKBytes()
	if err != nil {
		t.Fatalf("decode psk: %v", err)
	}

	listenAddr := reserveMainTCPAddr(t)
	runtime := &agentRuntime{
		cfg: &config.Config{
			Role:             "egress",
			PSK:              pskHex,
			TunnelListenAddr: listenAddr,
			Rules: []config.RuleConfig{
				{RuleID: "rule-1", ListenAddr: "127.0.0.1:6553"},
			},
		},
		psk:      psk,
		reporter: reporter.NewReporter("http://127.0.0.1", "egress-machine", "egress", "token"),
		ingress:  make(map[string]*ingressRuntime),
	}
	defer runtime.shutdown()

	if err := runtime.applyEgressRules(); err != nil {
		t.Fatalf("apply egress rules: %v", err)
	}
	if runtime.egress == nil {
		t.Fatal("expected egress tunnel to start")
	}
	if runtime.egressListenAddr != listenAddr {
		t.Fatalf("expected egress listen addr %s, got %s", listenAddr, runtime.egressListenAddr)
	}

	conn, err := net.DialTimeout("tcp", listenAddr, time.Second)
	if err != nil {
		t.Fatalf("dial egress listener: %v", err)
	}
	_ = conn.Close()
}

func TestAgentRuntimeStartInitialFailsWhenAllIngressRulesFail(t *testing.T) {
	pskHex := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	psk, err := (&config.Config{PSK: pskHex}).GetPSKBytes()
	if err != nil {
		t.Fatalf("decode psk: %v", err)
	}

	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve ingress port: %v", err)
	}
	defer blocker.Close()

	runtime := &agentRuntime{
		cfg: &config.Config{
			Role: "ingress",
			Rules: []config.RuleConfig{
				{
					RuleID:     "rule-1",
					ListenAddr: blocker.Addr().String(),
					Protocol:   "tcp",
					EgressAddr: "127.0.0.1:19999",
					TargetAddr: "example.com:443",
				},
			},
		},
		psk:      psk,
		reporter: reporter.NewReporter("http://127.0.0.1", "ingress-machine", "ingress", "token"),
		ingress:  make(map[string]*ingressRuntime),
	}
	defer runtime.shutdown()

	if err := runtime.startInitial(); err == nil {
		t.Fatal("expected startInitial to fail when all ingress rules fail")
	}
	if len(runtime.ingress) != 0 {
		t.Fatalf("expected no ingress runtimes, got %d", len(runtime.ingress))
	}
}

func startMainTCPEchoServer(t *testing.T) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp echo: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()

	return ln.Addr().String(), func() {
		_ = ln.Close()
		waitForMainDone(t, done)
	}
}

func reserveMainTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func waitForMainDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for helper goroutine to exit")
	}
}
