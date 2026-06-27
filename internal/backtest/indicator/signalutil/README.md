# signalutil — 信号模板工具集

定义信号构建、参数模板、操作符集合和通用评估函数。各指标子包通过调用这套 API 可以快速生成信号定义、批量构造信号列表并统一处理评估逻辑，避免重复代码。

---

## 一、参数工厂函数（templates.go）

创建 `indicator.ParamDef` 的快捷方法。

### ParamNumber

```go
func ParamNumber(key, label string, defaultVal float64, unit string) indicator.ParamDef
```

创建一个数值型参数（Type: `"number"`），Required。

### ParamRange

```go
func ParamRange(key, label string, minVal, maxVal, defaultVal float64, unit string) indicator.ParamDef
```

创建一个范围型参数（Type: `"range"`），带 min/max/Placeholder。

### ParamDays

```go
func ParamDays(defaultVal int) indicator.ParamDef
```

创建"回看天数"参数（Type: `"days"`），默认范围 5~120 天。用于交叉信号等需要配置周期的场景。

### ParamLookbackStart / ParamLookbackEnd

```go
func ParamLookbackStart(def float64, unit string) indicator.ParamDef
func ParamLookbackEnd(def float64, unit string) indicator.ParamDef
```

时间窗口参数。`start` 表示窗口起点（更早的 N 天前），`end` 表示窗口终点（更近的 N 天前）。用于信号的时间范围检查，例如"最近 3 天是否出现金叉"。

**语义约定**：`start >= end`，即 start 代表更早的时间，end 代表更近的时间。`N=0` 表示今天。

**用法示例**：

```go
// 信号参数定义
params := []indicator.ParamDef{
    signalutil.ParamLookbackStart(0, "天前"), // 默认仅今天(0天前)
    signalutil.ParamLookbackEnd(0, "天前"),
}

// Evaluate 中读取参数
start := int(config.GetFloat64(indicator.ParamKeyLookbackStart, 0))
end   := int(config.GetFloat64(indicator.ParamKeyLookbackEnd, 0))
idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(data))
```

---

## 二、NormalizeLookback — 时间窗口索引映射

```go
func NormalizeLookback(start, end, dataLen int) (idxStart, idxEnd int, err error)
```

将"N 天前"的窗口描述映射为 oldest-first 数组索引的 **半开区间** `[idxStart, idxEnd)`。

### 映射规则

| N（天前） | 含义 | oldest-first 索引 |
|----------|------|-------------------|
| 0 | 今天 | `dataLen - 1` |
| 1 | 昨天 | `dataLen - 2` |
| N | N 天前 | `dataLen - 1 - N` |

**半开区间**：`idxEnd` 是哨兵索引（不包含），实际检查范围是 `[idxStart, idxEnd-1]`。

### 行为

- **自动容错交换**：调用方无需保证 `start >= end`，函数内部自动 swap。
- **越界裁剪**：`idxStart < 0` 时裁剪为 0，`idxEnd > dataLen` 时裁剪为 dataLen。
- **支持单日检查**：`start == end` 时返回 1 个元素的窗口。
- **空窗口报错**：当裁剪后 `idxStart >= idxEnd`，返回错误。

### 用法示例

```go
// start=0, end=0, dataLen=100 → idxStart=99, idxEnd=100 → 仅检查 array[99]（今天）
// start=3, end=0, dataLen=100 → idxStart=96, idxEnd=100 → 检查 array[96..99]（最近4天）
// start=3, end=3, dataLen=100 → idxStart=96, idxEnd=97 → 仅检查 array[96]（3天前）
```

**遍历写法**：

```go
idxStart, idxEnd, err := signalutil.NormalizeLookback(start, end, len(signalData))
if err != nil {
    return indicator.ErrInvalidParam
}
for i := idxStart; i < idxEnd; i++ {
    if signalData[i] == TriggerValue {
        return indicator.Pass("触发 ✓")
    }
}
return indicator.Fail("在窗口内未触发")
```

---

## 三、操作符集合

预设的操作符选项列表，用于传递给信号的 `OperatorOptions`。

### NumberOps（数值型）

简单数值比较，默认无单位：

```go
var NumberOps = []indicator.OperatorOption{...}
// gt / gte / lt / lte / between（5个）
```

### NumberOpsWith / NumberOpsByUnit / NumberOpsByUnitMin

用于需要携带单位或参数最小值约束的场景：

```go
func NumberOpsWith(unit string, minVal float64) []indicator.OperatorOption   // 完整 6 个 op（含 NotBetween）
func NumberOpsByUnit(unit string) []indicator.OperatorOption                 // min=0
func NumberOpsByUnitMin(unit string, minVal float64) []indicator.OperatorOption
```

包含 6 个操作符：`lt / lte / gt / gte / between / not_between`。

```go
// 用法：财务指标带单位和最小值限制
numberOps := signalutil.NumberOpsByUnitMin(pe.UnitStr, 0)
```

### EnumOps（枚举型）

```go
func EnumOps(options []indicator.EnumOption) []indicator.OperatorOption
```

根据枚举选项生成 4 个操作符：`eq / neq / in / not_in`。

### BoolOps（布尔型）

```go
var BoolOps = []indicator.OperatorOption  // eq / neq（无需额外参数）
```

### CrossOps（序列交叉型）

```go
var CrossOps = []indicator.OperatorOption  // cross_above / cross_below
```

默认快慢线周期为 12/26（经典 MACD）。

---

## 四、信号模板工厂函数

### 4.1 数值型信号模板

三个模板对应最常见的数值信号需求：

```go
func NumberSignalGT(unit string) indicator.Signal       // 大于 (seq=01)
func NumberSignalLT(unit string) indicator.Signal       // 小于 (seq=02)
func NumberSignalBetween(unit string) indicator.Signal  // 区间内 (seq=03)
```

`NumberSignalGT` 同时支持 `gt` 和 `gte` 两种操作符。`NumberSignalBetween` 支持 `between` 和 `not_between`。

### 4.2 布尔型信号模板

```go
func BoolSignalIs(name, desc string) indicator.Signal           // 是 (seq=01, 操作符=eq)
func BoolSignalNot(name, desc string) indicator.Signal          // 不是 (seq=02, 操作符=neq)
func DefaultBoolSignals(isName, notName, desc string) []indicator.Signal  // 返回一对
```

### 4.3 枚举型信号模板

```go
func EnumSignalMatch(name, desc string, options []indicator.EnumOption) indicator.Signal
```

创建一个带枚举选项的单信号（seq=01），操作符为 `OpIn`。

### 4.4 序列型交叉信号模板

```go
func SeriesSignalCrossAbove(fastName, slowName string) indicator.Signal  // 上穿/金叉 (seq=01)
func SeriesSignalCrossBelow(fastName, slowName string) indicator.Signal  // 下穿/死叉 (seq=02)
```

信号名称自动拼接为 `"{fast}上{slow}"` / `"{fast}下{slow}"`。

---

## 五、批量信号构建 — RangeDef / EnumDef

用于"数据驱动"批量生成信号，避免手写重复代码。

### 5.1 RangeDef（数值型）

```go
type RangeDef struct {
    Seq          string                    // 2 位序号
    Desc         string                    // 显示名称
    Operator     indicator.CompareOperator // 操作符
    MinThreshold float64                   // 下限（GT/GTE 系列当阈值）
    MaxThreshold float64                   // 上限（LT/LTE 系列当阈值）
}
```

`Operator` 与阈值的对应关系：

| Operator | 使用哪个阈值 | 存入参数 |
|----------|-------------|---------|
| `OpGT` / `OpGTE` | `MinThreshold` | `ParamKeyThreshold` |
| `OpLT` / `OpLTE` | `MaxThreshold` | `ParamKeyThreshold` |
| `OpBetween` / `OpNotBetween` | `[MinThreshold, MaxThreshold]` | `ParamKeyMin` + `ParamKeyMax` |

批量构建：

```go
func BuildRangeSignals(defs []RangeDef, name string, ops []indicator.OperatorOption,
    wrapFn func(indicator.BaseSignal) indicator.Signal) []indicator.Signal
```

**完整示例**（PE 指标）：

```go
var peDefs = []signalutil.RangeDef{
    {Seq: "01", Desc: "小于0", Operator: indicator.OpLT, MinThreshold: 0, MaxThreshold: 0},
    {Seq: "02", Desc: "0~25", Operator: indicator.OpBetween, MinThreshold: 0, MaxThreshold: 25},
    {Seq: "03", Desc: "25~50", Operator: indicator.OpBetween, MinThreshold: 25, MaxThreshold: 50},
    {Seq: "04", Desc: "大于50", Operator: indicator.OpGT, MinThreshold: 50, MaxThreshold: 0},
}

numberOps := signalutil.NumberOpsByUnit("倍")
builtInSigs := signalutil.BuildRangeSignals(peDefs, "市盈率(TTM)", numberOps, func(bs indicator.BaseSignal) indicator.Signal {
    return &peTTMSignal{bs}
})
indicator.SetBuiltInSignals(builtInSigs)
```

### 5.2 EnumDef（枚举型）

```go
type EnumDef struct {
    Seq      string
    Desc     string
    Operator indicator.CompareOperator
    Options  []indicator.EnumOption
    Value    string   // 单值匹配（OpEQ/OpNEQ）
    Values   []string // 多值匹配（OpIn/OpNotIn）
}
```

批量构建：

```go
func BuildEnumSignals(defs []EnumDef, name string, ops []indicator.OperatorOption,
    wrapFn func(indicator.BaseSignal) indicator.Signal) []indicator.Signal
```

**完整示例**（上市板块）：

```go
var boardDefs = []signalutil.EnumDef{
    {Seq: "01", Desc: "主板", Operator: indicator.OpEQ, Value: "main"},
    {Seq: "02", Desc: "创业板", Operator: indicator.OpEQ, Value: "chinext"},
    {Seq: "03", Desc: "科创板", Operator: indicator.OpEQ, Value: "star"},
    {Seq: "04", Desc: "北交所", Operator: indicator.OpEQ, Value: "bse"},
}

ops := signalutil.EnumOps(ListingBoardOptions)
builtInSigs := signalutil.BuildEnumSignals(boardDefs, "上市板块", ops, func(bs indicator.BaseSignal) indicator.Signal {
    return &ListingBoardSignal{bs}
})
indicator.SetBuiltInSignals(builtInSigs)
```

---

## 六、通用评估函数

在信号的 `Evaluate` 方法中直接调用的评估逻辑。

### EvalNumberOp

```go
func EvalNumberOp(value float64, label, valueFmt, threshFmt string,
    signalID string, config *indicator.SignalConfig) *indicator.EvaluatedStock
```

对 `value` 按 `config.Operator` 进行比较，返回带格式化消息的评估结果。支持的操作符：`OpLT / OpLTE / OpGT / OpGTE / OpBetween / OpNotBetween`。

**参数**：
- `valueFmt`：数值的格式化字符串，如 `"%.2f"`
- `threshFmt`：阈值的格式化字符串，如 `"%.0f"`

```go
func (s *peTTMSignal) Evaluate(snap *model.StockDailySnapshot, config *indicator.SignalConfig) *indicator.EvaluatedStock {
    value := snap.PETTM
    return signalutil.EvalNumberOp(value, "市盈率(TTM)", "%.2f", "%.0f", config.SignalID, config)
}
```

### EvalEnumOp

```go
func EvalEnumOp(value, label, signalID string, config *indicator.SignalConfig) *indicator.EvaluatedStock
```

对 `value`（字符串）按 `config.Operator` 进行枚举匹配。支持：`OpEQ / OpNEQ / OpIn / OpNotIn`。

```go
func (s *ListingBoardSignal) Evaluate(name, board string, config *indicator.SignalConfig) *indicator.EvaluatedStock {
    return signalutil.EvalEnumOp(board, "板块", config.SignalID, config)
}
```

---

## 七、通用工具函数（utils.go）

```go
func ToStringSlice(val any) []string
```

将 `val` 安全转换为 `[]string`。支持 `[]string` 和 `[]any` 两种输入。

```go
func ContainsString(list []string, target string) bool
```

大小写不敏感的字符串包含判断。

```go
func ContainsSubstring(list []string, target string) bool
```

判断 `target` 是否包含列表中任一子串（大小写不敏感），可用于模糊匹配场景。

---

## 八、快速选择指南

| 指标类型 | 使用模板 | 定义表 | 操作符集 | 评估函数 |
|---------|---------|--------|---------|---------|
| 数值型（PE/PB/ROE等） | `BuildRangeSignals` + `RangeDef` | `NumberOpsByUnit()` | `EvalNumberOp` |
| 枚举型（板块/行业） | `BuildEnumSignals` + `EnumDef` | `EnumOps()` | `EvalEnumOp` |
| 布尔型（是否触发） | `DefaultBoolSignals` | `BoolOps` | 手写逻辑 |
| 序列型（交叉） | `SeriesSignalCrossAbove/Below` | `CrossOps` | 手写逻辑 |
| 时间窗口 | `ParamLookbackStart/End` + `NormalizeLookback` | — | 手写遍历 |

---

## 九、流程图

### 新增一个数值型指标的信号

```
1. 定义 RangeDef 列表 → 2. 选取操作符集 → 3. BuildRangeSignals 批量生成
                                                      ↓
                                              4. wrapFn 包装为自定义 Signal 类型
                                                      ↓
                                              5. SetBuiltInSignals / SetCustomSignals
                                                      ↓
                                              6. Evaluate 中调用 EvalNumberOp
```

### 新增一个枚举型指标的信号

```
1. 定义 EnumDef 列表 → 2. EnumOps(options) 生成操作符 → 3. BuildEnumSignals 批量生成
                                                              ↓
                                                      4. wrapFn 包装为自定义 Signal 类型
                                                              ↓
                                                      5. SetBuiltInSignals / SetCustomSignals
                                                              ↓
                                                      6. Evaluate 中调用 EvalEnumOp
```
