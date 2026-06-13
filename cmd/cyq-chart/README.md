# CYQ 筹码分布图生成工具

基于 CYQ 三角形分布 + 换手率衰减模型，计算指定股票的筹码分布，生成 ECharts 交互式 HTML 图表。

## 快速开始

```bash
# 从项目根目录运行
go run ./cmd/cyq-chart/
```

修改 `main.go` 顶部的 `stockCode` 变量后重新运行即可切换股票。

## 配置变量

直接编辑 `main.go` 顶部的变量：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `stockCode` | 股票代码（6位数字） | `"600519"` |
| `tradeDate` | 截止日期 YYYYMMDD，0=最新交易日 | `0` |
| `outputPath` | 输出 HTML 路径，空=本目录下 `cyq-<code>.html` | `""` |
| `klineLoadLimit` | K线加载条数，0=全部 | `0` |
| `configPath` | 配置文件路径 | `"config.yaml"` |

## 输出内容

生成的 HTML 包含三部分：

### 1. 统计卡片

| 指标 | 说明 |
|------|------|
| 获利比例 | 当前价以下筹码占总筹码的比例 (0~100%) |
| 平均成本 | 50% 分位数价格 (元) |
| 90% 成本 | 90% 筹码分布的价格区间 (下限~上限 元) |
| 90% 集中度 | (上限-下限)/(上限+下限)，越小越集中 |
| 70% 成本 | 70% 筹码分布的价格区间 (下限~上限 元) |
| 70% 集中度 | (上限-下限)/(上限+下限)，越小越集中 |

### 2. 筹码分布主图

- **面积图**：Y 轴为价格（150 档等间距刻度），X 轴为筹码密度百分比
- **红色** = 获利盘（价格 < 当前价），**蓝色** = 套牢盘（价格 > 当前价）
- 橙色虚线标记当前价
- 鼠标悬停可查看每个价位的具体筹码密度

### 3. 趋势图

- 近 120 个交易日的 **获利比例** 和 **90%/70% 集中度** 变化趋势
- 双 Y 轴：左轴为获利比例 (%)，右轴为集中度

## 算法原理

### 三角形分布 + 换手率衰减模型

1. **构建价格刻度**：取全局最高/最低价，等分为 150 档 Y 轴刻度
2. **逐根 K 线迭代**：
   - 旧筹码按换手率衰减：`x[i] *= (1 - turnoverRate)`
   - 新筹码按三角形分布分配（均价为峰顶）：一字板时为矩形
3. **计算指标**：
   - 获利比例 = 当前价以下筹码 / 总筹码
   - 平均成本 = 50% 分位数价格
   - 90%/70% 集中度 = 取对应百分比筹码的价格区间，计算 `(上限-下限)/(上限+下限)`

### 集中度解读

- 集中度 **越小** → 筹码越集中 → 说明主力持仓成本区域窄
- 集中度 **越大** → 筹码越分散 → 说明持仓成本分布广

## 文件结构

```
cmd/cyq-chart/
├── main.go      # 入口，配置变量 + 数据加载 + HTML 生成
└── README.md    # 本文档
```

核心计算逻辑位于 `internal/indicator/technical/cyq.go`，本工具调用 `technical.BuildCYQ()` 获取数据后渲染 HTML。

## 数据流

```
DB 日K线 → technical.BuildCYQ() → CYQResult → 预处理 JSON → fmt.Sprintf 拼接 HTML → 写入文件
```

## 东方财富筹码分布实现细节

通过逆向 `quote.eastmoney.com/newstatic/libs/quotechart2022.js`（Webpack 打包产物），整理东财前端筹码分布的完整实现。

### 核心参数（与本项目对比）

| 参数 | 东方财富 | 本项目 | 一致性 |
|------|---------|--------|:------:|
| 精度因子 `fator` | `150` | `CyqAccuracyFactor = 150` | ✅ |
| 价格精度公式 | `(max-min)/(fator-1)` | `(maxPrice-minPrice)/(factor-1)` | ✅ |
| Y轴刻度构建 | `(n+u*s).toFixed(2)/1` | `math.Round(...*100)/100` | ✅ |
| 换手率处理 | `Math.min(1, d.hsl/100 \|\| 0)` | `/100`, cap at 1.0 | ✅ |
| 衰减公式 | `c[b] *= 1 - A` | `xData[k] *= (1-hsl)` | ✅ |
| G点X坐标(正常) | `2/(p-_)` 即 `2/(high-low)` | `2/(high-low)` | ✅ |
| G点Y坐标 | `floor((avg-min)/accuracy)` | `floor((avg-minPrice)/accuracy)` | ✅ |
| 一字板处理 | `c[y[1]] += y[0]*A/2` | `xData[gIdx] += gPoint[0]*hsl/2` | ✅ |
| 上半三角 | `(price-low)/(avg-low) * gPoint*hsl` | 相同 | ✅ |
| 下半三角 | `(high-price)/(high-avg) * gPoint*hsl` | 相同 | ✅ |
| 获利比例 | 遍历刻度累加 `price以下/总筹码` | `cyqGetBenefitPart` 相同逻辑 | ✅ |
| 90%/70% 分位数 | `(1±p)/2` 取 5%~95% / 15%~85% | `cyqComputePercentChips` 相同 | ✅ |
| 集中度公式 | `(高-低)/(高+低)` | `(prHigh-prLow)/(prHigh+prLow)` | ✅ |
| 精度保留 | `toPrecision(12)/1` | `round12() = Round(f*1e12)/1e12` | ✅ |
| K线请求条数 | `lmt=210`（kline_count=90, data_count=2×90+30） | DB 全量加载 | ⚠️ 差异 |
| 复权方式 | 前复权 `fqt=1` | 取决于 DB 数据 | ⚠️ 需确认 |

### K线请求条数计算

东财前端 K 线请求参数 `lmt` 由 `data_count` 决定：

```
data_count = 2 × kline_count + 30
```

- `kline_count`：屏幕可见K线数，concept.js 初始值 `90`
- 初始 `data_count = 2 × 90 + 30 = 210`，即 API 请求 `lmt=210`

**缩放超出已有数据范围时的参数变化**：

```js
debounceReload = debounce(function(e) {
    var n = t.common_data.data_count;          // 当前 data_count
    var o = 2 * t.common_data.kline_count + 30 - t.show_offset;  // 需要的 data_count
    if (o > n) {
        t.common_data.data_count = o;  // 更新为更大的值
        t.draw();                      // 重新请求数据 + 重算CYQ
    } else if (e) {
        t.reDraw();                    // 不重新请求，只重绘
    }
}, 100);
```

当用户缩小K线（`zoomOut`）使得需要显示的K线数增多，导致 `2 × new_kline_count + 30 - show_offset > 当前 data_count` 时：
- `data_count` 更新为 `2 × new_kline_count + 30 - show_offset`
- 重新调用 `draw()` → 请求更多K线数据 → 重算CYQ

例如：缩小后 `kline_count` 从 90 → 150，`show_offset=0`：
- 需要 `data_count = 2×150+30 = 330`
- API 请求 `lmt=330`（从210增加到330）

### 重绘触发条件

| # | 触发场景 | 入口函数 | 重新请求数据 | 重算CYQ |
|---|---------|---------|:----------:|:-------:|
| 1 | 首次加载/页面打开 | `draw()` | ✅ | ✅ |
| 2 | 定时刷新（盘中每60秒） | `setInterval(getDataDraw, 60000)` | ✅ | ✅ |
| 3 | 切换周期（日K/周K/月K） | `changeType()` → `draw()` | ✅ | ✅ |
| 4 | 切换复权（前复权/后复权/不复权） | `changeFQ()` → `draw()` | ✅ | ✅ |
| 5 | 缩放K线超出已有数据范围 | `zoomIn()/zoomOut()` → `debounceReload()` → `draw()` | ✅ | ✅ |
| 6 | 缩放K线未超出范围 | `debounceReload()` → `reDraw()` | ❌ | ✅ |
| 7 | 鼠标移动十字线 | `change_data_index` 事件 → `calc(n)` | ❌ | ✅ 只算指定日期 |
| 8 | 切换技术指标 | `changeIndicator()` → `reDraw()` | ❌ | ✅ |

### 鼠标移动十字线的重算逻辑

```js
// drawCYQ 函数结尾注册事件：
e.event.delAll("change_data_index");
e.event.add("change_data_index", function(t) { a(e, n, t) });

// a() 函数内部：
function a(e, t, n) {
    null == n && (n = e.data.full_klines.length - 1);  // 默认最新日
    // new CYQCalculator(full_klines).calc(n)
    // 只算第 n 根K线那一天的筹码分布，不重新请求K线
}
```

用户拖动十字线到历史某天时，`calc(n)` 的 `n` 变化，从已缓存的 `full_klines` 中实时重算那一天的CYQ，无需网络请求。

### CYQ 绘制条件

```js
// draw.ts 中的条件
t.cyq && h && (0, c.drawCYQ)(e)
// t.cyq = options.cyq（配置开关，concept.js 中 cyq: true）
// h = data.klines.length > 0（有K线数据）

// 仅在日K模式下启用（type == "day"）
```

### 东财 CYQCalculator 核心源码（反编译）

```typescript
// src/modules/tools/indicator/cyq.ts
class CYQCalculator {
    klines; factor; minPrice; accuracy; range; x; y;

    constructor(klines, t) {
        // t = factor，默认 150
        this.klines = klines;
        this.factor = t || 150;
        // 找全局价格极值
        var max = -Infinity, min = Infinity;
        for (const k of klines) {
            max = Math.max(max, k.high);
            min = Math.min(min, k.low);
        }
        this.minPrice = min;
        this.accuracy = (max - min) / (this.factor - 1);
        this.range = 2 * klines.length;  // range = 2 × K线根数
        this.x = new Array(this.factor).fill(0);  // 筹码密度数组
        this.y = [];                        // Y轴价格刻度
        for (var i = 0; i < this.factor; i++) {
            this.y.push((min + this.accuracy * i).toFixed(2) / 1);
        }
    }

    calc(n) {
        // n = 目标K线索引，默认最后一根
        var c = this.x.slice();  // 复制筹码数组
        var f = this.y;
        for (var i = 0; i <= n; i++) {
            var d = this.klines[i];
            var A = Math.min(1, d.hsl / 100 || 0);  // 换手率，cap at 1

            // 1. 旧筹码衰减
            for (var b = 0; b < this.factor; b++) {
                c[b] *= (1 - A);
            }

            // 2. 计算三角形G点坐标
            var high = d.high, low = d.low, avg = (high + low) / 2;
            var gPoint;  // [x坐标(密度), y坐标(价格刻度索引)]
            if (high === low) {
                // 一字板：矩形分布
                gPoint = [1, Math.floor((avg - this.minPrice) / this.accuracy)];
                c[gPoint[1]] += gPoint[0] * A / 2;
            } else {
                // 正常：三角形分布
                gPoint = [
                    2 / (high - low),                                          // G点X
                    Math.floor((avg - this.minPrice) / this.accuracy)           // G点Y
                ];
                // 上半三角 (low ~ avg)
                for (var j = Math.floor((low - this.minPrice) / this.accuracy);
                     j <= gPoint[1]; j++) {
                    var price = this.minPrice + this.accuracy * j;
                    c[j] += (price - low) / (avg - low) * gPoint[0] * A;
                }
                // 下半三角 (avg ~ high)
                for (var j = gPoint[1] + 1;
                     j <= Math.floor((high - this.minPrice) / this.accuracy); j++) {
                    var price = this.minPrice + this.accuracy * j;
                    c[j] += (high - price) / (high - avg) * gPoint[0] * A;
                }
            }
        }

        // 3. 计算获利比例
        var total = c.reduce((a, b) => a + b, 0);
        var close = this.klines[n].close;
        var profitSum = 0;
        for (var i = 0; i < this.factor; i++) {
            if (f[i] < close) profitSum += c[i];
        }
        var benefitPart = profitSum / total;

        // 4. 计算平均成本（50%分位数）
        var avgCost = getCostByChip(c, f, total, 0.5);

        // 5. 计算90%/70%集中度
        var percentChips = {
            90: computePercentChips(c, f, total, 0.9),
            70: computePercentChips(c, f, total, 0.7)
        };

        return { x: c, y: f, benefitPart, avgCost, percentChips };
    }
}
```

### 东财 drawCYQ Canvas 绘制逻辑

```typescript
// src/modules/kline/cyq.ts
function drawCYQ(e) {
    var n = document.createElement("div");
    n.className = "quotechart2022_c_cyq";
    // ... 设置样式 ...
    e.container.appendChild(n);

    // 默认计算最后一根K线
    calcAndDraw(e, n, e.data.full_klines.length - 1);

    // 注册十字线移动事件
    e.event.delAll("change_data_index");
    e.event.add("change_data_index", function(t) {
        calcAndDraw(e, n, t);  // t = 鼠标指向的K线索引
    });
}

function calcAndDraw(e, t, n) {
    // 1. 创建/替换 Canvas
    // 2. new CYQCalculator(full_klines).calc(n)
    // 3. 获利盘（价格 < close）：红色渐变 #F0927D → #FCE6DF
    // 4. 套牢盘（价格 >= close）：蓝色渐变 #88B4FB → #C4E2FF
    // 5. 平均成本虚线：橙色 #F97400，虚线 [6,2]
    // 6. 右侧信息面板：日期/获利比例/获利盘占比条/平均成本/90%成本/集中度/70%成本/集中度
}
```

## 注意事项

- 需要数据库中有对应股票的日 K 线数据（至少 60 条）
- 需要 `config.yaml` 配置正确的数据库连接
- HTML 依赖在线 ECharts CDN，首次打开需联网
