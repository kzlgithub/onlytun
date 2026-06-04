package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/onlytun/agent/config"
	"github.com/onlytun/agent/reporter"
	"github.com/onlytun/agent/tunnel"
)

const (
	defaultConfigPath = "/etc/onlytun/cache.json"
	configSyncPeriod  = 15 * time.Second
)

var Version = "dev"

type panelConfigResponse struct {
	Rules      []config.RuleConfig `json:"rules"`
	UpdateTask *panelUpdateTask    `json:"update_task"`
}

type panelUpdateTask struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type updateResultRequest struct {
	TaskID  string `json:"task_id"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type updateClaimRequest struct {
	TaskID string `json:"task_id"`
}

type ingressRuntime struct {
	rule   config.RuleConfig
	tunnel *tunnel.IngressTunnel
}

type applyResult struct {
	started int
	active  int
	failed  map[string]error
}

type agentRuntime struct {
	cfgPath    string
	cfg        *config.Config
	psk        []byte
	reporter   *reporter.Reporter
	httpClient *http.Client

	mu      sync.Mutex
	ingress map[string]*ingressRuntime
	egress  *tunnel.EgressTunnel

	egressListenAddr string
}

func main() {
	log.SetFlags(log.LstdFlags)

	configPath := flag.String("config", defaultConfigPath, "path to config cache file")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(Version)
		return
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logErrorf("load config %q failed: %v", *configPath, err)
		os.Exit(1)
	}

	psk, err := cfg.GetPSKBytes()
	if err != nil {
		logErrorf("decode psk failed: %v", err)
		os.Exit(1)
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	runtime := &agentRuntime{
		cfgPath:  *configPath,
		cfg:      cfg,
		psk:      psk,
		reporter: reporter.NewReporter(cfg.PanelURL, cfg.MachineID, cfg.Role, cfg.Token, Version),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		ingress: make(map[string]*ingressRuntime),
	}
	runtime.reporter.SetMachineMeta(cfg.IsIX, cfg.TunnelAdvertiseAddr)

	if err := runtime.startInitial(); err != nil {
		logErrorf("startup failed: %v", err)
		runtime.shutdown()
		os.Exit(1)
	}

	runtime.reporter.Start(ctx)
	go runtime.runConfigSyncLoop(ctx)

	logInfof("agent started with role=%s", cfg.Role)
	<-ctx.Done()
	logInfof("shutdown signal received")
	runtime.shutdown()
}

func (a *agentRuntime) startInitial() error {
	switch strings.ToLower(a.cfg.Role) {
	case "ingress":
		result := a.applyIngressRules(a.cfg.Rules)
		if len(a.cfg.Rules) > 0 && result.active == 0 {
			return fmt.Errorf("failed to start all ingress rules: %s", formatRuleErrors(result.failed))
		}
	case "egress":
		if err := a.applyEgressRules(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported role %q", a.cfg.Role)
	}
	return nil
}

func (a *agentRuntime) runConfigSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(configSyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.syncConfig(ctx); err != nil {
				logWarnf("config sync failed: %v", err)
			}
		}
	}
}

func (a *agentRuntime) syncConfig(ctx context.Context) error {
	payload, err := a.fetchPanelConfig(ctx)
	if err != nil {
		return err
	}
	rules := payload.Rules

	a.mu.Lock()
	a.cfg.Rules = cloneRules(rules)
	cfgSnapshot := *a.cfg
	cfgSnapshot.Rules = cloneRules(rules)
	a.mu.Unlock()

	if err := config.SaveConfig(a.cfgPath, &cfgSnapshot); err != nil {
		logWarnf("save config cache failed: %v", err)
	}

	switch strings.ToLower(cfgSnapshot.Role) {
	case "ingress":
		result := a.applyIngressRules(rules)
		if len(rules) > 0 && result.active == 0 {
			return fmt.Errorf("all ingress rules are down after sync: %s", formatRuleErrors(result.failed))
		}
	case "egress":
		if err := a.applyEgressRules(); err != nil {
			return err
		}
	}

	if payload.UpdateTask != nil {
		a.handleUpdateTask(payload.UpdateTask)
	}

	return nil
}

func (a *agentRuntime) fetchPanelConfig(ctx context.Context) (*panelConfigResponse, error) {
	u, err := url.Parse(strings.TrimRight(a.cfg.PanelURL, "/") + "/api/agent/config")
	if err != nil {
		return nil, err
	}

	query := u.Query()
	query.Set("machine_id", a.cfg.MachineID)
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.Token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("panel returned status %s", resp.Status)
	}

	var payload panelConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func (a *agentRuntime) handleUpdateTask(task *panelUpdateTask) {
	if task == nil || strings.TrimSpace(task.ID) == "" {
		return
	}
	if task.Kind != "" && task.Kind != "agent" {
		a.reportUpdateResult(task.ID, false, "unsupported update task kind: "+task.Kind)
		return
	}

	if err := a.claimUpdateTask(task.ID); err != nil {
		logWarnf("agent update task %s claim failed: %v", task.ID, err)
		return
	}

	if err := scheduleAgentUpdate(a.cfg.PanelURL, a.cfg.Token, task.ID); err != nil {
		logWarnf("agent update task %s failed to schedule: %v", task.ID, err)
		a.reportUpdateResult(task.ID, false, err.Error())
		return
	}

	logInfof("agent update task %s scheduled", task.ID)
}

func (a *agentRuntime) claimUpdateTask(taskID string) error {
	body, err := json.Marshal(updateClaimRequest{TaskID: taskID})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpoint := strings.TrimRight(a.cfg.PanelURL, "/") + "/api/agent/update-claim"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("panel returned status %s", resp.Status)
	}
	return nil
}

func (a *agentRuntime) reportUpdateResult(taskID string, success bool, errText string) {
	body, err := json.Marshal(updateResultRequest{
		TaskID:  taskID,
		Success: success,
		Error:   errText,
	})
	if err != nil {
		logWarnf("encode update result failed: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpoint := strings.TrimRight(a.cfg.PanelURL, "/") + "/api/agent/update-result"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		logWarnf("build update result request failed: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		logWarnf("post update result failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logWarnf("post update result returned %s", resp.Status)
	}
}

func scheduleAgentUpdate(panelURL, token, taskID string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("agent update is only supported on linux systemd hosts")
	}

	script := `#!/bin/bash
set -u
LOG=/var/log/onlytun-agent-update.log
PANEL_URL=` + shellQuote(strings.TrimRight(panelURL, "/")) + `
TOKEN=` + shellQuote(token) + `
TASK_ID=` + shellQuote(taskID) + `

json_escape() {
  printf '%s' "$1" | sed ':a;N;$!ba;s/\\/\\\\/g;s/"/\\"/g;s/\n/\\n/g'
}

report_result() {
  local success="$1"
  local error_text="${2:-}"
  local escaped_error
  local attempt
  escaped_error="$(json_escape "$error_text")"
  for attempt in $(seq 1 20); do
    if curl -fsS -X POST "${PANEL_URL}/api/agent/update-result" \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{\"task_id\":\"${TASK_ID}\",\"success\":${success},\"error\":\"${escaped_error}\"}" >/dev/null 2>&1; then
      echo "[OK] update result reported on attempt ${attempt}"
      return 0
    fi
    echo "[WARN] update result report attempt ${attempt} failed"
    sleep 3
  done
  echo "[ERROR] update result report failed after retries"
  return 1
}

run_update() {
if [ -f /root/install.sh ]; then
  bash /root/install.sh --update
else
  bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install.sh) --update
fi
}

{
  echo "[INFO] onlytun agent update started at $(date -Is)"
  output="$(run_update 2>&1)"
  code=$?
  printf '%s\n' "$output"
  if [ "$code" -eq 0 ]; then
    for attempt in $(seq 1 30); do
      if systemctl is-active --quiet onlytun-agent; then
        break
      fi
      sleep 1
    done
    report_result true "" || true
    echo "[OK] onlytun agent update finished at $(date -Is)"
  else
    short_output="$(printf '%s' "$output" | tail -c 1200)"
    report_result false "agent update failed with exit ${code}: ${short_output}" || true
    echo "[ERROR] onlytun agent update failed with exit ${code}"
  fi
  rm -f "$0"
} >>"$LOG" 2>&1
`
	file, err := os.CreateTemp("", "onlytun-agent-update-*.sh")
	if err != nil {
		return err
	}
	path := file.Name()
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		_ = os.Remove(path)
		return err
	}

	unit := fmt.Sprintf("onlytun-agent-update-%d", time.Now().UnixNano())
	runScript := "/bin/bash " + shellQuote(path)
	command := "systemd-run --unit=" + shellQuote(unit) + " --property=Type=oneshot " + runScript + " >/dev/null 2>&1 || " +
		"nohup /bin/bash -c " + shellQuote("sleep 1\n"+runScript) + " >/dev/null 2>&1 &"
	return exec.Command("/bin/bash", "-c", command).Start()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func (a *agentRuntime) applyIngressRules(newRules []config.RuleConfig) applyResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := applyResult{
		failed: make(map[string]error),
	}

	newMap := make(map[string]config.RuleConfig, len(newRules))
	for _, rule := range newRules {
		newMap[rule.RuleID] = rule
	}

	for ruleID, runtime := range a.ingress {
		newRule, ok := newMap[ruleID]
		if !ok {
			logInfof("stopping removed ingress rule %s", ruleID)
			runtime.tunnel.Stop()
			a.reporter.UnregisterStatsSource(ruleID)
			delete(a.ingress, ruleID)
			continue
		}
		if sameRule(runtime.rule, newRule) {
			continue
		}

		logInfof("restarting modified ingress rule %s", ruleID)
		newRuntime, err := a.startIngressTunnel(newRule)
		if err == nil {
			runtime.tunnel.Stop()
			a.reporter.UnregisterStatsSource(ruleID)
			a.ingress[ruleID] = newRuntime
			result.started++
			continue
		}

		// If the new rule reuses the same listener, retry with a stop/start swap and roll back on failure.
		if runtime.rule.ListenAddr == newRule.ListenAddr {
			runtime.tunnel.Stop()
			a.reporter.UnregisterStatsSource(ruleID)
			delete(a.ingress, ruleID)

			tunnelInstance, restartErr := a.startIngressTunnel(newRule)
			if restartErr == nil {
				a.ingress[ruleID] = tunnelInstance
				result.started++
				continue
			}

			logErrorf("restart ingress rule %s failed: %v", ruleID, restartErr)
			result.failed[ruleID] = restartErr

			oldRuntime, rollbackErr := a.startIngressTunnel(runtime.rule)
			if rollbackErr != nil {
				logErrorf("rollback ingress rule %s failed: %v", ruleID, rollbackErr)
			} else {
				a.ingress[ruleID] = oldRuntime
			}
			continue
		}

		logErrorf("restart ingress rule %s failed: %v", ruleID, err)
		result.failed[ruleID] = err
	}

	for _, rule := range newRules {
		if _, ok := a.ingress[rule.RuleID]; ok {
			continue
		}

		tunnelInstance, err := a.startIngressTunnel(rule)
		if err != nil {
			logErrorf("start ingress rule %s failed: %v", rule.RuleID, err)
			result.failed[rule.RuleID] = err
			continue
		}
		a.ingress[rule.RuleID] = tunnelInstance
		result.started++
	}

	result.active = len(a.ingress)
	return result
}

func (a *agentRuntime) startIngressTunnel(rule config.RuleConfig) (*ingressRuntime, error) {
	tun := tunnel.NewIngressTunnel(tunnel.IngressConfig{
		ListenAddr: rule.ListenAddr,
		Protocol:   rule.Protocol,
		EgressAddr: rule.EgressAddr,
		TargetAddr: rule.TargetAddr,
		PSK:        append([]byte(nil), a.psk...),
		RuleID:     rule.RuleID,
	})
	if err := tun.Start(); err != nil {
		return nil, err
	}

	a.reporter.RegisterStatsSource(rule.RuleID, tun.GetStats)
	logInfof("ingress rule %s started on %s (%s)", rule.RuleID, rule.ListenAddr, rule.Protocol)
	return &ingressRuntime{rule: rule, tunnel: tun}, nil
}

func (a *agentRuntime) applyEgressRules() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	listenAddr := strings.TrimSpace(a.cfg.TunnelListenAddr)
	if listenAddr == "" {
		return fmt.Errorf("egress tunnel_listen_addr is empty")
	}

	if a.egress != nil && a.egressListenAddr == listenAddr {
		return nil
	}

	if a.egress != nil {
		logInfof("restarting egress listener on %s", listenAddr)
		a.egress.Stop()
		a.egress = nil
		a.egressListenAddr = ""
	}

	egress := tunnel.NewEgressTunnel(tunnel.EgressConfig{
		ListenAddr: listenAddr,
		PSK:        append([]byte(nil), a.psk...),
	})
	if err := egress.Start(); err != nil {
		return err
	}

	a.egress = egress
	a.egressListenAddr = listenAddr
	logInfof("egress listener started on %s", listenAddr)
	return nil
}

func (a *agentRuntime) shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for ruleID, runtime := range a.ingress {
		logInfof("stopping ingress rule %s", ruleID)
		runtime.tunnel.Stop()
		a.reporter.UnregisterStatsSource(ruleID)
		delete(a.ingress, ruleID)
	}

	if a.egress != nil {
		logInfof("stopping egress listener")
		a.egress.Stop()
		a.egress = nil
		a.egressListenAddr = ""
	}
}

func sameRule(a, b config.RuleConfig) bool {
	return a.RuleID == b.RuleID &&
		a.ListenAddr == b.ListenAddr &&
		a.Protocol == b.Protocol &&
		a.EgressAddr == b.EgressAddr &&
		a.TargetAddr == b.TargetAddr
}

func cloneRules(rules []config.RuleConfig) []config.RuleConfig {
	out := make([]config.RuleConfig, len(rules))
	copy(out, rules)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].RuleID < out[j].RuleID
	})
	return out
}

func formatRuleErrors(failed map[string]error) string {
	if len(failed) == 0 {
		return "no rule errors recorded"
	}

	keys := make([]string, 0, len(failed))
	for ruleID := range failed {
		keys = append(keys, ruleID)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, ruleID := range keys {
		parts = append(parts, fmt.Sprintf("%s: %v", ruleID, failed[ruleID]))
	}
	return strings.Join(parts, "; ")
}

func logInfof(format string, args ...any) {
	log.Printf("[INFO] "+format, args...)
}

func logWarnf(format string, args ...any) {
	log.Printf("[WARN] "+format, args...)
}

func logErrorf(format string, args ...any) {
	log.Printf("[ERROR] "+format, args...)
}
