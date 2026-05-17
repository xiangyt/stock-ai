package subscription

import (
	"strings"
)

// ============================================================================
//  通知模板定义与渲染
// ============================================================================

// defaultSuccessTemplate 默认有匹配结果模板
const defaultSuccessTemplate = `【AI选股】策略订阅通知

策略: {strategy_name}
监控范围: {scope_label}
扫描数量: {total_scanned}
匹配数量: {match_count}
执行耗时: {duration_ms}ms
执行时间: {run_time}

匹配股票:
{match_list}`

// defaultEmptyTemplate 默认无匹配结果模板
const defaultEmptyTemplate = `【AI选股】策略订阅通知

策略: {strategy_name}
监控范围: {scope_label}
扫描数量: {total_scanned}
匹配数量: 0
执行耗时: {duration_ms}ms
执行时间: {run_time}

本次无匹配股票。`

// defaultErrorTemplate 默认执行失败模板
const defaultErrorTemplate = `【AI选股】策略订阅执行异常

策略: {strategy_name}
执行时间: {run_time}
错误信息: {error_msg}

请检查订阅配置或联系管理员。`

// RenderTemplate 使用 strings.ReplaceAll 替换 {variable_name} 占位符
func RenderTemplate(tpl string, vars map[string]string) string {
	result := tpl
	for key, value := range vars {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// GetDefaultTemplate 根据是否有匹配结果返回对应默认模板
func GetDefaultTemplate(hasMatch bool, hasError bool) string {
	if hasError {
		return defaultErrorTemplate
	}
	if hasMatch {
		return defaultSuccessTemplate
	}
	return defaultEmptyTemplate
}
