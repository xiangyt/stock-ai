package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/service"
)

// ========== 用法说明 ==========

const usage = `K线同步工具 — 多周期三模式

用法:
  sync-kline [子命令] [选项]

子命令:
  init   初始化模式: 同花顺全量拉取骨架数据（amount=0）
  daily  每日增量: 同花顺 GetToday 等接口获取当期数据
  fill   补全金额: 东财全量拉取补 amount=0 的记录

选项:
  -config string   配置文件路径 (默认 "config.yaml")
  -periods string  同步周期,逗号分隔 (默认 "daily,weekly,monthly,yearly")
                   支持值: daily, weekly, monthly, yearly
  -h               显示帮助信息

示例:
  # 初始化所有周期
  go run main.go init

  # 仅初始化日K和周K
  go run main.go init -periods daily,weekly

  # 每日增量同步（可配置定时任务每天运行）
  go run main.go daily

  # 补全金额（建议每周低频运行，东财不稳定）
  go run main.go fill -periods daily

无子命令时默认执行 daily 模式。
`

func main() {
	// 解析通用参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	periodsStr := flag.String("periods", "daily,weekly,monthly,yearly", "同步周期(逗号分隔)")
	showHelp := flag.Bool("h", false, "显示帮助")
	flag.Parse()

	if *showHelp || len(os.Args) < 2 && flag.NArg() == 0 {
		fmt.Print(usage)
		return
	}

	// 解析子命令
	modeStr := strings.ToLower(flag.Arg(0))
	if modeStr == "" {
		modeStr = "daily" // 默认 daily 模式
	}
	mode, ok := map[string]service.SyncMode{
		"init":  service.SyncModeInit,
		"daily": service.SyncModeDaily,
		"fill":  service.SyncModeFill,
	}[modeStr]
	if !ok {
		log.Fatalf("未知子命令: %s\n支持的命令: init, daily, fill\n", modeStr)
	}

	// 解析周期列表
	periods := parsePeriods(*periodsStr)
	if len(periods) == 0 {
		log.Fatalf("无效的周期参数: %s\n支持值: daily, weekly, monthly, yearly\n", *periodsStr)
	}
	periodLabels := make([]string, len(periods))
	for i, p := range periods {
		periodLabels[i] = db.KLineLabel(p)
	}
	log.Printf("=== K线同步 | 模式=%s | 周期=%s ===\n",
		strings.ToUpper(modeStr), strings.Join(periodLabels, ","))

	// 初始化
	if err := initAll(*configPath); err != nil {
		return
	}
	defer db.Close()
	ctx := context.Background()
	svc := service.NewSyncKLineService()

	// 执行对应模式
	var results []service.SyncBatchResult
	switch mode {
	case service.SyncModeInit:
		results = svc.InitAllStocks(ctx, periods)
	case service.SyncModeDaily:
		results = svc.SyncDailyForAll(ctx, periods)
	case service.SyncModeFill:
		results = svc.FillMissingAmount(ctx, periods)
	default:
		log.Fatal("未实现的模式")
	}

	// 输出汇总
	printSummary(results)
}

// ========== 初始化 ==========

func initAll(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
		return err
	}

	if err := db.Init(&cfg.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
		return err
	}

	// if err := db.AutoMigrate(); err != nil {
	// 	log.Fatalf("数据库迁移失败: %v", err)
	// }

	registerAdapters(cfg.DataSources)
	return nil
}

// registerAdapters 参考 server/main.go 注册所有采集器
func registerAdapters(dataSources []config.DataSourceItem) {
	registry := adapter.GetRegistry()

	for _, dsCfg := range dataSources {
		if !dsCfg.Enabled {
			log.Printf("跳过未启用的数据源: %s", dsCfg.Name)
			continue
		}

		var ds adapter.DataSource
		switch dsCfg.Provider {
		case "eastmoney":
			ds = eastmoney.New()
			initConfig := map[string]interface{}{"cookie": dsCfg.Cookie}
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
				log.Printf("初始化 %s 失败: %v", dsCfg.Name, err)
				continue
			}
		case "ths":
			ds = ths.New()
			if err := ds.Init(nil); err != nil {
				log.Printf("初始化 %s 失败: %v", dsCfg.Name, err)
				continue
			}
		default:
			log.Printf("未知的数据源类型: %s (provider=%s)", dsCfg.Name, dsCfg.Provider)
			continue
		}

		if err := registry.Register(ds); err != nil {
			log.Printf("注册数据源 %s 失败: %v", dsCfg.Name, err)
		} else {
			log.Printf("✅ 已注册: %s (%s)", ds.DisplayName(), dsCfg.Name)
		}
	}

	log.Printf("已注册: %v\n", registry.Names())
}

// ========== 辅助函数 ==========

// parsePeriods 将逗号分隔的字符串解析为周期列表
func parsePeriods(s string) []db.KLinePeriod {
	valid := map[string]db.KLinePeriod{
		"daily":   db.KLinePeriodDaily,
		"weekly":  db.KLinePeriodWeekly,
		"monthly": db.KLinePeriodMonthly,
		"yearly":  db.KLinePeriodYearly,
	}
	var result []db.KLinePeriod
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if v, ok := valid[p]; ok {
			result = append(result, v)
		}
	}
	return result
}

// printSummary 打印批量结果汇总
func printSummary(results []service.SyncBatchResult) {
	totalS, totalSk, totalF := 0, 0, 0
	for _, r := range results {
		totalS += r.Success
		totalSk += r.SkipNoDelta
		totalF += r.Fail
	}
	log.Println("==============================")
	log.Printf("全部完成! 成功=%d 跳过=%d 失败=%d", totalS, totalSk, totalF)
	log.Println("==============================")
}
