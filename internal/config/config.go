package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置
type Config struct {
	Server     ServerConfig      `mapstructure:"server"`
	Database   DatabaseConfig    `mapstructure:"database"`
	MCP        MCPConfig         `mapstructure:"mcp"`
	Log        LogConfig         `mapstructure:"log"`
	DataSources []DataSourceItem `mapstructure:"data_sources"` // 多数据源列表
}

// DataSourceItem 单个数据源配置
type DataSourceItem struct {
	Name     string            `mapstructure:"name"`               // 数据源名称（唯一标识）
	Enabled  bool              `mapstructure:"enabled"`            // 是否启用
	Provider string            `mapstructure:"provider"`           // 提供商类型: eastmoney/ths
	Cookie   string            `mapstructure:"cookie"`            // Cookie（东方财富等需要）
	Extra    map[string]string `mapstructure:"extra,omitempty"`   // 扩展参数（各数据源自定义）
}

// GetDataSourceByName 按名称获取数据源配置
func (c *Config) GetDataSourceByName(name string) (*DataSourceItem, bool) {
	for i := range c.DataSources {
		if c.DataSources[i].Name == name {
			return &c.DataSources[i], true
		}
	}
	return nil, false
}

// ServerConfig 服务配置
type ServerConfig struct {
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	Charset         string `mapstructure:"charset"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// MCPConfig MCP 配置
type MCPConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Name      string `mapstructure:"name"`
	Version   string `mapstructure:"version"`
	Transport string `mapstructure:"transport"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// overrideFromEnv 从环境变量覆盖配置
func overrideFromEnv(cfg *Config) {
	// 数据库密码
	if pwd := os.Getenv("DB_PASSWORD"); pwd != "" {
		cfg.Database.Password = pwd
	}

	// 按数据源名称匹配环境变量（如 EM_COOKIE, THS_COOKIE 等）
	for i := range cfg.DataSources {
		name := strings.ToUpper(cfg.DataSources[i].Name)
		if cookie := os.Getenv(name + "_COOKIE"); cookie != "" {
			cfg.DataSources[i].Cookie = cookie
		}
	}
}

var GlobalConfig *Config

// Load 加载配置文件
func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// 设置默认值
	setDefaults()

	// 环境变量覆盖 (前缀: AI_STOCK_)
	viper.SetEnvPrefix("AI_STOCK")
	viper.AutomaticEnv()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		// 配置文件不存在时，使用默认配置
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		fmt.Println("配置文件不存在，使用默认配置")
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 从环境变量覆盖敏感信息
	overrideFromEnv(&cfg)

	GlobalConfig = &cfg
	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults() {
	// Server 默认值
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)

	// Database 默认值
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.user", "root")
	viper.SetDefault("database.dbname", "ai_stock_picker")
	viper.SetDefault("database.charset", "utf8mb4")
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.max_open_conns", 100)
	viper.SetDefault("database.conn_max_lifetime", 3600)

	// MCP 默认值
	viper.SetDefault("mcp.enabled", true)
	viper.SetDefault("mcp.name", "stock-ai")
	viper.SetDefault("mcp.version", "1.0.0")
	viper.SetDefault("mcp.transport", "stdio")

	// Log 默认值
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output", "stdout")
	viper.SetDefault("log.max_size", 100)
	viper.SetDefault("log.max_backups", 10)
	viper.SetDefault("log.max_age", 30)
}

// Get 获取全局配置
func Get() *Config {
	if GlobalConfig == nil {
		// 尝试加载默认配置文件
		if _, err := Load("config.yaml"); err != nil {
			// 使用默认配置
			GlobalConfig = &Config{}
			setDefaults()
			viper.Unmarshal(GlobalConfig)
		}
	}
	return GlobalConfig
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.DBName,
		c.Charset,
	)
}
