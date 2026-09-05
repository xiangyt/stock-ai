// 每日估值快照计算工具 — 直接编辑下方变量，运行即可。
//
// 用法:
//
//	go run ./cmd/sync-snapshot/
//
// 修改 stockCode 后重新运行。
// stockCode 为空时执行全量计算（所有股票 × 所有日期）。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/adapter/tencentstock"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/adapter/ths2"
	"stock-ai/internal/config"
	"stock-ai/internal/datacollect"
	"stock-ai/internal/db"
)

// ============================================================================
//  运行配置 — 直接修改下面的变量
// ============================================================================

// stockCode 目标股票代码（6位数字，如 600519 = 茅台）；空 = 全量计算所有股票
var stockCode = ""

// configPath 配置文件路径
var configPath = "config.yaml"

func main() {
	// 打印运行参数
	log.Printf("=== 快照计算 | code=%s ===\n", stockCode)
	if stockCode != "" {
		log.Printf("  股票代码: %s\n", stockCode)
	}

	// 初始化
	if err := initAll(configPath); err != nil {
		return
	}
	defer db.Close()
	ctx := context.Background()

	// 创建服务
	svc := datacollect.GetSnapshotService()

	// 统一调用
	startTime := time.Now()
	result := svc.Calc(ctx, stockCode)

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

	registry := adapter.GetRegistry()
	for _, dsCfg := range cfg.DataSources {
		if !dsCfg.Enabled {
			log.Println("跳过未启用的数据源", "name", dsCfg.Name)
			continue
		}

		var ds adapter.DataSource
		switch dsCfg.Provider {
		case eastmoney.AdapterName:
			ds = eastmoney.New()
			initConfig := map[string]interface{}{
				"cookie": dsCfg.Cookie,
			}
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
				fmt.Println("初始化数据源失败", "name", dsCfg.Name, "error", err)
				continue
			}
		case ths.AdapterName:
			ds = ths.New()
			if err := ds.Init(nil); err != nil {
				fmt.Println("初始化数据源失败", "name", dsCfg.Name, "error", err)
				continue
			}
		case tencentstock.AdapterName:
			ds = tencentstock.New()
			if err := ds.Init(nil); err != nil {
				fmt.Println("初始化数据源失败", "name", dsCfg.Name, "error", err)
				continue
			}
		case ths2.AdapterName:
			ds = ths2.New()
			initConfig := map[string]interface{}{
				"cookie": dsCfg.Cookie,
			}
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
				fmt.Println("初始化数据源失败", "name", dsCfg.Name, "error", err)
				continue
			}
		default:
			fmt.Println("未知的数据源类型", "name", dsCfg.Name, "provider", dsCfg.Provider)
			continue
		}

		if err := registry.Register(ds); err != nil {
			fmt.Println("注册数据源失败", "name", dsCfg.Name, "error", err)
		} else {
			fmt.Println("已注册数据源", "display_name", ds.DisplayName(), "name", dsCfg.Name)
		}
	}
	fmt.Println("已注册数据源", "names", registry.Names())

	return nil
}

// ========== 输出 ==========

func printResult(result datacollect.SnapshotBatchResult) {
	if result.Mode == "all_stocks_all_dates" {
		fmt.Println("==============================")
		fmt.Printf("全部完成! 股票:成功=%d 失败=%d | 快照:写入成功=%d 写入失败=%d | 耗时=%.1fs\n",
			result.SuccessStocks, result.FailStocks,
			result.SuccessSnapshots, result.FailSnapshots,
			result.CostSeconds)
		fmt.Println("==============================")
	} else {
		fmt.Printf("[单股票] 快照=%d 写入成功=%d 写入失败=%d 耗时=%.1fs\n",
			result.TotalSnapshots, result.SuccessSnapshots, result.FailSnapshots,
			result.CostSeconds)
	}
}
