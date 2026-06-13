package db

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const SharedPSKSettingKey = "shared_psk"

type Machine struct {
	ID                  string    `gorm:"primaryKey;size:36" json:"id"`
	Name                string    `gorm:"size:128;not null" json:"name"`
	Role                string    `gorm:"size:16;not null;index" json:"role"`
	GroupID             string    `gorm:"size:36;index" json:"group_id"`
	IP                  string    `gorm:"size:64" json:"ip"`
	IsIX                bool      `gorm:"not null;default:false" json:"is_ix"`
	TunnelAdvertiseAddr string    `gorm:"size:255" json:"tunnel_advertise_addr"`
	Token               string    `gorm:"size:128;not null;uniqueIndex" json:"token"`
	Online              bool      `gorm:"not null;default:false" json:"online"`
	OS                  string    `gorm:"size:64" json:"os"`
	AgentVersion        string    `gorm:"size:64" json:"agent_version"`
	CPUPercent          float64   `gorm:"not null;default:0" json:"cpu_percent"`
	MemPercent          float64   `gorm:"not null;default:0" json:"mem_percent"`
	DiskPercent         float64   `gorm:"not null;default:0" json:"disk_percent"`
	UptimeSeconds       uint64    `gorm:"not null;default:0" json:"uptime_seconds"`
	NetBytesUp          uint64    `gorm:"not null;default:0" json:"net_bytes_up"`
	NetBytesDown        uint64    `gorm:"not null;default:0" json:"net_bytes_down"`
	NetUpBps            uint64    `gorm:"not null;default:0" json:"net_up_bps"`
	NetDownBps          uint64    `gorm:"not null;default:0" json:"net_down_bps"`
	LastHeartbeat       time.Time `gorm:"index" json:"last_heartbeat"`
	OnlineSince         time.Time `json:"online_since"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type MachineGroup struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Role      string    `gorm:"size:16;not null;index" json:"role"`
	Remark    string    `gorm:"type:text" json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ForwardRule struct {
	ID                string    `gorm:"primaryKey;size:36" json:"id"`
	Name              string    `gorm:"size:128;not null" json:"name"`
	IngressMachineID  string    `gorm:"size:36;not null;index" json:"ingress_machine_id"`
	IngressPort       int       `gorm:"not null" json:"ingress_port"`
	EgressMachineID   string    `gorm:"size:36;not null;index" json:"egress_machine_id"`
	TargetAddr        string    `gorm:"size:255;not null" json:"target_addr"`
	TargetPort        int       `gorm:"not null" json:"target_port"`
	Protocol          string    `gorm:"size:16;not null" json:"protocol"`
	Enabled           bool      `gorm:"not null;default:true;index" json:"enabled"`
	TrafficLimitBytes int64     `gorm:"not null;default:0" json:"traffic_limit_bytes"`
	Remark            string    `gorm:"type:text" json:"remark"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type DeviceGroupRule struct {
	ID                string    `gorm:"primaryKey;size:36" json:"id"`
	Name              string    `gorm:"size:128;not null" json:"name"`
	IngressGroupID    string    `gorm:"size:36;not null;index" json:"ingress_group_id"`
	EgressGroupID     string    `gorm:"size:36;not null;index" json:"egress_group_id"`
	IngressPort       int       `gorm:"not null" json:"ingress_port"`
	TargetAddr        string    `gorm:"size:255;not null" json:"target_addr"`
	TargetPort        int       `gorm:"not null" json:"target_port"`
	Protocol          string    `gorm:"size:16;not null" json:"protocol"`
	Enabled           bool      `gorm:"not null;default:true;index" json:"enabled"`
	TrafficLimitBytes int64     `gorm:"not null;default:0" json:"traffic_limit_bytes"`
	Remark            string    `gorm:"type:text" json:"remark"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type TrafficStat struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RuleID    string    `gorm:"size:36;not null;uniqueIndex:idx_rule_hour" json:"rule_id"`
	Hour      time.Time `gorm:"not null;uniqueIndex:idx_rule_hour;index" json:"hour"`
	BytesUp   int64     `gorm:"not null;default:0" json:"bytes_up"`
	BytesDown int64     `gorm:"not null;default:0" json:"bytes_down"`
	PeakConns int64     `gorm:"not null;default:0" json:"peak_conns"`
}

type InstallToken struct {
	Token     string    `gorm:"primaryKey;size:128" json:"token"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	Used      bool      `gorm:"not null;default:false" json:"used"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MachineUpdateTask struct {
	ID          string     `gorm:"primaryKey;size:36" json:"id"`
	MachineID   string     `gorm:"size:36;not null;index" json:"machine_id"`
	Kind        string     `gorm:"size:32;not null" json:"kind"`
	Status      string     `gorm:"size:32;not null;index" json:"status"`
	FromVersion string     `gorm:"size:64" json:"from_version"`
	Error       string     `gorm:"type:text" json:"error"`
	RequestedAt time.Time  `gorm:"index" json:"requested_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *Machine) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return nil
}

func (r *ForwardRule) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

func (g *MachineGroup) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.NewString()
	}
	return nil
}

func (r *DeviceGroupRule) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

func (t *MachineUpdateTask) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.Kind == "" {
		t.Kind = "agent"
	}
	if t.Status == "" {
		t.Status = "pending"
	}
	if t.RequestedAt.IsZero() {
		t.RequestedAt = time.Now()
	}
	return nil
}

func OpenDatabase(path string) (*gorm.DB, error) {
	if path == "" {
		return nil, errors.New("db: empty database path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := AutoMigrate(gdb); err != nil {
		return nil, err
	}
	return gdb, nil
}

func AutoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&Machine{},
		&MachineGroup{},
		&ForwardRule{},
		&DeviceGroupRule{},
		&TrafficStat{},
		&InstallToken{},
		&MachineUpdateTask{},
		&SystemSetting{},
	)
}

func GenerateHexSecret(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
