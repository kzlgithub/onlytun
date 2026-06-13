package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	gopsnet "github.com/shirou/gopsutil/v3/net"
)

const reportInterval = 30 * time.Second

type heartbeatPayload struct {
	MachineID           string  `json:"machine_id"`
	Role                string  `json:"role"`
	IP                  string  `json:"ip"`
	AgentVersion        string  `json:"agent_version"`
	IsIX                bool    `json:"is_ix"`
	TunnelAdvertiseAddr string  `json:"tunnel_advertise_addr"`
	CPUPercent          float64 `json:"cpu_percent"`
	MemPercent          float64 `json:"mem_percent"`
	DiskPercent         float64 `json:"disk_percent"`
	UptimeSec           uint64  `json:"uptime_seconds"`
	NetBytesUp          uint64  `json:"net_bytes_up"`
	NetBytesDown        uint64  `json:"net_bytes_down"`
}

// Reporter 负责向面板上报心跳和统计数据。
type Reporter struct {
	panelURL   string
	machineID  string
	role       string
	token      string
	accessAddr string
	version    string
	client     *http.Client

	mu                  sync.RWMutex
	isIX                bool
	tunnelAdvertiseAddr string
	sources             map[string]func() Stats
	last                map[string]Stats
}

// NewReporter 创建 Reporter。
func NewReporter(panelURL, machineID, role, token, accessAddr string, version ...string) *Reporter {
	reportVersion := ""
	if len(version) > 0 {
		reportVersion = version[0]
	}
	return &Reporter{
		panelURL:   strings.TrimRight(panelURL, "/"),
		machineID:  machineID,
		role:       role,
		token:      token,
		accessAddr: strings.TrimSpace(accessAddr),
		version:    strings.TrimSpace(reportVersion),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		sources: make(map[string]func() Stats),
		last:    make(map[string]Stats),
	}
}

// Start 开始定期上报（心跳和统计均每30秒一次），非阻塞。
func (r *Reporter) Start(ctx context.Context) {
	go r.runHeartbeatLoop(ctx)
	go r.runNetStatsLoop(ctx)
	go r.runStatsLoop(ctx)
}

func (r *Reporter) SetMachineMeta(isIX bool, tunnelAdvertiseAddr string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.isIX = isIX
	r.tunnelAdvertiseAddr = strings.TrimSpace(tunnelAdvertiseAddr)
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

	diskPercent := 0.0
	if diskStats, err := disk.Usage("/"); err == nil {
		diskPercent = diskStats.UsedPercent
	} else {
		log.Printf("[WARN] reporter heartbeat disk sample failed: %v", err)
	}

	uptimeSec, err := host.Uptime()
	if err != nil {
		log.Printf("[WARN] reporter heartbeat uptime sample failed: %v", err)
	}

	netBytesUp, netBytesDown := sampleNetBytes()
	r.mu.RLock()
	isIX := r.isIX
	tunnelAdvertiseAddr := r.tunnelAdvertiseAddr
	r.mu.RUnlock()

	payload := heartbeatPayload{
		MachineID:           r.machineID,
		Role:                r.role,
		IP:                  r.accessAddr,
		AgentVersion:        r.version,
		IsIX:                isIX,
		TunnelAdvertiseAddr: tunnelAdvertiseAddr,
		CPUPercent:          firstCPUPercent(cpuPercent),
		MemPercent:          memStats.UsedPercent,
		DiskPercent:         diskPercent,
		UptimeSec:           uptimeSec,
		NetBytesUp:          netBytesUp,
		NetBytesDown:        netBytesDown,
	}

	if err := r.postJSON(ctx, "/api/agent/heartbeat", payload); err != nil {
		log.Printf("[WARN] reporter heartbeat post failed: %v", err)
	}
}

func sampleNetBytes() (uint64, uint64) {
	bytesUp, bytesDown, err := sampleNetBytesWithError()
	if err != nil {
		log.Printf("[WARN] reporter heartbeat network sample failed: %v", err)
		return 0, 0
	}
	return bytesUp, bytesDown
}

func sampleNetBytesWithError() (uint64, uint64, error) {
	counters, err := gopsnet.IOCounters(true)
	if err != nil {
		return 0, 0, err
	}
	var bytesUp, bytesDown uint64
	for _, item := range counters {
		if item.Name == "lo" || strings.HasPrefix(item.Name, "lo:") {
			continue
		}
		bytesUp += item.BytesSent
		bytesDown += item.BytesRecv
	}
	return bytesUp, bytesDown, nil
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
