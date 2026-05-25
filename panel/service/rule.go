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
)

type RuleService struct {
	db         *gorm.DB
	tunnelPort int
}

type RuleInput struct {
	Name             string `json:"name"`
	IngressMachineID string `json:"ingress_machine_id"`
	IngressPort      int    `json:"ingress_port"`
	EgressMachineID  string `json:"egress_machine_id"`
	TargetAddr       string `json:"target_addr"`
	TargetPort       int    `json:"target_port"`
	Protocol         string `json:"protocol"`
	Enabled          bool   `json:"enabled"`
	Remark           string `json:"remark"`
}

type RuleRealtimeStat struct {
	BytesUp   int64 `json:"bytes_up"`
	BytesDown int64 `json:"bytes_down"`
	PeakConns int64 `json:"peak_conns"`
}

type RuleView struct {
	paneldb.ForwardRule
	RealtimeStat RuleRealtimeStat `json:"realtime_stat"`
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
	if err := s.db.Create(rule).Error; err != nil {
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
	if err := s.db.Model(&paneldb.ForwardRule{}).Where("id = ?", id).Updates(rule).Error; err != nil {
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
	if err := s.db.Model(rule).Update("enabled", !rule.Enabled).Error; err != nil {
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
		var egress paneldb.Machine
		if err := s.db.Take(&egress, "id = ?", rule.EgressMachineID).Error; err != nil {
			return nil, err
		}
		if strings.TrimSpace(egress.IP) == "" {
			return nil, ErrMachineIPMissing
		}

		configs = append(configs, AgentRuleConfig{
			RuleID:     rule.ID,
			ListenAddr: net.JoinHostPort("0.0.0.0", strconv.Itoa(rule.IngressPort)),
			Protocol:   rule.Protocol,
			EgressAddr: net.JoinHostPort(egress.IP, strconv.Itoa(s.tunnelPort)),
			TargetAddr: net.JoinHostPort(rule.TargetAddr, strconv.Itoa(rule.TargetPort)),
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

	return &paneldb.ForwardRule{
		ID:               id,
		Name:             strings.TrimSpace(input.Name),
		IngressMachineID: input.IngressMachineID,
		IngressPort:      input.IngressPort,
		EgressMachineID:  input.EgressMachineID,
		TargetAddr:       strings.TrimSpace(input.TargetAddr),
		TargetPort:       input.TargetPort,
		Protocol:         protocol,
		Enabled:          input.Enabled,
		Remark:           strings.TrimSpace(input.Remark),
	}, nil
}
