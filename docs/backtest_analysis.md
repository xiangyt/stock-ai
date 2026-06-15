# ai-stock-picker 回测功能实施 — 关键代码结构分析

> 分析日期: 2026-01-18
> 目标: 为实施回测功能，理解现有代码架构

---

## 一、项目总览

| 维度 | 说明 |
|------|------|
| 语言 | Go 1.21+ |
| ORM | GORM (MySQL) |
| 配置 | Viper (config.yaml) |
| HTTP框架 | Gin |
| 核心架构 | **指标引擎 + 信号体系** |

**目录结构**:
```
internal/
├── config/          # 配置管理
├── db/              # 数据库 & DAO 层
├── indicator/       # 指标引擎核心
│   ├── financial/   # 财务