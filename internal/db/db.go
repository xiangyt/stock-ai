package db

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"stock-ai/internal/config"

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
		Logger: NewGormLogger(logLevel),
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
	slog.Info("数据库连接成功")
	return nil
}

// AutoMigrate 自动迁移数据库表结构（已禁用，所有建表由 sql/ 目录手动管理）
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	// 暂时全部关闭自动建表，统一用 SQL 文件管理
	return nil

	// return DB.AutoMigrate(
	// 	&model.User{},
	// 	&model.Stock{},
	// 	&model.DailyKline{},
	// 	&model.Strategy{},
	// 	&model.PushBot{},
	// 	// 持仓管理
	// 	&model.Position{},
	// 	&model.PositionTrade{},
	// 	// 策略订阅
	// 	&model.Subscription{},
	// 	&model.SubscriptionBot{},
	// 	&model.SubscriptionLog{},
	// 	// 盯盘助手
	// 	&model.MonitorConfig{},
	// 	&model.MonitorConfigBot{},
	// 	// 数据采集
	// 	&model.DataCollectTask{},
	// 	&model.DataCollectBot{},
	// 	// 法定节假日
	// 	&model.TradingHoliday{},
	// )
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	if DB == nil {
		slog.Error("数据库未初始化，请先调用 db.Init()")
		os.Exit(1)
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
