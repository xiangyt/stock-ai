package helpers

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type UserAgentGenerator struct {
	rand *rand.Rand
}

func NewUserAgentGenerator() *UserAgentGenerator {
	return &UserAgentGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (g *UserAgentGenerator) GenerateUserAgent() string {
	browsers := []string{"Chrome", "Firefox", "Safari", "Edge"}
	chromeVersions := []string{
		"120.0.0.0", "121.0.0.0", "122.0.0.0", "123.0.0.0", "124.0.0.0",
		"125.0.0.0", "126.0.0.0", "127.0.0.0", "128.0.0.0", "129.0.0.0",
		"130.0.0.0", "131.0.0.0", "132.0.0.0", "133.0.0.0", "134.0.0.0",
		"135.0.0.0", "136.0.0.0", "137.0.0.0", "138.0.0.0", "139.0.0.0",
		"140.0.0.0", "141.0.0.0", "142.0.0.0", "143.0.0.0", "144.0.0.0",
		"145.0.0.0", "146.0.0.0", "147.0.0.0", "148.0.0.0", "149.0.0.0",
	}
	firefoxVersions := []string{
		"118.0", "119.0", "120.0", "121.0", "122.0", "123.0", "124.0", "125.0",
		"126.0", "127.0", "128.0", "129.0", "130.0", "131.0", "132.0",
		"133.0", "134.0", "135.0",
	}
	safariVersions := []string{"537.36", "605.1.15", "604.1.38", "605.1.15"}
	osList := []struct{ name string }{
		{"Windows NT 10.0; Win64; x64"},
		{"Macintosh; Intel Mac OS X 10_15_7"},
		{"X11; Linux x86_64"},
	}
	browser := browsers[g.rand.Intn(len(browsers))]
	osInfo := osList[g.rand.Intn(len(osList))]
	switch browser {
	case "Chrome":
		v := chromeVersions[g.rand.Intn(len(chromeVersions))]
		wv := safariVersions[g.rand.Intn(len(safariVersions))]
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/%s (KHTML, like Gecko) Chrome/%s Safari/%s", osInfo.name, wv, v, wv)
	case "Firefox":
		v := firefoxVersions[g.rand.Intn(len(firefoxVersions))]
		return fmt.Sprintf("Mozilla/5.0 (%s; rv:%s) Gecko/20100101 Firefox/%s", osInfo.name, v, v)
	case "Safari":
		wv := safariVersions[g.rand.Intn(len(safariVersions))]
		sv := fmt.Sprintf("17.%d", g.rand.Intn(5)+1)
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/%s (KHTML, like Gecko) Version/%s Safari/%s", osInfo.name, wv, sv, wv)
	case "Edge":
		ev := chromeVersions[g.rand.Intn(len(chromeVersions))]
		ewv := safariVersions[g.rand.Intn(len(safariVersions))]
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/%s (KHTML, like Gecko) Chrome/%s Safari/%s Edg/%s", osInfo.name, ewv, ev, ewv, ev)
	default:
		dv := chromeVersions[g.rand.Intn(len(chromeVersions))]
		dwv := safariVersions[g.rand.Intn(len(safariVersions))]
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/%s (KHTML, like Gecko) Chrome/%s Safari/%s", osInfo.name, dwv, dv, dwv)
	}
}

func (g *UserAgentGenerator) GenerateSecChUa(userAgent string) string {
	if strings.Contains(userAgent, "Chrome") && !strings.Contains(userAgent, "Edg") {
		cv := g.extractChromeVersion(userAgent)
		return "\x22Not;A=Brand\x22;v=\x22199\x22, \x22Google Chrome\x22;v=\x22" + cv + "\x22, \x22Chromium\x22;v=\x22" + cv + "\x22"
	}
	if strings.Contains(userAgent, "Firefox") {
		return "\x22Not;A=Brand\x22;v=\x22199\x22, \x22Firefox\x22;v=\x22130\x22"
	}
	if strings.Contains(userAgent, "Safari") && !strings.Contains(userAgent, "Chrome") {
		return "\x22Not;A=Brand\x22;v=\x22199\x22, \x22Safari\x22;v=\x2217\x22"
	}
	if strings.Contains(userAgent, "Edg") {
		ev := g.extractChromeVersion(userAgent)
		return "\x22Not;A=Brand\x22;v=\x22199\x22, \x22Microsoft Edge\x22;v=\x22" + ev + "\x22, \x22Chromium\x22;v=\x22" + ev + "\x22"
	}
	return "\x22Not;A=Brand\x22;v=\x22199\x22, \x22Google Chrome\x22;v=\x22139\x22, \x22Chromium\x22;v=\x22139\x22"
}

func (g *UserAgentGenerator) extractChromeVersion(ua string) string {
	idx := strings.Index(ua, "Chrome/")
	if idx >= 0 {
		start := idx + 7
		end := start
		for end < len(ua) && ua[end] != ' ' && ua[end] != '.' {
			end++
		}
		if end > start {
			ver := ua[start:end]
			dotIdx := strings.Index(ver, ".")
			if dotIdx >= 0 {
				return ver[:dotIdx]
			}
			return ver
		}
	}
	return "139"
}

func GetPlatformFromUA(ua string) string {
	switch {
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
		return "macOS"
	default:
		return "Unknown"
	}
}
