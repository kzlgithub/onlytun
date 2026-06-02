package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	paneldb "github.com/onlytun/panel/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AdminPasswordHashSettingKey = "admin_password_sha256"
	DeviceGroupModeSettingKey   = "device_group_mode_enabled"
)

type SettingsService struct {
	db *gorm.DB
}

func NewSettingsService(gdb *gorm.DB) *SettingsService {
	return &SettingsService{db: gdb}
}

func HashAdminPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func (s *SettingsService) AdminPasswordHash(defaultPassword string) (string, error) {
	var setting paneldb.SystemSetting
	if err := s.db.Take(&setting, "key = ?", AdminPasswordHashSettingKey).Error; err == nil {
		value := strings.TrimSpace(setting.Value)
		if value != "" {
			return value, nil
		}
	} else if err != gorm.ErrRecordNotFound {
		return "", err
	}
	return HashAdminPassword(defaultPassword), nil
}

func (s *SettingsService) SetAdminPasswordHash(hash string) error {
	record := paneldb.SystemSetting{
		Key:   AdminPasswordHashSettingKey,
		Value: strings.TrimSpace(hash),
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"value": record.Value}),
	}).Create(&record).Error
}

func (s *SettingsService) DeviceGroupModeEnabled() (bool, error) {
	var setting paneldb.SystemSetting
	if err := s.db.Take(&setting, "key = ?", DeviceGroupModeSettingKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(setting.Value), "true"), nil
}

func (s *SettingsService) SetDeviceGroupModeEnabled(enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	record := paneldb.SystemSetting{
		Key:   DeviceGroupModeSettingKey,
		Value: value,
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"value": record.Value}),
	}).Create(&record).Error
}
