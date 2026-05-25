package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

const reportInterval = 30 * time.Second

type heartbeatPayload struct {
	MachineID  string  `json:"machine_id"`
	Role       string  `json:"role"`
	IP         string  `json:"ip"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
}

// Reporter 负责向面板上报心跳和统计数据。
type Reporter struct {
	panelURL  string
	machineID string
	role      string
	token     string
	client    *http.Client

	mu      sync.RWMutex
	sources map[string]func() Stats
}

// NewReporter 创建 Reporter。
func NewReporter(panelURL, machineID, role, token string) *Reporter {
	return &Reporter{
		panelURL:  strings.TrimRight(panelURL, "/"),
		machineID: machineID,
		role:      role,
		token:     token,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		sources: make(map[string]func() Stats),
	}
}

// Start 开始定期上报（心跳和统计均每30秒一次），非阻塞。
func (r *Reporter) Start(ctx context.Context) {
	go r.runHeartbeatLoop(ctx)
	go r.runStatsLoop(ctx)
}

func (r *Reporter) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	r.sendHeartbeat(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.sendHeartbeat(ctx)
		}
	}
}

func (r *Reporter) sendHeartbeat(ctx context.Context) {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		log.Printf("[WARN] reporter heartbeat cpu sample failed: %v", err)
		return
	}

	memStats, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("[WARN] reporter heartbeat memory sample failed: %v", err)
		return
	}

	payload := heartbeatPayload{
		MachineID:  r.machineID,
		Role:       r.role,
		IP:         detectOutboundIP(),
		CPUPercent: firstCPUPercent(cpuPercent),
		MemPercent: memStats.UsedPercent,
	}

	if err := r.postJSON(ctx, "/api/agent/heartbeat", payload); err != nil {
		log.Printf("[WARN] reporter heartbeat post failed: %v", err)
	}
}

func detectOutboundIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 2*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()

	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

func firstCPUPercent(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func (r *Reporter) postJSON(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.panelURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if len(snippet) > 0 {
			return fmt.Errorf("panel returned status %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
		}
		return fmt.Errorf("panel returned status %s", resp.Status)
	}

	return nil
}
