package service

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	paneldb "github.com/onlytun/panel/db"
	"gorm.io/gorm"
)

var (
	ErrGroupNotFound         = errors.New("service: machine group not found")
	ErrGroupNameRequired     = errors.New("service: machine group name is required")
	ErrGroupHasRules         = errors.New("service: machine group has rules")
	ErrGroupRuleNotFound     = errors.New("service: device group rule not found")
	ErrGroupRulePortConflict = errors.New("service: device group rule port conflict")
)

type GroupService struct {
	db         *gorm.DB
	tunnelPort int
}

type MachineGroupInput struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Remark string `json:"remark"`
}

type GroupMembersInput struct {
	MachineIDs []string `json:"machine_ids"`
}

type MachineGroupView struct {
	paneldb.MachineGroup
	MachineCount int64 `json:"machine_count"`
}

type DeviceGroupRuleInput struct {
	Name              string `json:"name"`
	IngressGroupID    string `json:"ingress_group_id"`
	EgressGroupID     string `json:"egress_group_id"`
	IngressPort       int    `json:"ingress_port"`
	TargetAddr        string `json:"target_addr"`
	TargetPort        int    `json:"target_port"`
	Protocol          string `json:"protocol"`
	TrafficLimitBytes int64  `json:"traffic_limit_bytes"`
	Enabled           bool   `json:"enabled"`
	Remark            string `json:"remark"`
}

type DeviceGroupRuleView struct {
	paneldb.DeviceGroupRule
	IngressGroupName    string           `json:"ingress_group_name"`
	EgressGroupName     string           `json:"egress_group_name"`
	IngressMachineCount int64            `json:"ingress_machine_count"`
	EffectiveMachines   int64            `json:"effective_machines"`
	ConflictMachines    int64            `json:"conflict_machines"`
	OnlineEgressCount   int64            `json:"online_egress_count"`
	RealtimeStat        RuleRealtimeStat `json:"realtime_stat"`
	TotalBytes          int64            `json:"total_bytes"`
	TodayBytes          int64            `json:"today_bytes"`
	LimitExceeded       bool             `json:"limit_exceeded"`
}

type DeviceGroupRulePortConflictError struct {
	Rule paneldb.DeviceGroupRule
}

func (e *DeviceGroupRulePortConflictError) Error() string {
	name := strings.TrimSpace(e.Rule.Name)
	if name == "" {
		name = e.Rule.ID
	}
	return fmt.Sprintf("device group ingress port is already used by group rule %q", name)
}

func (e *DeviceGroupRulePortConflictError) Is(target error) bool {
	return target == ErrGroupRulePortConflict
}

func NewGroupService(gdb *gorm.DB, tunnelPort int) *GroupService {
	return &GroupService{db: gdb, tunnelPort: tunnelPort}
}

func (s *GroupService) ListGroups(role string) ([]MachineGroupView, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	var groups []paneldb.MachineGroup
	query := s.db.Order("created_at DESC")
	if role != "" {
		query = query.Where("role = ?", role)
	}
	if err := query.Find(&groups).Error; err != nil {
		return nil, err
	}

	views := make([]MachineGroupView, 0, len(groups))
	for _, group := range groups {
		var count int64
		if err := s.db.Model(&paneldb.Machine{}).Where("group_id = ?", group.ID).Count(&count).Error; err != nil {
			return nil, err
		}
		views = append(views, MachineGroupView{MachineGroup: group, MachineCount: count})
	}
	return views, nil
}

func (s *GroupService) CreateGroup(input MachineGroupInput) (*paneldb.MachineGroup, error) {
	group, err := buildMachineGroup(input)
	if err != nil {
		return nil, err
	}
	if err := s.db.Create(group).Error; err != nil {
		return nil, err
	}
	return group, nil
}

func (s *GroupService) UpdateGroup(id string, input MachineGroupInput) (*paneldb.MachineGroup, error) {
	group, err := s.GetGroup(id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrGroupNameRequired
	}
	updates := map[string]any{
		"name":   name,
		"remark": strings.TrimSpace(input.Remark),
	}
	if err := s.db.Model(group).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetGroup(id)
}

func (s *GroupService) DeleteGroup(id string) error {
	group, err := s.GetGroup(id)
	if err != nil {
		return err
	}
	var ruleCount int64
	query := s.db.Model(&paneldb.DeviceGroupRule{})
	if group.Role == "ingress" {
		query = query.Where("ingress_group_id = ?", id)
	} else {
		query = query.Where("egress_group_id = ?", id)
	}
	if err := query.Count(&ruleCount).Error; err != nil {
		return err
	}
	if ruleCount > 0 {
		return ErrGroupHasRules
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&paneldb.Machine{}).Where("group_id = ?", id).Update("group_id", "").Error; err != nil {
			return err
		}
		result := tx.Delete(&paneldb.MachineGroup{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrGroupNotFound
		}
		return nil
	})
}

func (s *GroupService) SetGroupMembers(id string, input GroupMembersInput) (*MachineGroupView, error) {
	group, err := s.GetGroup(id)
	if err != nil {
		return nil, err
	}

	ids := uniqueStrings(input.MachineIDs)
	var machines []paneldb.Machine
	if len(ids) > 0 {
		if err := s.db.Where("id IN ?", ids).Find(&machines).Error; err != nil {
			return nil, err
		}
		if len(machines) != len(ids) {
			return nil, ErrInvalidMachine
		}
		for _, machine := range machines {
			if machine.Role != group.Role {
				return nil, ErrInvalidMachine
			}
		}
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&paneldb.Machine{}).Where("group_id = ?", id).Update("group_id", "").Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := tx.Model(&paneldb.Machine{}).Where("id IN ?", ids).Update("group_id", id).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	views, err := s.ListGroups(group.Role)
	if err != nil {
		return nil, err
	}
	for _, view := range views {
		if view.ID == id {
			return &view, nil
		}
	}
	return nil, ErrGroupNotFound
}

func (s *GroupService) GetGroup(id string) (*paneldb.MachineGroup, error) {
	var group paneldb.MachineGroup
	if err := s.db.Take(&group, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	return &group, nil
}

func (s *GroupService) ListDeviceGroupRules() ([]paneldb.DeviceGroupRule, error) {
	var rules []paneldb.DeviceGroupRule
	if err := s.db.Order("created_at DESC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *GroupService) CreateDeviceGroupRule(input DeviceGroupRuleInput) (*paneldb.DeviceGroupRule, *paneldb.DeviceGroupRule, error) {
	rule, err := s.buildDeviceGroupRule("", input)
	if err != nil {
		return nil, nil, err
	}
	var conflict *paneldb.DeviceGroupRule
	if input.Enabled {
		conflict, err = s.findGroupRuleConflict("", rule.IngressGroupID, rule.IngressPort, rule.Protocol)
		if err != nil {
			return nil, nil, err
		}
		if conflict != nil {
			rule.Enabled = false
		}
	}
	if err := s.db.Create(rule).Error; err != nil {
		return nil, nil, err
	}
	return rule, conflict, nil
}

func (s *GroupService) UpdateDeviceGroupRule(id string, input DeviceGroupRuleInput) (*paneldb.DeviceGroupRule, error) {
	if _, err := s.GetDeviceGroupRule(id); err != nil {
		return nil, err
	}
	rule, err := s.buildDeviceGroupRule(id, input)
	if err != nil {
		return nil, err
	}
	if input.Enabled {
		if conflict, err := s.findGroupRuleConflict(id, rule.IngressGroupID, rule.IngressPort, rule.Protocol); err != nil {
			return nil, err
		} else if conflict != nil {
			return nil, &DeviceGroupRulePortConflictError{Rule: *conflict}
		}
	}
	if err := s.db.Model(&paneldb.DeviceGroupRule{}).Where("id = ?", id).Updates(map[string]any{
		"name":                rule.Name,
		"ingress_group_id":    rule.IngressGroupID,
		"egress_group_id":     rule.EgressGroupID,
		"ingress_port":        rule.IngressPort,
		"target_addr":         rule.TargetAddr,
		"target_port":         rule.TargetPort,
		"protocol":            rule.Protocol,
		"traffic_limit_bytes": rule.TrafficLimitBytes,
		"enabled":             rule.Enabled,
		"remark":              rule.Remark,
	}).Error; err != nil {
		return nil, err
	}
	return s.GetDeviceGroupRule(id)
}

func (s *GroupService) DeleteDeviceGroupRule(id string) error {
	result := s.db.Delete(&paneldb.DeviceGroupRule{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupRuleNotFound
	}
	return nil
}

func (s *GroupService) ToggleDeviceGroupRule(id string) (*paneldb.DeviceGroupRule, error) {
	rule, err := s.GetDeviceGroupRule(id)
	if err != nil {
		return nil, err
	}
	nextEnabled := !rule.Enabled
	if nextEnabled {
		if conflict, err := s.findGroupRuleConflict(rule.ID, rule.IngressGroupID, rule.IngressPort, rule.Protocol); err != nil {
			return nil, err
		} else if conflict != nil {
			return nil, &DeviceGroupRulePortConflictError{Rule: *conflict}
		}
	}
	if err := s.db.Model(rule).Update("enabled", nextEnabled).Error; err != nil {
		return nil, err
	}
	return s.GetDeviceGroupRule(id)
}

func (s *GroupService) GetDeviceGroupRule(id string) (*paneldb.DeviceGroupRule, error) {
	var rule paneldb.DeviceGroupRule
	if err := s.db.Take(&rule, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGroupRuleNotFound
		}
		return nil, err
	}
	return &rule, nil
}

func (s *GroupService) BuildDeviceGroupRuleViews(rules []paneldb.DeviceGroupRule, stats map[string]RuleRealtimeStat, totals map[string]int64, todayTotals map[string]int64) ([]DeviceGroupRuleView, error) {
	views := make([]DeviceGroupRuleView, 0, len(rules))
	for _, rule := range rules {
		view, err := s.buildDeviceGroupRuleView(rule, stats[rule.ID], totals[rule.ID], todayTotals[rule.ID])
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *GroupService) buildDeviceGroupRuleView(rule paneldb.DeviceGroupRule, stat RuleRealtimeStat, total, todayTotal int64) (DeviceGroupRuleView, error) {
	var ingressGroup, egressGroup paneldb.MachineGroup
	_ = s.db.Take(&ingressGroup, "id = ?", rule.IngressGroupID).Error
	_ = s.db.Take(&egressGroup, "id = ?", rule.EgressGroupID).Error

	var ingressMachines []paneldb.Machine
	if err := s.db.Where("group_id = ? AND role = ?", rule.IngressGroupID, "ingress").Find(&ingressMachines).Error; err != nil {
		return DeviceGroupRuleView{}, err
	}
	onlineEgress, err := s.onlineEgressMachines(rule.EgressGroupID)
	if err != nil {
		return DeviceGroupRuleView{}, err
	}

	var conflicts int64
	if len(ingressMachines) > 0 {
		for _, machine := range ingressMachines {
			conflict, err := (&RuleService{db: s.db, tunnelPort: s.tunnelPort}).findIngressPortConflict("", machine.ID, rule.IngressPort, rule.Protocol)
			if err != nil {
				return DeviceGroupRuleView{}, err
			}
			if conflict != nil {
				conflicts++
			}
		}
	}
	effective := int64(len(ingressMachines)) - conflicts
	limitExceeded := rule.TrafficLimitBytes > 0 && total >= rule.TrafficLimitBytes
	if len(onlineEgress) == 0 || limitExceeded || !rule.Enabled {
		effective = 0
	}

	return DeviceGroupRuleView{
		DeviceGroupRule:     rule,
		IngressGroupName:    ingressGroup.Name,
		EgressGroupName:     egressGroup.Name,
		IngressMachineCount: int64(len(ingressMachines)),
		EffectiveMachines:   effective,
		ConflictMachines:    conflicts,
		OnlineEgressCount:   int64(len(onlineEgress)),
		RealtimeStat:        stat,
		TotalBytes:          total,
		TodayBytes:          todayTotal,
		LimitExceeded:       limitExceeded,
	}, nil
}

func (s *GroupService) buildDeviceGroupRule(id string, input DeviceGroupRuleInput) (*paneldb.DeviceGroupRule, error) {
	protocol := strings.ToLower(strings.TrimSpace(input.Protocol))
	if protocol != "tcp" && protocol != "udp" && protocol != "both" {
		return nil, ErrInvalidProtocol
	}
	if input.IngressPort <= 0 || input.IngressPort > 65535 || input.TargetPort <= 0 || input.TargetPort > 65535 {
		return nil, ErrInvalidPort
	}
	if input.TrafficLimitBytes < 0 {
		return nil, ErrInvalidTraffic
	}
	if strings.TrimSpace(input.TargetAddr) == "" {
		return nil, ErrInvalidTarget
	}
	if err := s.ensureGroupRole(input.IngressGroupID, "ingress"); err != nil {
		return nil, err
	}
	if err := s.ensureGroupRole(input.EgressGroupID, "egress"); err != nil {
		return nil, err
	}
	return &paneldb.DeviceGroupRule{
		ID:                id,
		Name:              strings.TrimSpace(input.Name),
		IngressGroupID:    input.IngressGroupID,
		EgressGroupID:     input.EgressGroupID,
		IngressPort:       input.IngressPort,
		TargetAddr:        strings.TrimSpace(input.TargetAddr),
		TargetPort:        input.TargetPort,
		Protocol:          protocol,
		TrafficLimitBytes: input.TrafficLimitBytes,
		Enabled:           input.Enabled,
		Remark:            strings.TrimSpace(input.Remark),
	}, nil
}

func (s *GroupService) ensureGroupRole(id, role string) error {
	var group paneldb.MachineGroup
	if err := s.db.Take(&group, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGroupNotFound
		}
		return err
	}
	if group.Role != role {
		return ErrInvalidMachine
	}
	return nil
}

func (s *GroupService) findGroupRuleConflict(ruleID, ingressGroupID string, ingressPort int, protocol string) (*paneldb.DeviceGroupRule, error) {
	var rules []paneldb.DeviceGroupRule
	query := s.db.Where("enabled = ? AND ingress_group_id = ? AND ingress_port = ?", true, ingressGroupID, ingressPort)
	if strings.TrimSpace(ruleID) != "" {
		query = query.Where("id <> ?", ruleID)
	}
	if err := query.Find(&rules).Error; err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if protocolsOverlap(protocol, rule.Protocol) {
			conflict := rule
			return &conflict, nil
		}
	}
	return nil, nil
}

func (s *GroupService) onlineEgressMachines(groupID string) ([]paneldb.Machine, error) {
	var machines []paneldb.Machine
	if err := s.db.
		Where("group_id = ? AND role = ? AND online = ? AND ip <> ?", groupID, "egress", true, "").
		Order("id ASC").
		Find(&machines).Error; err != nil {
		return nil, err
	}
	return machines, nil
}

func (s *GroupService) PickEgressForRule(rule paneldb.DeviceGroupRule, ingressMachineID string) (*paneldb.Machine, error) {
	machines, err := s.onlineEgressMachines(rule.EgressGroupID)
	if err != nil {
		return nil, err
	}
	if len(machines) == 0 {
		return nil, ErrMachineIPMissing
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(ingressMachineID + ":" + rule.ID))
	return &machines[int(h.Sum32())%len(machines)], nil
}

func buildMachineGroup(input MachineGroupInput) (*paneldb.MachineGroup, error) {
	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role != "ingress" && role != "egress" {
		return nil, ErrInvalidRole
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrGroupNameRequired
	}
	return &paneldb.MachineGroup{
		Name:   name,
		Role:   role,
		Remark: strings.TrimSpace(input.Remark),
	}, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func groupRuleAgentConfig(rule paneldb.DeviceGroupRule, egress paneldb.Machine, tunnelPort int) AgentRuleConfig {
	return AgentRuleConfig{
		RuleID:            rule.ID,
		ListenAddr:        JoinHostPort("0.0.0.0", rule.IngressPort),
		Protocol:          rule.Protocol,
		EgressAddr:        JoinHostPort(egress.IP, tunnelPort),
		TargetAddr:        JoinHostPort(rule.TargetAddr, rule.TargetPort),
		TrafficLimitBytes: rule.TrafficLimitBytes,
	}
}
