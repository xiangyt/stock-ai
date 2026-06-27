package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"stock-ai/internal/adapter"
	"stock-ai/internal/adapter/eastmoney"
	"stock-ai/internal/adapter/ths"
	"stock-ai/internal/api/handler"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/model"
	"stock-ai/internal/service"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// StockResult 仅包含 AI 需要的核心字段
type StockResult struct {
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func main() {
	// 1. 定义并解析命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 2. 将标准库 log 的输出重定向到标准错误输出 (stderr)
	// 这一步非常关键！确保启动日志和后续业务日志不会污染 MCP 的 JSON 通信
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 3. 启动 MCP 服务，传入解析后的配置文件路径
	log.Printf("正在启动 MCP 服务，使用配置文件: %s\n", *configPath)
	startMCPServer(*configPath)
}

// startMCPServer 启动 MCP 服务
func startMCPServer(configPath string) {
	// 1. 创建 MCP 服务器实例
	s := server.NewMCPServer("StockAI-Strategy-Server", "1.0.0")

	// 2. 定义选股工具
	strategyTool := mcp.NewTool("execute_stock_strategy",
		mcp.WithDescription("根据指定的策略ID执行选股，仅返回符合条件的股票代码、名称和价格"),
		mcp.WithNumber("strategy_id",
			mcp.Required(),
			mcp.Description("策略 ID (必填，需从数据库加载)"),
		),
		mcp.WithString("date",
			mcp.Description("选股日期，格式 YYYY-MM-DD (默认为最新交易日)"),
		),
		mcp.WithNumber("concurrency",
			mcp.Description("最大并发数 (默认 100)"),
		),
	)

	// 3. 添加工具的处理逻辑
	s.AddTool(strategyTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 提取参数
		args, _ := request.Params.Arguments.(map[string]any)
		strategyID := uint(args["strategy_id"].(float64))
		date, _ := args["date"].(string)

		concurrency := 100
		if c, ok := args["concurrency"].(float64); ok {
			concurrency = int(c)
		}

		// 执行选股核心逻辑
		results, err := executeStrategy(configPath, strategyID, date, concurrency)
		if err != nil {
			// 如果出错，返回错误信息给 AI
			return mcp.NewToolResultError(err.Error()), nil
		}

		// 将结果序列化为 JSON 返回（AI 极其擅长解析 JSON 数组）
		jsonData, _ := json.Marshal(results)
		return mcp.NewToolResultText(string(jsonData)), nil
	})

	// 4. 启动基于 stdio 的 MCP 服务器
	log.Println("StockAI MCP Server 已启动，等待 AI 调用...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP 服务器启动失败: %v\n", err)
	}
}

// executeStrategy 核心选股逻辑（精简版，去掉了所有 CLI 打印）
func executeStrategy(configPath string, strategyID uint, date string, maxConcurrency int) ([]StockResult, error) {
	var err error
	if err := initDB(configPath); err != nil {
		return nil, err
	}
	defer db.Close()

	if strategyID == 0 {
		return nil, err
	}

	strategy, err := db.GetStrategyByID(strategyID)
	if err != nil {
		return nil, err
	}

	configs, err := parseSignals(strategy.Conditions)
	if err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		return nil, err
	}

	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	registerAdapters()

	screenSvc := service.NewScreenService()
	indHandler := handler.NewIndicatorHandler(screenSvc)
	reg := indHandler.Registry()

	stocks, err := screenSvc.BuildAll(maxConcurrency, date)
	if err != nil {
		return nil, err
	}

	if len(stocks) == 0 {
		return []StockResult{}, nil
	}

	results := reg.Engine().Execute(stocks, configs, maxConcurrency)

	// 只提取需要的字段
	var passedList []StockResult
	for _, r := range results {
		if r.Result == indicator.ResultPassed {
			passedList = append(passedList, StockResult{
				Code:  r.Code,
				Name:  r.Name,
				Price: r.Price,
			})
		}
	}

	return passedList, nil
}

// ========== 以下保留你原有的基础辅助函数（initDB, registerAdapters, parseSignals） ==========
// 为了保持代码完整，这些函数保持不变，去掉了其中的 log.Printf 以保持 MCP 返回的纯净
// 如果 AI 需要看日志，建议在配置文件中开启文件日志，而不是打印到标准输出（stdout）干扰 MCP 协议

var globalConfig *config.Config

func initDB(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := db.Init(&cfg.Database); err != nil {
		return err
	}
	globalConfig = cfg
	return nil
}

func registerAdapters() {
	if globalConfig == nil {
		return
	}
	cfg := globalConfig
	registry := adapter.GetRegistry()

	for _, dsCfg := range cfg.DataSources {
		if !dsCfg.Enabled {
			continue
		}
		var ds adapter.DataSource
		switch dsCfg.Provider {
		case eastmoney.AdapterName:
			ds = eastmoney.New()
			initConfig := map[string]interface{}{"cookie": dsCfg.Cookie}
			for k, v := range dsCfg.Extra {
				initConfig[k] = v
			}
			if err := ds.Init(initConfig); err != nil {
				continue
			}
		case ths.AdapterName:
			ds = ths.New()
			if err := ds.Init(nil); err != nil {
				continue
			}
		default:
			continue
		}
		registry.Register(ds)
	}
}

func parseSignals(conditionsJSON string) ([]*indicator.SignalConfig, error) {
	if conditionsJSON == "" {
		return nil, nil
	}
	var signals []model.StrategySignal
	if err := json.Unmarshal([]byte(conditionsJSON), &signals); err != nil {
		return nil, err
	}
	configs := make([]*indicator.SignalConfig, 0, len(signals))
	for _, sig := range signals {
		configs = append(configs, &indicator.SignalConfig{
			SignalID: sig.SignalID,
			Operator: indicator.CompareOperator(sig.Operator),
			Params:   sig.Params,
		})
	}
	return configs, nil
}
