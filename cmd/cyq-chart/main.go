// 筹码分布图 HTML 生成工具 — 直接编辑下方变量，运行即可生成。
//
// 用法:
//
//	go run ./cmd/cyq-chart/
//
// 修改 stockCode 后重新运行。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/backtest/indicator/technical"
	"stock-ai/utils"
)

// ============================================================================
//  配置 — 直接修改下面的变量
// ============================================================================

// stockCode 目标股票代码（6位数字，如 600519 = 茅台）
var stockCode = "000895"

// outputPath 输出 HTML 文件路径，空 = cmd/cyq-chart/cyq-<code>.html
var outputPath = ""

// klineCount 展示K线条数（东财默认90），实际加载条数 = 2×klineCount+30
var klineCount = 90

// configPath 配置文件路径
var configPath = "config.yaml"

// ============================================================================
//  main
// ============================================================================

func main() {
	// 加载配置 + 初始化数据库
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if err := db.Init(&cfg.Database); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer db.Close()

	// 加载 K 线（东财公式: data_count = 2×klineCount+30）
	limit := 2*klineCount + 30
	klines, err := db.FindDailyKlines(stockCode, utils.TodayTradeDate(), limit)
	if err != nil {
		log.Fatalf("加载日K线失败: %v", err)
	}
	if len(klines) < technical.CyqMinKlines {
		log.Fatalf("K线数据不足: 需要≥%d条，实际%d条", technical.CyqMinKlines, len(klines))
	}

	// 截止日期取最新一条K线的交易日
	displayDate := klines[0].TradeDate

	// 股票名称
	stockName := stockCode
	if s, err := db.FindStockByCode(stockCode); err == nil && s.Code != "" {
		stockName = s.Name
	}

	log.Printf("股票: %s (%s), K线条数: %d, 截止日期: %d", stockCode, stockName, len(klines), displayDate)

	// 计算 CYQ
	result := technical.BuildCYQ(klines, 150)
	latestIdx := len(result.ClosePrice) - 1

	log.Printf("获利比例: %.2f%%, 平均成本: %.2f", result.ProfitRatio[latestIdx]*100, result.AvgCost[latestIdx])
	log.Printf("90%%成本: %.2f ~ %.2f, 集中度: %.4f", result.Cost90[latestIdx][0], result.Cost90[latestIdx][1], result.Conc90[latestIdx])
	log.Printf("70%%成本: %.2f ~ %.2f, 集中度: %.4f", result.Cost70[latestIdx][0], result.Cost70[latestIdx][1], result.Conc70[latestIdx])

	// 生成 HTML
	outPath := outputPath
	if outPath == "" {
		// 默认输出到本文件所在目录
		_, thisFile, _, _ := runtime.Caller(0)
		outPath = filepath.Join(filepath.Dir(thisFile), fmt.Sprintf("cyq-%s.html", stockCode))
	}

	// 预处理趋势数据（最近klineCount个交易日）
	trendLen := klineCount
	if trendLen > len(result.ProfitRatio) {
		trendLen = len(result.ProfitRatio)
	}
	profitTrend := make([]string, trendLen)
	conc90Trend := make([]string, trendLen)
	conc70Trend := make([]string, trendLen)
	trendDates := make([]string, trendLen)
	startIdx := len(result.ProfitRatio) - trendLen
	for i := 0; i < trendLen; i++ {
		profitTrend[i] = fmt.Sprintf("%.2f", result.ProfitRatio[startIdx+i]*100)
		conc90Trend[i] = fmt.Sprintf("%.2f", result.Conc90[startIdx+i]*100)
		conc70Trend[i] = fmt.Sprintf("%.2f", result.Conc70[startIdx+i]*100)
	}
	// klines 从新到旧，取前 trendLen 条的日期和收盘价（反转后从旧到新）
	trendKN := trendLen
	if trendKN > len(klines) {
		trendKN = len(klines)
	}
	rawDates := make([]string, trendKN)
	rawClose := make([]string, trendKN)
	for i := 0; i < trendKN; i++ {
		d := klines[i].TradeDate // YYYYMMDD
		rawDates[i] = fmt.Sprintf("%d", d)
		rawClose[i] = fmt.Sprintf("%.2f", float64(klines[i].Close)/100.0)
	}
	// 反转：从旧到新，与 result 序列一致
	for i, j := 0, len(rawDates)-1; i < j; i, j = i+1, j-1 {
		rawDates[i], rawDates[j] = rawDates[j], rawDates[i]
		rawClose[i], rawClose[j] = rawClose[j], rawClose[i]
	}
	copy(trendDates, rawDates)

	// 计算最近klineCount条K线的价格范围（纵坐标展示范围）
	// klines 从新到旧，最近klineCount条 = 前klineCount个元素
	recentN := klineCount
	if recentN > len(klines) {
		recentN = len(klines)
	}
	var yMin, yMax float64
	for i := 0; i < recentN; i++ {
		k := klines[i]
		lo := float64(k.Low) / 100.0
		hi := float64(k.High) / 100.0
		if i == 0 || lo < yMin {
			yMin = lo
		}
		if i == 0 || hi > yMax {
			yMax = hi
		}
	}
	// 留一点余量
	yMin = math.Round((yMin-0.5)*100) / 100
	yMax = math.Round((yMax+0.5)*100) / 100
	log.Printf("纵坐标范围: %.2f ~ %.2f", yMin, yMax)

	if err := generateHTML(outPath, &chartData{
		StockCode:   stockCode,
		StockName:   stockName,
		TradeDate:   displayDate,
		ClosePrice:  fmt.Sprintf("%.2f", result.ClosePrice[latestIdx]),
		ProfitRatio: fmt.Sprintf("%.2f", result.ProfitRatio[latestIdx]*100),
		AvgCost:     fmt.Sprintf("%.2f", result.AvgCost[latestIdx]),
		Cost90Low:   fmt.Sprintf("%.2f", result.Cost90[latestIdx][0]),
		Cost90High:  fmt.Sprintf("%.2f", result.Cost90[latestIdx][1]),
		Conc90:      fmt.Sprintf("%.2f", result.Conc90[latestIdx]*100),
		Cost70Low:   fmt.Sprintf("%.2f", result.Cost70[latestIdx][0]),
		Cost70High:  fmt.Sprintf("%.2f", result.Cost70[latestIdx][1]),
		Conc70:      fmt.Sprintf("%.2f", result.Conc70[latestIdx]*100),
		XDataJSON:   mustJSON(result.XData),
		YDataJSON:   mustJSON(result.YData),
		YMin:        fmt.Sprintf("%.2f", yMin),
		YMax:        fmt.Sprintf("%.2f", yMax),
		ProfitTrend: "[" + strings.Join(profitTrend, ",") + "]",
		Conc90Trend: "[" + strings.Join(conc90Trend, ",") + "]",
		Conc70Trend: "[" + strings.Join(conc70Trend, ",") + "]",
		TrendDates:  mustJSON(trendDates),
		CloseTrend:  "[" + strings.Join(rawClose, ",") + "]",
	}); err != nil {
		log.Fatalf("生成HTML失败: %v", err)
	}

	log.Printf("✅ 已生成: %s", outPath)
}

// ============================================================================
//  HTML 生成
// ============================================================================

// chartData 模板渲染数据（所有字段均为可直接嵌入 HTML/JS 的字符串）
type chartData struct {
	StockCode   string
	StockName   string
	TradeDate   int
	ClosePrice  string
	ProfitRatio string
	AvgCost     string
	Cost90Low   string
	Cost90High  string
	Conc90      string
	Cost70Low   string
	Cost70High  string
	Conc70      string
	XDataJSON   string
	YDataJSON   string
	YMin        string
	YMax        string
	ProfitTrend string
	Conc90Trend string
	Conc70Trend string
	CloseTrend  string
	TrendDates  string
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("JSON序列化失败: %v", err)
	}
	return string(b)
}

func generateHTML(path string, data *chartData) error {
	html := buildHTML(data)
	return os.WriteFile(path, []byte(html), 0644)
}

// ============================================================================
//  HTML 生成 — 直接字符串拼接，避免模板引擎的转义问题
// ============================================================================

func buildHTML(d *chartData) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s (%s) 筹码分布图</title>
<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "PingFang SC", "Microsoft YaHei", sans-serif; background: #f5f5f5; color: #333; }
  .container { max-width: 900px; margin: 0 auto; padding: 20px; }
  h1 { text-align: center; font-size: 22px; color: #333; margin-bottom: 8px; }
  .subtitle { text-align: center; color: #888; font-size: 14px; margin-bottom: 20px; }
  .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 10px; margin-bottom: 20px; }
  .stat-card { background: #fff; border-radius: 6px; padding: 14px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
  .stat-card .label { font-size: 12px; color: #888; margin-bottom: 4px; }
  .stat-card .value { font-size: 18px; font-weight: 600; }
  .profit .value { color: #c0392b; }
  .loss .value { color: #4169e1; }
  .neutral .value { color: #333; }
  .chart-box { background: #fff; border-radius: 8px; padding: 12px; margin-bottom: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
  #mainChart { width: 540px; height: 660px; margin: 0 auto; }
  #trendChart { width: 100%%; height: 300px; }
  .legend { text-align: center; color: #999; font-size: 12px; margin-top: 10px; }
</style>
</head>
<body>
<div class="container">
  <h1>%s (%s) 筹码分布图</h1>
  <div class="subtitle">截止日期: %d | 当前价: %s 元 | 平均成本: %s 元</div>

  <div class="stats-grid">
    <div class="stat-card profit">
      <div class="label">获利比例</div>
      <div class="value">%s%%</div>
    </div>
    <div class="stat-card neutral">
      <div class="label">90%% 成本</div>
      <div class="value">%s ~ %s 元</div>
    </div>
    <div class="stat-card neutral">
      <div class="label">90%% 集中度</div>
      <div class="value">%s%%</div>
    </div>
    <div class="stat-card neutral">
      <div class="label">70%% 成本</div>
      <div class="value">%s ~ %s 元</div>
    </div>
    <div class="stat-card neutral">
      <div class="label">70%% 集中度</div>
      <div class="value">%s%%</div>
    </div>
  </div>

  <div class="chart-box">
    <div id="mainChart"></div>
  </div>

  <div class="chart-box">
    <div id="trendChart"></div>
  </div>

  <div class="legend">筹码分布基于 CYQ 三角形分布 + 换手率衰减模型 | 红色=获利盘 绿色=套牢盘 | 集中度越小筹码越集中</div>
</div>

<script>
(function() {
  var xData = %s;
  var yData = %s;
  var currentPrice = %s;
  var yMin = %s;
  var yMax = %s;

  // 按 yMin/yMax 过滤，只展示最近90条K线价格范围
  var fxData = [], fyData = [];
  for (var i = 0; i < yData.length; i++) {
    if (yData[i] >= yMin && yData[i] <= yMax) {
      fxData.push(xData[i]);
      fyData.push(yData[i]);
    }
  }
  xData = fxData;
  yData = fyData;

  // 归一化 xData 为百分比
  var maxChip = 0;
  for (var i = 0; i < xData.length; i++) {
    if (xData[i] > maxChip) maxChip = xData[i];
  }
  var xPercent = [];
  for (var i = 0; i < xData.length; i++) {
    xPercent.push(maxChip > 0 ? (xData[i] / maxChip * 100) : 0);
  }

  // ===== 主图：筹码分布面积图 =====
  var mainChart = echarts.init(document.getElementById('mainChart'));

  // 找到当前价在 yData 中的索引
  var currentPriceIdx = 0;
  for (var i = 0; i < yData.length; i++) {
    if (yData[i] >= currentPrice) { currentPriceIdx = i; break; }
  }

  // 拆分获利/套牢数据：在 currentPriceIdx 处重叠一个点，避免间隙
  var profitData = [];
  var lossData = [];
  for (var i = 0; i < xPercent.length; i++) {
    if (i <= currentPriceIdx) {
      profitData.push(xPercent[i]);
    } else {
      profitData.push(null);
    }
    if (i >= currentPriceIdx) {
      lossData.push(xPercent[i]);
    } else {
      lossData.push(null);
    }
  }

  mainChart.setOption({
    backgroundColor: '#fff',
    title: { text: '筹码分布', left: 'center', top: 5, textStyle: { color: '#333', fontSize: 16 } },
    tooltip: {
      trigger: 'axis',
      formatter: function(params) {
        var idx = params[0].dataIndex;
        return '价格: ' + yData[idx].toFixed(2) + ' 元<br/>筹码密度: ' + xPercent[idx].toFixed(2) + '%%';
      }
    },
    grid: { left: 60, right: 70, top: 50, bottom: 30 },
    xAxis: {
      type: 'value',
      max: function(value) { return Math.ceil(value.max / 10) * 10; },
      axisLine: { lineStyle: { color: '#ccc' } },
      axisLabel: { color: '#666' },
      splitLine: { show: false }
    },
    yAxis: {
      type: 'category',
      data: yData.map(function(v) { return v.toFixed(2); }),
      axisLine: { lineStyle: { color: '#ccc' } },
      axisLabel: { color: '#666', fontSize: 11 },
      inverse: false,
      axisTick: { show: false }
    },
    series: [
      {
        name: '套牢盘',
        type: 'line',
        data: lossData,
        smooth: true,
        symbol: 'none',
        lineStyle: { width: 0 },
        areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(65, 105, 225, 0.85)' },
          { offset: 1, color: 'rgba(65, 105, 225, 0.35)' }
        ])}
      },
      {
        name: '获利盘',
        type: 'line',
        data: profitData,
        smooth: true,
        symbol: 'none',
        lineStyle: { width: 0 },
        areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(192, 57, 43, 0.85)' },
          { offset: 1, color: 'rgba(192, 57, 43, 0.35)' }
        ])}
      },
    ]
  });

  // ===== 趋势图：获利比例 & 集中度 =====
  var trendChart = echarts.init(document.getElementById('trendChart'));
  var profitTrend = %s;
  var conc90Trend = %s;
  var conc70Trend = %s;
  var trendDates = %s;
  var closeTrend = %s;

  // 集中度纵坐标取两者中的较大值（始终为90%%集中度）
  var concMax = 0;
  for (var i = 0; i < conc90Trend.length; i++) {
    if (conc90Trend[i] > concMax) concMax = conc90Trend[i];
    if (conc70Trend[i] > concMax) concMax = conc70Trend[i];
  }
  concMax = Math.ceil(concMax);

  trendChart.setOption({
    backgroundColor: 'transparent',
    title: { text: '获利比例 & 集中度 & 收盘价趋势', left: 'center', top: 5, textStyle: { color: '#333', fontSize: 16 } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['获利比例(%%)', '90%%集中度(%%)', '70%%集中度(%%)', '收盘价'], top: 30, textStyle: { color: '#666' } },
    xAxis: { type: 'category', data: trendDates, axisLine: { lineStyle: { color: '#ccc' } }, axisLabel: { color: '#666', fontSize: 10, interval: Math.ceil(trendDates.length / 8) } },
    yAxis: [
      { type: 'value', name: '获利比例(%%)', position: 'left', axisLine: { lineStyle: { color: '#c0392b' } }, axisLabel: { color: '#666' }, splitLine: { lineStyle: { color: '#eee' } } },
      { type: 'value', name: '集中度(%%)', position: 'right', max: concMax, axisLine: { lineStyle: { color: '#4169e1' } }, axisLabel: { color: '#666' }, splitLine: { show: false } },
      { type: 'value', show: false, min: 'dataMin', max: 'dataMax' }
    ],
    grid: { left: 60, right: 60, top: 70, bottom: 30 },
    series: [
      { name: '获利比例(%%)', type: 'line', data: profitTrend, yAxisIndex: 0, smooth: true, lineStyle: { color: '#c0392b', width: 2 }, itemStyle: { color: '#c0392b' }, symbol: 'none' },
      { name: '90%%集中度(%%)', type: 'line', data: conc90Trend, yAxisIndex: 1, smooth: true, lineStyle: { color: '#4169e1', width: 2 }, itemStyle: { color: '#4169e1' }, symbol: 'none' },
      { name: '70%%集中度(%%)', type: 'line', data: conc70Trend, yAxisIndex: 1, smooth: true, lineStyle: { color: '#e67e22', width: 2 }, itemStyle: { color: '#e67e22' }, symbol: 'none' },
      { name: '收盘价', type: 'line', data: closeTrend, yAxisIndex: 2, smooth: true, lineStyle: { color: '#2ca02c', width: 2 }, itemStyle: { color: '#2ca02c' }, symbol: 'none' }
    ]
  });

  // 自适应
  window.addEventListener('resize', function() {
    mainChart.resize();
    trendChart.resize();
  });
})();
</script>
</body>
</html>`,
		d.StockName, d.StockCode,
		d.StockName, d.StockCode,
		d.TradeDate, d.ClosePrice, d.AvgCost,
		d.ProfitRatio,
		d.Cost90Low, d.Cost90High,
		d.Conc90,
		d.Cost70Low, d.Cost70High,
		d.Conc70,
		d.XDataJSON,
		d.YDataJSON,
		d.ClosePrice,
		d.YMin,
		d.YMax,
		d.ProfitTrend,
		d.Conc90Trend,
		d.Conc70Trend,
		d.TrendDates,
		d.CloseTrend,
	)
}
