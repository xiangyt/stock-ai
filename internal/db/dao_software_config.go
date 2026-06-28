package db

import (
	"errors"

	"stock-ai/internal/model"

	"gorm.io/gorm"
)

// GetSoftwareConfigsByUser 获取某用户的所有软件配置
func GetSoftwareConfigsByUser(userID uint) ([]model.UserSoftwareConfig, error) {
	var configs []model.UserSoftwareConfig
	err := GetDB().Where("user_id = ?", userID).Order("software_name ASC").Find(&configs).Error
	return configs, err
}

// GetSoftwareConfig 获取某用户指定软件的配置
func GetSoftwareConfig(userID uint, softwareName string) (*model.UserSoftwareConfig, error) {
	var cfg model.UserSoftwareConfig
	err := GetDB().Where("user_id = ? AND software_name = ?", userID, softwareName).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpsertSoftwareConfig 插入或更新用户软件配置
func UpsertSoftwareConfig(cfg *model.UserSoftwareConfig) error {
	return GetDB().Transaction(func(tx *gorm.DB) error {
		var existing model.UserSoftwareConfig
		err := tx.Where("user_id = ? AND software_name = ?", cfg.UserID, cfg.SoftwareName).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err == nil {
			// 更新
			return tx.Model(&existing).Updates(map[string]interface{}{
				"display_name": cfg.DisplayName,
				"cookie":       cfg.Cookie,
				"extra":        cfg.Extra,
				"enabled":      cfg.Enabled,
			}).Error
		}

		// 新建
		return tx.Create(cfg).Error
	})
}
