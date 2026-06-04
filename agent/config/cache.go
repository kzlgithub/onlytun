package config

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const pskSize = 32

var errInvalidPSKLength = errors.New("config: psk must decode to 32 bytes")

// RuleConfig 单条转发规则配置。
type RuleConfig struct {
	RuleID            string `json:"rule_id"`
	ListenAddr        string `json:"listen_addr"`
	Protocol          string `json:"protocol"`
	EgressAddr        string `json:"egress_addr"`
	TargetAddr        string `json:"target_addr"`
	TrafficLimitBytes int64  `json:"traffic_limit_bytes"`
}

// Config 完整本地配置。
type Config struct {
	MachineID           string       `json:"machine_id"`
	Role                string       `json:"role"`
	PSK                 string       `json:"psk"`
	PanelURL            string       `json:"panel_url"`
	Token               string       `json:"token"`
	TunnelListenAddr    string       `json:"tunnel_listen_addr"`
	TunnelAdvertiseAddr string       `json:"tunnel_advertise_addr"`
	IsIX                bool         `json:"is_ix"`
	Rules               []RuleConfig `json:"rules"`
}

// LoadConfig 从文件加载配置，文件不存在返回 error。
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveConfig 原子写入配置文件。
func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		return errors.New("config: nil config")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".onlytun-cache-*.tmp")
	if err != nil {
		return err
	}

	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// GetPSKBytes 将 Config 中 hex 编码的 PSK 解码为 []byte。
func (c *Config) GetPSKBytes() ([]byte, error) {
	if c == nil {
		return nil, errors.New("config: nil config")
	}
	psk, err := hex.DecodeString(c.PSK)
	if err != nil {
		return nil, err
	}
	if len(psk) != pskSize {
		return nil, errInvalidPSKLength
	}
	return psk, nil
}
