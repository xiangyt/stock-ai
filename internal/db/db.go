package db

import (
	"fmt"
	"log"
	"time"

	"stock-ai/internal/config"
	"stock-ai/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.DatabaseConfig) error {
	dsn := cfg.GetDSN()

	var logLevel logger.LogLevel
	if config.Get().Server.Mode == "debug" {
		logLevel = logger.Info
	} else {
		logLevel = logger.Error
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层连接失败: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}

	DB = db
	log.Println("数据库连接成功")
	return nil
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	return DB.AutoMigrate(
		&model.Stock{},
		// &model.StockPrice{},
		// &model.HotTopic{},
		// &model.FilterCondition{},
		// &model.DataSourceConfig{},
		// &model.CollectTask{},
		// &model.CollectLog{},
	)
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	if DB == nil {
		log.Fatal("数据库未初始化，请先调用 db.Init()")
	}
	return DB
}

// Close 关闭数据库连接
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
