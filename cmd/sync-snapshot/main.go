package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/config"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/db"
)

// ========== 用法说明 ==========

const usage = `每日估值快照计算工具

用法:
  sync-snapshot [选项]

选项:
  -config string     配置文件路径 (默认 "config.yaml")
  -code string       股票代码 (空=所有股票)
  -h                显示帮助信息

示例:
  # 计算 600519 的所有日期快照（从同花顺获取实时K线）
  go run main.go -code 600519

  # 全量计算（所有股票 × 所有日期）
  go run main.go
`

func main() {
	// 解析参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	code := flag.String("code", "", "股票代码（空=所有）")
	showHelp := flag.Bool("h", false, "显示帮助")
	flag.Parse()

	if *showHelp {
		fmt.Print(usage)
		return
	}

	// 打印运行参数
	log.Printf("=== 快照计算 | code=%s ===\n", strings.TrimSpace(*code))
	if *code != "" {
		log.Printf("  股票代码: %s\n", *code)
	}

	// 初始化
	if err := initAll(*configPath); err != nil {
		return
	}
	defer db.Close()
	ctx := context.Background()

	// 创建服务
	svc := datacollect.GetSnapshotService()

	// 统一调用
	startTime := time.Now()
	result := svc.Calc(ctx, strings.TrimSpace(*code))

	// 输出汇总
	printResult(result)
	log.Printf("\n总耗时: %.1fs\n", time.Since(startTime).Seconds())
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

	// 初始化同花顺适配器并注册到数据源中心
	thsAdapter := ths.New()
	if err := thsAdapter.Init(nil); err != nil {
		log.Fatalf("初始化同花顺适配器失败: %v", err)
		return err
	}
	registry := adapter.GetRegistry()
	if err := registry.Register(thsAdapter); err != nil {
		log.Fatalf("注册同花顺数据源失败: %v", err)
		return err
	}
	log.Println("✅ 同花顺适配器已就绪")

	return nil
}

// ========== 输出 ==========

func printResult(result datacollect.SnapshotBatchResult) {
	if result.Mode == "all_stocks_all_dates" {
		fmt.Println("==============================")
		fmt.Printf("全部完成! 成功=%d 失败=%d 耗时=%.1fs\n", result.Success, result.Fail, result.CostSeconds)
		fmt.Println("==============================")
	} else {
		fmt.Printf("[单股票] 成功=%d 失败=%d 耗时=%.1fs\n",
			result.Success, result.Fail, result.CostSeconds)
	}
}
