package service

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	paneldb "github.com/onlytun/panel/db"
	"gorm.io/gorm"
)

var (
	ErrRuleNotFound     = errors.New("service: rule not found")
	ErrInvalidProtocol  = errors.New("service: invalid protocol")
	ErrInvalidMachine   = errors.New("service: invalid machine reference")
	ErrInvalidTarget    = errors.New("service: invalid target address")
	ErrMachineIPMissing = errors.New("service: egress machine has no IP")
	ErrRulePortConflict = errors.New("入口机端口已被占用")
)

type RulePortConflictError struct {
	Rule paneldb.ForwardRule
}

func (e *RulePortConflictError) Error() string {
	name := strings.TrimSpace(e.Rule.Name)
	if name == "" {
		name = e.Rule.ID
	}
	return fmt.Sprintf("入口机端口已被规则“%s”占用", name)
}

func (e *RulePortConflictError) Is(target error) bool {
	return target == ErrRulePortConflict
}

type RuleService struct {
	db         *gorm.DB
	tunnelPort int
}

type RuleInput struct {
	Name              string `json:"name"`
	IngressMachineID  string `json:"ingress_machine_id"`
	IngressPort       int    `json:"ingress_port"`
	EgressMachineID   string `json:"egress_machine_id"`
	TargetAddr        string `json:"target_addr"`
	TargetPort        int    `json:"target_port"`
	Protocol          string `json:"protocol"`
	TrafficLimitBytes int64  `json:"traffic_limit_bytes"`
	Enabled           bool   `json:"enabled"`
	Remark            string `json:"remark"`
}

type RuleRealtimeStat struct {
	BytesUp   int64 `json:"bytes_up"`
	BytesDown int64 `json:"bytes_down"`
	PeakConns int64 `json:"peak_conns"`
}

type RuleView struct {
	paneldb.ForwardRule
	RealtimeStat  RuleRealtimeStat `json:"realtime_stat"`
	TotalBytes    int64            `json:"total_bytes"`
	TodayBytes    int64            `json:"today_bytes"`
	LimitExceeded bool             `json:"limit_exceeded"`
}

func NewRuleService(gdb *gorm.DB, tunnelPort int) *RuleService {
	return &RuleService{db: gdb, tunnelPort: tunnelPort}
}

func (s *RuleService) ListRules() ([]paneldb.ForwardRule, error) {
	var rules []paneldb.ForwardRule
	if err := s.db.Order("created_at DESC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *RuleService) CreateRule(input RuleInput) (*paneldb.ForwardRule, error) {
	rule, err := s.buildRule("", input)
	if err != nil {
		return nil, err
	}
	if err := s.db.Select("*").Create(rule).Error; err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *RuleService) UpdateRule(id string, input RuleInput) (*paneldb.ForwardRule, error) {
	if _, err := s.GetRule(id); err != nil {
		return nil, err
	}

	rule, err := s.buildRule(id, input)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"name":                rule.Name,
		"ingress_machine_id":  rule.IngressMachineID,
		"ingress_port":        rule.IngressPort,
		"egress_machine_id":   rule.EgressMachineID,
		"target_addr":         rule.TargetAddr,
		"target_port":         rule.TargetPort,
		"protocol":            rule.Protocol,
		"traffic_limit_bytes": rule.TrafficLimitBytes,
		"enabled":             rule.Enabled,
		"remark":              rule.Remark,
	}
	if err := s.db.Model(&paneldb.ForwardRule{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetRule(id)
}

func (s *RuleService) DeleteRule(id string) error {
	result := s.db.Delete(&paneldb.ForwardRule{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRuleNotFound
	}
	return nil
}

func (s *RuleService) ToggleRule(id string) (*paneldb.ForwardRule, error) {
	rule, err := s.GetRule(id)
	if err != nil {
		return nil, err
	}
	nextEnabled := !rule.Enabled
	if nextEnabled {
		if err := s.ensureIngressPortAvailable(rule.ID, rule.IngressMachineID, rule.IngressPort, rule.Protocol); err != nil {
			return nil, err
		}
	}
	if err := s.db.Model(rule).Update("enabled", nextEnabled).Error; err != nil {
		return nil, err
	}
	return s.GetRule(id)
}

func (s *RuleService) GetRule(id string) (*paneldb.ForwardRule, error) {
	var rule paneldb.ForwardRule
	if err := s.db.Take(&rule, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRuleNotFound
		}
		return nil, err
	}
	return &rule, nil
}

func (s *RuleService) EnabledRulesForMachine(machine *paneldb.Machine) ([]AgentRuleConfig, error) {
	if machine == nil {
		return nil, ErrMachineNotFound
	}

	var rules []paneldb.ForwardRule
	if err := s.db.
		Where("enabled = ? AND (ingress_machine_id = ? OR egress_machine_id = ?)", true, machine.ID, machine.ID).
		Find(&rules).Error; err != nil {
		return nil, err
	}

	configs := make([]AgentRuleConfig, 0, len(rules))
	for _, rule := range rules {
		exceeded, err := s.ruleLimitExceeded(rule)
		if err != nil {
			return nil, err
		}
		if exceeded {
			continue
		}

		var egress paneldb.Machine
		if err := s.db.Take(&egress, "id = ?", rule.EgressMachineID).Error; err != nil {
			return nil, err
		}
		if strings.TrimSpace(egress.IP) == "" {
			return nil, ErrMachineIPMissing
		}

		configs = append(configs, AgentRuleConfig{
			RuleID:            rule.ID,
			ListenAddr:        net.JoinHostPort("0.0.0.0", strconv.Itoa(rule.IngressPort)),
			Protocol:          rule.Protocol,
			EgressAddr:        net.JoinHostPort(egress.IP, strconv.Itoa(s.tunnelPort)),
			TargetAddr:        net.JoinHostPort(rule.TargetAddr, strconv.Itoa(rule.TargetPort)),
			TrafficLimitBytes: rule.TrafficLimitBytes,
		})
	}

	sort.Slice(configs, func(i, j int) bool {
		return configs[i].RuleID < configs[j].RuleID
	})
	return configs, nil
}

func (s *RuleService) buildRule(id string, input RuleInput) (*paneldb.ForwardRule, error) {
	protocol := strings.ToLower(strings.TrimSpace(input.Protocol))
	if protocol != "tcp" && protocol != "udp" && protocol != "both" {
		return nil, ErrInvalidProtocol
	}
	if input.IngressPort <= 0 || input.IngressPort > 65535 || input.TargetPort <= 0 || input.TargetPort > 65535 {
		return nil, fmt.Errorf("service: invalid port")
	}
	if input.TrafficLimitBytes < 0 {
		return nil, fmt.Errorf("service: invalid traffic limit")
	}
	if strings.TrimSpace(input.TargetAddr) == "" {
		return nil, ErrInvalidTarget
	}

	var ingress paneldb.Machine
	if err := s.db.Take(&ingress, "id = ?", input.IngressMachineID).Error; err != nil || ingress.Role != "ingress" {
		return nil, ErrInvalidMachine
	}
	var egress paneldb.Machine
	if err := s.db.Take(&egress, "id = ?", input.EgressMachineID).Error; err != nil || egress.Role != "egress" {
		return nil, ErrInvalidMachine
	}

	if input.Enabled {
		if err := s.ensureIngressPortAvailable(id, input.IngressMachineID, input.IngressPort, protocol); err != nil {
			return nil, err
		}
	}

	return &paneldb.ForwardRule{
		ID:                id,
		Name:              strings.TrimSpace(input.Name),
		IngressMachineID:  input.IngressMachineID,
		IngressPort:       input.IngressPort,
		EgressMachineID:   input.EgressMachineID,
		TargetAddr:        strings.TrimSpace(input.TargetAddr),
		TargetPort:        input.TargetPort,
		Protocol:          protocol,
		TrafficLimitBytes: input.TrafficLimitBytes,
		Enabled:           input.Enabled,
		Remark:            strings.TrimSpace(input.Remark),
	}, nil
}

func (s *RuleService) ensureIngressPortAvailable(ruleID, ingressMachineID string, ingressPort int, protocol string) error {
	var rules []paneldb.ForwardRule
	query := s.db.Where("enabled = ? AND ingress_machine_id = ? AND ingress_port = ?", true, ingressMachineID, ingressPort)
	if strings.TrimSpace(ruleID) != "" {
		query = query.Where("id <> ?", ruleID)
	}
	if err := query.Find(&rules).Error; err != nil {
		return err
	}

	for _, rule := range rules {
		if protocolsOverlap(protocol, rule.Protocol) {
			return &RulePortConflictError{Rule: rule}
		}
	}
	return nil
}

func protocolsOverlap(left, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	if left == "both" || right == "both" {
		return true
	}
	return left == right
}

func (s *RuleService) ruleLimitExceeded(rule paneldb.ForwardRule) (bool, error) {
	if rule.TrafficLimitBytes <= 0 {
		return false, nil
	}
	total, err := s.totalTrafficForRule(rule.ID)
	if err != nil {
		return false, err
	}
	return total >= rule.TrafficLimitBytes, nil
}

func (s *RuleService) totalTrafficForRule(ruleID string) (int64, error) {
	var total int64
	err := s.db.Model(&paneldb.TrafficStat{}).
		Select("COALESCE(SUM(bytes_up + bytes_down), 0)").
		Where("rule_id = ?", ruleID).
		Scan(&total).Error
	return total, err
}

func (s *RuleService) TotalTrafficForRules(ruleIDs []string) (map[string]int64, error) {
	totals := make(map[string]int64, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return totals, nil
	}

	type row struct {
		RuleID string
		Total  int64
	}
	var rows []row
	if err := s.db.Model(&paneldb.TrafficStat{}).
		Select("rule_id, COALESCE(SUM(bytes_up + bytes_down), 0) AS total").
		Where("rule_id IN ?", ruleIDs).
		Group("rule_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, item := range rows {
		totals[item.RuleID] = item.Total
	}
	return totals, nil
}
