package helpers

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// UserAgentGenerator 用户代理生成器
// 从 stock 项目移植
type UserAgentGenerator struct {
	rand *rand.Rand
}

// NewUserAgentGenerator 创建新的用户代理生成器
func NewUserAgentGenerator() *UserAgentGenerator {
	return &UserAgentGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateUserAgent 生成随机的User-Agent
func (g *UserAgentGenerator) GenerateUserAgent() string {
	browsers := []string{"Chrome", "Firefox", "Safari", "Edge"}

	chromeVersions := []string{
		"120.0.0.0", "121.0.0.0", "122.0.0.0", "123.0.0.0", "124.0.0.0",
		"125.0.0.0", "126.0.0.0", "127.0.0.0", "128.0.0.0", "129.0.0.0",
		"130.0.0.0", "131.0.0.0", "132.0.0.0", "133.0.0.0", "134.0.0.0",
		"135.0.0.0", "136.0.0.0", "137.0.0.0", "138.0.0.0", "139.0.0.0",
	}

	firefoxVersions := []string{
		"118.0", "119.0", "120.0", "121.0", "122.0", "123.0", "124.0", "125.0",
		"126.0", "127.0", "128.0", "129.0", "130.0", "131.0", "132.0",
	}

	safariVersions := []string{"537.36", "605.1.15", "604.1.38", "605.1.15"}

	osList := []struct {
		name string
	}{
		{"Windows NT 10.0; Win64; x64"},
		{"Macintosh; Intel Mac OS X 10_15_7"},
		{"Macintosh; Intel Mac OS X 14_4_1"},
		{"X11; Linux x86_64"},
	}

	browser := browsers[g.rand.Intn(len(browsers))]
	osInfo := osList[g.rand.Intn(len(osList))]

	switch browser {
	case "Chrome":
		version := chromeVersions[g.rand.Intn(len(chromeVersions))]
		webkitVersion := safariVersions[g.rand.Intn(len(safariVersions))]
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/%s (KHTML, like Gecko) Chrome/%s Safari/%s",
			osInfo.name, webkitVersion, version, webkitVersion)
	case "Firefox":
		version := firefoxVersions[g.rand.Intn(len(firefoxVersions))]
		return fmt.Sprintf("Mozilla/5.0 (%s; rv:%s) Gecko/20100101 Firefox/%s",
			osInfo.name, version, version)
	default:
		version := chromeVersions[g.rand.Intn(len(chromeVersions))]
		webkitVersion := safariVersions[g.rand.Intn(len(safariVersions))]
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/%s (KHTML, like Gecko) Chrome/%s Safari/%s",
			osInfo.name, webkitVersion, version, webkitVersion)
	}
}

// GenerateSecChUa 生成对应的sec-ch-ua头
func (g *UserAgentGenerator) GenerateSecChUa(userAgent string) string {
	if strings.Contains(userAgent, "Chrome") && !strings.Contains(userAgent, "Edg") {
		return `"Not;A=Brand";v="99", "Google Chrome";v="139", "Chromium";v="139"`
	} else if strings.Contains(userAgent, "Firefox") {
		return `"Not;A=Brand";v="99", "Firefox";v="130"`
	}
	return `"Not;A=Brand";v="99", "Google Chrome";v="139", "Chromium":v="139"`
}

// GetPlatform 从User-Agent中提取平台信息
func GetPlatformFromUA(ua string) string {
	if strings.Contains(ua, "Windows") {
		return "Windows"
	} else if strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS X") {
		return "macOS"
	} else if strings.Contains(ua, "Linux") {
		return "Linux"
	}
	return "Unknown"
}
