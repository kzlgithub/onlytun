package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onlytun/panel/service"
)

type heartbeatRequest struct {
	MachineID           string  `json:"machine_id"`
	Role                string  `json:"role"`
	IP                  string  `json:"ip"`
	AgentVersion        string  `json:"agent_version"`
	IsIX                *bool   `json:"is_ix"`
	TunnelAdvertiseAddr *string `json:"tunnel_advertise_addr"`
	CPUPercent          float64 `json:"cpu_percent"`
	MemPercent          float64 `json:"mem_percent"`
	DiskPercent         float64 `json:"disk_percent"`
	UptimeSec           uint64  `json:"uptime_seconds"`
	NetBytesUp          uint64  `json:"net_bytes_up"`
	NetBytesDown        uint64  `json:"net_bytes_down"`
}

type netStatsRequest struct {
	MachineID    string `json:"machine_id"`
	NetBytesUp   uint64 `json:"net_bytes_up"`
	NetBytesDown uint64 `json:"net_bytes_down"`
	NetUpBps     uint64 `json:"net_up_bps"`
	NetDownBps   uint64 `json:"net_down_bps"`
}

type statsRequest struct {
	MachineID string                  `json:"machine_id"`
	Stats     []service.AgentStatItem `json:"stats"`
}

type updateResultRequest struct {
	TaskID  string `json:"task_id"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type updateClaimRequest struct {
	TaskID string `json:"task_id"`
}

type registerRequest struct {
	Name                string `json:"name"`
	Role                string `json:"role"`
	IP                  string `json:"ip"`
	OS                  string `json:"os"`
	IsIX                bool   `json:"is_ix"`
	TunnelAdvertiseAddr string `json:"tunnel_advertise_addr"`
}

func (h *Handler) GetRuleStats(c *gin.Context) {
	series, err := h.Stats.GetSeries(c.Param("rule_id"), c.DefaultQuery("range", "day"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, series)
}

func (h *Handler) GetRecentTraffic(c *gin.Context) {
	scope := c.DefaultQuery("scope", "rules")
	days := 5
	if rawDays := strings.TrimSpace(c.Query("days")); rawDays != "" {
		parsed, err := strconv.Atoi(rawDays)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid days"})
			return
		}
		days = parsed
	}

	var ruleIDs []string
	switch scope {
	case "rules":
		rules, err := h.Rules.ListRules()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ruleIDs = make([]string, 0, len(rules))
		for _, rule := range rules {
			ruleIDs = append(ruleIDs, rule.ID)
		}
	case "group_rules":
		rules, err := h.Groups.ListDeviceGroupRules()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ruleIDs = make([]string, 0, len(rules))
		for _, rule := range rules {
			ruleIDs = append(ruleIDs, rule.ID)
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scope"})
		return
	}

	summary, err := h.Stats.RecentDailyTrafficForRules(ruleIDs, days, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"scope":      scope,
		"rule_count": len(ruleIDs),
		"points":     summary.Points,
		"total_up":   summary.TotalUp,
		"total_down": summary.TotalDown,
		"total":      summary.Total,
	})
}

func (h *Handler) AgentHeartbeat(c *gin.Context) {
	machine, ok := MachineFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req heartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.MachineID != "" && req.MachineID != machine.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "machine id mismatch"})
		return
	}

	ip := strings.TrimSpace(req.IP)
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "access address is required"})
		return
	}

	if err := h.Machines.UpdateHeartbeat(machine, req.Role, ip, req.AgentVersion, req.IsIX, req.TunnelAdvertiseAddr, req.CPUPercent, req.MemPercent, req.DiskPercent, req.UptimeSec, req.NetBytesUp, req.NetBytesDown); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidRole) || errors.Is(err, service.ErrInvalidTunnelAddr) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) AgentNetStats(c *gin.Context) {
	machine, ok := MachineFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req netStatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.MachineID != "" && req.MachineID != machine.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "machine id mismatch"})
		return
	}

	if err := h.Machines.UpdateNetStats(machine, req.NetBytesUp, req.NetBytesDown, req.NetUpBps, req.NetDownBps); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) AgentStats(c *gin.Context) {
	machine, ok := MachineFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req statsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := h.Stats.IngestStats(machine, service.AgentStatsInput{
		MachineID: req.MachineID,
		Stats:     req.Stats,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) AgentConfig(c *gin.Context) {
	machine, ok := MachineFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rules, err := h.Rules.EnabledRulesForMachine(machine)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updateTask, err := h.Machines.PendingUpdateForMachine(machine.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"rules":       rules,
		"update_task": updateTask,
	})
}

func (h *Handler) AgentClaimUpdate(c *gin.Context) {
	machine, ok := MachineFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req updateClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	task, err := h.Machines.ClaimMachineUpdate(machine.ID, req.TaskID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUpdateTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": task})
}

func (h *Handler) AgentUpdateResult(c *gin.Context) {
	machine, ok := MachineFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req updateResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := h.Machines.FinishMachineUpdate(machine.ID, service.MachineUpdateResult{
		TaskID:  req.TaskID,
		Success: req.Success,
		Error:   req.Error,
	}); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUpdateTaskNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) AgentRegister(c *gin.Context) {
	token, err := bearerToken(c.GetHeader("Authorization"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ip := strings.TrimSpace(req.IP)
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "access address is required"})
		return
	}

	machine, psk, err := h.Machines.RegisterMachine(token, service.RegisterMachineInput{
		Name:                req.Name,
		Role:                req.Role,
		OS:                  req.OS,
		IsIX:                req.IsIX,
		TunnelAdvertiseAddr: req.TunnelAdvertiseAddr,
	}, ip)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, service.ErrInstallTokenNotFound),
			errors.Is(err, service.ErrInstallTokenExpired),
			errors.Is(err, service.ErrInstallTokenUsed):
			status = http.StatusUnauthorized
		case errors.Is(err, service.ErrInvalidRole),
			errors.Is(err, service.ErrInvalidTunnelAddr),
			errors.Is(err, service.ErrMachineAccessAddrRequired):
			status = http.StatusBadRequest
		default:
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"machine_id": machine.ID,
		"psk":        psk,
	})
}
