package service

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	paneldb "github.com/onlytun/panel/db"
	"gorm.io/gorm"
)

const installTokenTTL = 24 * time.Hour

var (
	ErrInstallTokenNotFound = errors.New("service: install token not found")
	ErrInstallTokenExpired  = errors.New("service: install token expired")
	ErrInstallTokenUsed     = errors.New("service: install token already used")
	ErrMachineNotFound      = errors.New("service: machine not found")
	ErrMachineHasRules      = errors.New("service: machine has enabled rules")
	ErrMachineNameRequired  = errors.New("service: machine name is required")
	ErrMachineOffline       = errors.New("service: machine is offline")
	ErrInvalidRole          = errors.New("service: invalid role")
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
	Name string
	Role string
	OS   string
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
}

type AgentUpdateTask struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
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

	var machine *paneldb.Machine
	var sharedPSK string
	err := s.db.Transaction(func(tx *gorm.DB) error {
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
			Name:          name,
			Role:          role,
			IP:            strings.TrimSpace(ip),
			Token:         token,
			Online:        true,
			OS:            strings.TrimSpace(input.OS),
			LastHeartbeat: now,
			OnlineSince:   now,
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
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrMachineNameRequired
	}

	var machine paneldb.Machine
	if err := s.db.Take(&machine, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMachineNotFound
		}
		return nil, err
	}

	if err := s.db.Model(&machine).Update("name", name).Error; err != nil {
		return nil, err
	}
	machine.Name = name
	return &machine, nil
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

func (s *MachineService) UpdateHeartbeat(machine *paneldb.Machine, role, ip string, cpuPercent, memPercent, diskPercent float64, uptimeSec, netBytesUp, netBytesDown uint64) error {
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
	if !machine.Online || machine.OnlineSince.IsZero() {
		updates["online_since"] = time.Now()
	}

	return s.db.Model(&paneldb.Machine{}).
		Where("id = ?", machine.ID).
		Updates(updates).Error
}

func (s *MachineService) MarkOfflineBefore(cutoff time.Time) error {
	return s.db.Model(&paneldb.Machine{}).
		Where("online = ? AND last_heartbeat < ?", true, cutoff).
		Update("online", false).Error
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

	baseURL = strings.TrimRight(baseURL, "/")
	return &InstallScriptPayload{
		IngressCommand: buildInstallCommand(baseURL, "ingress", ingressToken.Token, s.tunnelPort),
		EgressCommand:  buildInstallCommand(baseURL, "egress", egressToken.Token, s.tunnelPort),
	}, nil
}

func buildInstallCommand(baseURL, role, token string, tunnelPort int) string {
	_ = tunnelPort
	escapedBaseURL := shellQuote(baseURL)
	escapedToken := shellQuote(token)
	return fmt.Sprintf(
		"bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install.sh) --token %s --role %s --panel %s",
		escapedToken,
		role,
		escapedBaseURL,
	)
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
