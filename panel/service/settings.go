package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	paneldb "github.com/onlytun/panel/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const AdminPasswordHashSettingKey = "admin_password_sha256"

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
