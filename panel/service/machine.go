package service

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	paneldb "github.com/onlytun/panel/db"
	"gorm.io/gorm"
)

const (
	installTokenTTL   = 24 * time.Hour
	updateTaskTimeout = 20 * time.Minute
)

var (
	ErrInstallTokenNotFound = errors.New("service: install token not found")
	ErrInstallTokenExpired  = errors.New("service: install token expired")
	ErrInstallTokenUsed     = errors.New("service: install token already used")
	ErrMachineNotFound      = errors.New("service: machine not found")
	ErrMachineHasRules      = errors.New("service: machine has enabled rules")
	ErrMachineNameRequired  = errors.New("service: machine name is required")
	ErrMachineOffline       = errors.New("service: machine is offline")
	ErrInvalidRole          = errors.New("service: invalid role")
	ErrInvalidTunnelAddr    = errors.New("service: invalid tunnel advertise address")
	ErrUpdateTaskNotFound   = errors.New("service: update task not found")
)

type MachineService struct {
	db         *gorm.DB
	tunnelPort int
}

type MachineListItem struct {
	paneldb.Machine
	RuleCount      int64                      `json:"rule_count"`
	LastUpdateTask *paneldb.MachineUpdateTask `json:"last_update_task,omitempty"`
}

type RegisterMachineInput struct {
	Name                string
	Role                string
	OS                  string
	IsIX                bool
	TunnelAdvertiseAddr string
}

type AgentRuleConfig struct {
	RuleID            string `json:"rule_id"`
	ListenAddr        string `json:"listen_addr"`
	Protocol          string `json:"protocol"`
	EgressAddr        string `json:"egress_addr"`
	TargetAddr        string `json:"target_addr"`
	TrafficLimitBytes int64  `json:"traffic_limit_bytes"`
}

type InstallScriptPayload struct {
	IngressCommand string `json:"ingress_command"`
	EgressCommand  string `json:"egress_command"`
	IXCommand      string `json:"ix_command"`
}

type AgentUpdateTask struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type MachineUpdateInput struct {
	Name                string
	IsIX                *bool
	TunnelAdvertiseAddr *string
}

type MachineUpdateResult struct {
	TaskID  string
	Success bool
	Error   string
}

func NewMachineService(gdb *gorm.DB, tunnelPort int) *MachineService {
	return &MachineService{db: gdb, tunnelPort: tunnelPort}
}

func (s *MachineService) ListMachines() ([]MachineListItem, error) {
	if err := s.expireStaleUpdateTasks(time.Now()); err != nil {
		return nil, err
	}

	var machines []paneldb.Machine
	if err := s.db.Order("created_at DESC").Find(&machines).Error; err != nil {
		return nil, err
	}

	items := make([]MachineListItem, 0, len(machines))
	for _, machine := range machines {
		var ruleCount int64
		if err := s.db.Model(&paneldb.ForwardRule{}).
			Where("ingress_machine_id = ? OR egress_machine_id = ?", machine.ID, machine.ID).
			Count(&ruleCount).Error; err != nil {
			return nil, err
		}
		item := MachineListItem{
			Machine:   machine,
			RuleCount: ruleCount,
		}

		var task paneldb.MachineUpdateTask
		taskResult := s.db.
			Where("machine_id = ?", machine.ID).
			Order("created_at DESC").
			Limit(1).
			Find(&task)
		if taskResult.Error != nil {
			return nil, taskResult.Error
		}
		if taskResult.RowsAffected > 0 {
			taskCopy := task
			item.LastUpdateTask = &taskCopy
		}

		items = append(items, item)
	}

	return items, nil
}

func (s *MachineService) GenerateInstallToken() (*paneldb.InstallToken, error) {
	token, err := paneldb.GenerateHexSecret(24)
	if err != nil {
		return nil, err
	}

	record := &paneldb.InstallToken{
		Token:     token,
		ExpiresAt: time.Now().Add(installTokenTTL),
	}
	if err := s.db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func (s *MachineService) DeleteMachine(id string) error {
	var enabledCount int64
	if err := s.db.Model(&paneldb.ForwardRule{}).
		Where("(ingress_machine_id = ? OR egress_machine_id = ?) AND enabled = ?", id, id, true).
		Count(&enabledCount).Error; err != nil {
		return err
	}
	if enabledCount > 0 {
		return ErrMachineHasRules
	}

	result := s.db.Delete(&paneldb.Machine{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMachineNotFound
	}
	return nil
}

func (s *MachineService) RequestMachineUpdate(id string) (*paneldb.MachineUpdateTask, error) {
	if err := s.expireStaleUpdateTasks(time.Now()); err != nil {
		return nil, err
	}

	var task *paneldb.MachineUpdateTask
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var machine paneldb.Machine
		if err := tx.Take(&machine, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMachineNotFound
			}
			return err
		}
		if !machine.Online {
			return ErrMachineOffline
		}

		var existing paneldb.MachineUpdateTask
		err := tx.
			Where("machine_id = ? AND status IN ?", id, []string{"pending", "running"}).
			Order("created_at DESC").
			Limit(1).
			Find(&existing).Error
		if err != nil {
			return err
		}
		if existing.ID != "" {
			task = &existing
			return nil
		}

		record := &paneldb.MachineUpdateTask{
			MachineID:   id,
			Kind:        "agent",
			Status:      "pending",
			FromVersion: strings.TrimSpace(machine.AgentVersion),
			RequestedAt: time.Now(),
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		task = record
		return nil
	})
	return task, err
}

func (s *MachineService) PendingUpdateForMachine(machineID string) (*AgentUpdateTask, error) {
	if err := s.expireStaleUpdateTasks(time.Now()); err != nil {
		return nil, err
	}

	var task paneldb.MachineUpdateTask
	result := s.db.
		Where("machine_id = ? AND status = ?", machineID, "pending").
		Order("created_at ASC").
		Limit(1).
		Find(&task)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &AgentUpdateTask{ID: task.ID, Kind: task.Kind}, nil
}

func (s *MachineService) ClaimMachineUpdate(machineID, taskID string) (*AgentUpdateTask, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrUpdateTaskNotFound
	}

	var claimed *AgentUpdateTask
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var task paneldb.MachineUpdateTask
		err := tx.
			Where("id = ? AND machine_id = ? AND status = ?", taskID, machineID, "pending").
			Take(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUpdateTaskNotFound
		}
		if err != nil {
			return err
		}

		now := time.Now()
		if err := tx.Model(&paneldb.MachineUpdateTask{}).
			Where("id = ? AND status = ?", task.ID, "pending").
			Updates(map[string]any{
				"status":     "running",
				"started_at": &now,
			}).Error; err != nil {
			return err
		}
		claimed = &AgentUpdateTask{ID: task.ID, Kind: task.Kind}
		return nil
	})
	return claimed, err
}

func (s *MachineService) FinishMachineUpdate(machineID string, input MachineUpdateResult) error {
	if strings.TrimSpace(input.TaskID) == "" {
		return ErrUpdateTaskNotFound
	}

	status := "failed"
	errText := strings.TrimSpace(input.Error)
	if input.Success {
		status = "success"
		errText = ""
	}
	if len(errText) > 4000 {
		errText = errText[:4000]
	}

	now := time.Now()
	result := s.db.Model(&paneldb.MachineUpdateTask{}).
		Where("id = ? AND machine_id = ?", input.TaskID, machineID).
		Updates(map[string]any{
			"status":      status,
			"error":       errText,
			"finished_at": &now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUpdateTaskNotFound
	}
	return nil
}

func (s *MachineService) RegisterMachine(token string, input RegisterMachineInput, ip string) (*paneldb.Machine, string, error) {
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role != "ingress" && role != "egress" {
		return nil, "", ErrInvalidRole
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = strings.TrimSpace(ip)
	}
	if name == "" {
		name = fmt.Sprintf("%s-%d", role, time.Now().Unix())
	}

	isIX := input.IsIX && role == "egress"
	advertiseAddr, err := normalizeTunnelAdvertiseAddr(input.TunnelAdvertiseAddr)
	if err != nil {
		return nil, "", err
	}
	if isIX && advertiseAddr == "" {
		return nil, "", ErrInvalidTunnelAddr
	}
	if role != "egress" && advertiseAddr != "" {
		return nil, "", ErrInvalidTunnelAddr
	}

	var machine *paneldb.Machine
	var sharedPSK string
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var installToken paneldb.InstallToken
		if err := tx.Take(&installToken, "token = ?", token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInstallTokenNotFound
			}
			return err
		}
		if installToken.Used {
			return ErrInstallTokenUsed
		}
		if time.Now().After(installToken.ExpiresAt) {
			return ErrInstallTokenExpired
		}

		psk, err := getOrCreateSharedPSK(tx)
		if err != nil {
			return err
		}
		sharedPSK = psk

		now := time.Now()
		record := &paneldb.Machine{
			Name:                name,
			Role:                role,
			IP:                  strings.TrimSpace(ip),
			IsIX:                isIX,
			TunnelAdvertiseAddr: advertiseAddr,
			Token:               token,
			Online:              true,
			OS:                  strings.TrimSpace(input.OS),
			LastHeartbeat:       now,
			OnlineSince:         now,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		if err := tx.Model(&installToken).Updates(map[string]any{
			"used":       true,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}

		machine = record
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return machine, sharedPSK, nil
}

func (s *MachineService) UpdateMachineName(id, name string) (*paneldb.Machine, error) {
	return s.UpdateMachine(id, MachineUpdateInput{Name: name})
}

func (s *MachineService) UpdateMachine(id string, input MachineUpdateInput) (*paneldb.Machine, error) {
	var machine paneldb.Machine
	if err := s.db.Take(&machine, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMachineNotFound
		}
		return nil, err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = machine.Name
	}
	if strings.TrimSpace(name) == "" {
		return nil, ErrMachineNameRequired
	}

	updates := map[string]any{
		"name": name,
	}

	nextIsIX := machine.IsIX
	if input.IsIX != nil {
		nextIsIX = *input.IsIX && machine.Role == "egress"
		updates["is_ix"] = nextIsIX
	}

	nextAdvertiseAddr := machine.TunnelAdvertiseAddr
	if input.TunnelAdvertiseAddr != nil {
		advertiseAddr, err := normalizeTunnelAdvertiseAddr(*input.TunnelAdvertiseAddr)
		if err != nil {
			return nil, err
		}
		nextAdvertiseAddr = advertiseAddr
		updates["tunnel_advertise_addr"] = advertiseAddr
	}
	if nextIsIX && nextAdvertiseAddr == "" {
		return nil, ErrInvalidTunnelAddr
	}
	if machine.Role != "egress" && (nextIsIX || nextAdvertiseAddr != "") {
		return nil, ErrInvalidTunnelAddr
	}

	if err := s.db.Model(&machine).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.getMachine(id)
}

func (s *MachineService) AuthenticateMachineToken(token string) (*paneldb.Machine, error) {
	var machine paneldb.Machine
	if err := s.db.Take(&machine, "token = ?", token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMachineNotFound
		}
		return nil, err
	}
	return &machine, nil
}

func (s *MachineService) UpdateHeartbeat(machine *paneldb.Machine, role, ip, agentVersion string, isIX *bool, tunnelAdvertiseAddr *string, cpuPercent, memPercent, diskPercent float64, uptimeSec, netBytesUp, netBytesDown uint64) error {
	if machine == nil {
		return ErrMachineNotFound
	}

	role = strings.ToLower(strings.TrimSpace(role))
	if role != "" && role != machine.Role {
		return ErrInvalidRole
	}

	updateIP := strings.TrimSpace(ip)
	if updateIP == "" {
		updateIP = machine.IP
	}

	updates := map[string]any{
		"ip":             updateIP,
		"online":         true,
		"cpu_percent":    cpuPercent,
		"mem_percent":    memPercent,
		"disk_percent":   diskPercent,
		"uptime_seconds": uptimeSec,
		"net_bytes_up":   netBytesUp,
		"net_bytes_down": netBytesDown,
		"last_heartbeat": time.Now(),
	}
	if strings.TrimSpace(agentVersion) != "" {
		updates["agent_version"] = strings.TrimSpace(agentVersion)
	}
	nextIsIX := machine.IsIX
	if isIX != nil {
		nextIsIX = *isIX && machine.Role == "egress"
		updates["is_ix"] = nextIsIX
	}
	nextAdvertiseAddr := machine.TunnelAdvertiseAddr
	if tunnelAdvertiseAddr != nil {
		advertiseAddr, err := normalizeTunnelAdvertiseAddr(*tunnelAdvertiseAddr)
		if err != nil {
			return err
		}
		nextAdvertiseAddr = advertiseAddr
		updates["tunnel_advertise_addr"] = advertiseAddr
	}
	if nextIsIX && nextAdvertiseAddr == "" {
		return ErrInvalidTunnelAddr
	}
	if machine.Role != "egress" && (nextIsIX || nextAdvertiseAddr != "") {
		return ErrInvalidTunnelAddr
	}
	if !machine.Online || machine.OnlineSince.IsZero() {
		updates["online_since"] = time.Now()
	}

	if err := s.db.Model(&paneldb.Machine{}).
		Where("id = ?", machine.ID).
		Updates(updates).Error; err != nil {
		return err
	}

	return s.reconcileUpdateTaskFromHeartbeat(machine.ID, strings.TrimSpace(agentVersion), time.Now())
}

func (s *MachineService) MarkOfflineBefore(cutoff time.Time) error {
	return s.db.Model(&paneldb.Machine{}).
		Where("online = ? AND last_heartbeat < ?", true, cutoff).
		Update("online", false).Error
}

func (s *MachineService) reconcileUpdateTaskFromHeartbeat(machineID, agentVersion string, now time.Time) error {
	var task paneldb.MachineUpdateTask
	result := s.db.
		Where("machine_id = ? AND status = ?", machineID, "running").
		Order("started_at DESC, created_at DESC").
		Limit(1).
		Find(&task)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}

	if task.FromVersion != "" && agentVersion != "" && agentVersion != task.FromVersion {
		return s.finishUpdateTask(task.ID, machineID, "success", "", now)
	}
	if task.StartedAt != nil && now.Sub(*task.StartedAt) > updateTaskTimeout {
		return s.finishUpdateTask(task.ID, machineID, "failed", "agent update result timed out", now)
	}
	return nil
}

func (s *MachineService) expireStaleUpdateTasks(now time.Time) error {
	cutoff := now.Add(-updateTaskTimeout)
	return s.db.Model(&paneldb.MachineUpdateTask{}).
		Where("status IN ? AND updated_at < ?", []string{"pending", "running"}, cutoff).
		Updates(map[string]any{
			"status":      "failed",
			"error":       "update task timed out",
			"finished_at": &now,
		}).Error
}

func (s *MachineService) finishUpdateTask(taskID, machineID, status, errText string, now time.Time) error {
	return s.db.Model(&paneldb.MachineUpdateTask{}).
		Where("id = ? AND machine_id = ? AND status IN ?", taskID, machineID, []string{"pending", "running"}).
		Updates(map[string]any{
			"status":      status,
			"error":       errText,
			"finished_at": &now,
		}).Error
}

func (s *MachineService) BuildInstallScripts(baseURL string) (*InstallScriptPayload, error) {
	ingressToken, err := s.GenerateInstallToken()
	if err != nil {
		return nil, err
	}
	egressToken, err := s.GenerateInstallToken()
	if err != nil {
		return nil, err
	}
	ixToken, err := s.GenerateInstallToken()
	if err != nil {
		return nil, err
	}

	baseURL = strings.TrimRight(baseURL, "/")
	return &InstallScriptPayload{
		IngressCommand: buildInstallCommand(baseURL, "ingress", ingressToken.Token, nil),
		EgressCommand:  buildInstallCommand(baseURL, "egress", egressToken.Token, nil),
		IXCommand:      buildInstallCommand(baseURL, "egress", ixToken.Token, []string{"--ix"}),
	}, nil
}

func buildInstallCommand(baseURL, role, token string, extraArgs []string) string {
	escapedBaseURL := shellQuote(baseURL)
	escapedToken := shellQuote(token)
	command := fmt.Sprintf(
		"bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install.sh) --token %s --role %s --panel %s",
		escapedToken,
		role,
		escapedBaseURL,
	)
	for _, arg := range extraArgs {
		if strings.HasPrefix(arg, "--") {
			command += " " + arg
		} else {
			command += " " + shellQuote(arg)
		}
	}
	return command
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func requestBaseURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return strings.TrimRight(parsed.String(), "/")
}

func getOrCreateSharedPSK(tx *gorm.DB) (string, error) {
	var setting paneldb.SystemSetting
	err := tx.Take(&setting, "key = ?", paneldb.SharedPSKSettingKey).Error
	if err == nil {
		return setting.Value, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	psk, err := paneldb.GenerateHexSecret(32)
	if err != nil {
		return "", err
	}
	record := &paneldb.SystemSetting{
		Key:   paneldb.SharedPSKSettingKey,
		Value: psk,
	}
	if err := tx.Create(record).Error; err != nil {
		return "", err
	}
	return psk, nil
}

func JoinHostPort(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func EgressConnectAddr(machine paneldb.Machine, tunnelPort int) string {
	if addr := strings.TrimSpace(machine.TunnelAdvertiseAddr); addr != "" {
		return addr
	}
	if host := strings.TrimSpace(machine.IP); host != "" {
		return JoinHostPort(host, tunnelPort)
	}
	return ""
}

func normalizeTunnelAdvertiseAddr(value string) (string, error) {
	addr := strings.TrimSpace(value)
	if addr == "" {
		return "", nil
	}

	host, portText, err := net.SplitHostPort(addr)
	if err != nil || strings.TrimSpace(host) == "" {
		return "", ErrInvalidTunnelAddr
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return "", ErrInvalidTunnelAddr
	}
	return net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(port)), nil
}

func (s *MachineService) getMachine(id string) (*paneldb.Machine, error) {
	var machine paneldb.Machine
	if err := s.db.Take(&machine, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMachineNotFound
		}
		return nil, err
	}
	return &machine, nil
}
