package indicator

import "strings"

// ============================================================================
//  通用工具函数 — 各指标子包复用
// ============================================================================

// ToStringSlice 将任意值安全转换为 []string。
// 支持输入: []string (直接返回)、[]any (逐元素类型断言)、其他返回 nil。
//
// 典型场景: SignalConfig.Params["values"] 可能是 JSON 反序列化后的 []any
func ToStringSlice(val any) []string {
	switch v := val.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, len(v))
		for i, s := range v {
			result[i], _ = s.(string)
		}
		return result
	default:
		return nil
	}
}

// ContainsString 判断 target 是否存在于列表中 (大小写不敏感)。
// 使用 strings.EqualFold 进行匹配，适用于板块代码、行业名称等场景。
func ContainsString(list []string, target string) bool {
	for _, s := range list {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

// ContainsSubstring 判断目标字符串是否包含列表中任一子串 (大小写不敏感)。
// 典型场景: 概念标签模糊匹配，如 "半导体" 匹配 "半导体设备"
func ContainsSubstring(list []string, target string) bool {
	lowerTarget := strings.ToLower(target)
	for _, s := range list {
		if strings.Contains(lowerTarget, strings.ToLower(s)) {
			return true
		}
	}
	return false
}
