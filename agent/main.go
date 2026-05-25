package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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
	configSyncPeriod  = 60 * time.Second
	egressStatsRuleID = "egress"
)

type panelConfigResponse struct {
	Rules []config.RuleConfig `json:"rules"`
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
	flag.Parse()

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
		reporter: reporter.NewReporter(cfg.PanelURL, cfg.MachineID, cfg.Role, cfg.Token),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		ingress: make(map[string]*ingressRuntime),
	}

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
	rules, err := a.fetchPanelRules(ctx)
	if err != nil {
		return err
	}

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

	return nil
}

func (a *agentRuntime) fetchPanelRules(ctx context.Context) ([]config.RuleConfig, error) {
	u, err := url.Parse(strings.TrimRight(a.cfg.PanelURL, "/") + "/api/agent/config")
	if err != nil {
		return nil, err
	}

	query := u.Query()
	query.Set("machine_id", a.cfg.MachineID)
	query.Set("token", a.cfg.Token)
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

	return payload.Rules, nil
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
		a.reporter.UnregisterStatsSource(egressStatsRuleID)
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
	a.reporter.RegisterStatsSource(egressStatsRuleID, egress.GetStats)
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
		a.reporter.UnregisterStatsSource(egressStatsRuleID)
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
