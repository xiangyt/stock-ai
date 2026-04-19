package helpers

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// CookieGenerator Cookie生成器
// 从 stock 项目移植，生成东方财富风格的随机Cookie
type CookieGenerator struct {
	rand *rand.Rand
}

// NewCookieGenerator 创建新的Cookie生成器
func NewCookieGenerator() *CookieGenerator {
	return &CookieGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateCookie 生成随机的东方财富风格Cookie
func (g *CookieGenerator) GenerateCookie() string {
	cookies := []string{
		fmt.Sprintf("qgqp_b_id=%s", g.randomHex(32)),
		fmt.Sprintf("st_nvi=%s", g.randomAlphaNum(25)),
		g.generateNid18(),
		g.generateNid18CreateTime(),
		g.generateGviem(),
		g.generateGviemCreateTime(),
		"speakVolume=100",
		"readStatus=pointRead",
		"batchReadIsOn=false",
		"guidesStatus=off",
		"highContrastMode=defaltMode",
		"cursorStatus=off",
		"magnifierIsOn=false",
		"readZoom=1",
		"percentStatus=100",
		"PointReadIsOn=false",
		"fontZoom=1",
		"speakFunctionIsOn=true",
		"textModeStatus=off",
		"speakSpeed=0",
		"wzaIsOn=false",
		"readScreen=false",
		"mtp=1",
		g.generateCt(),
		g.generateUt(), // cookie中的ut（区别于URL参数的ut）
		generatePi(),
		generateUidal(),
		fmt.Sprintf("sid=%d", g.rand.Int63n(900000000)+100000000),
		"vtpst=|",
		fmt.Sprintf("st_si=%d", g.rand.Int63n(99999999999999)+10000000000000),
		"fullscreengg=1",
		"fullscreengg2=1",
		g.generateWebsitepoptgApiTime(),
		"st_asi=delete",
		fmt.Sprintf("st_pvi=%d", g.rand.Int63n(99999999999999)+10000000000000),
		g.generateStSp(),
		g.generateStInirUrl(),
		fmt.Sprintf("st_sn=%d", g.rand.Intn(500)+50),
		g.generateStPsi(),
	}

	return strings.Join(cookies, "; ")
}

func (g *CookieGenerator) generateNidCreateTime() string {
	now := time.Now()
	past := now.AddDate(0, 0, -30)
	ts := past.Unix() + g.rand.Int63n(now.Unix()-past.Unix())
	return fmt.Sprintf("nid_create_time=%d", ts*1000+int64(g.rand.Intn(1000)))
}

func (g *CookieGenerator) generateGviCreateTime() string {
	now := time.Now()
	past := now.AddDate(0, 0, -30)
	ts := past.Unix() + g.rand.Int63n(now.Unix()-past.Unix())
	return fmt.Sprintf("gvi_create_time=%d", ts*1000+int64(g.rand.Intn(1000)))
}

func (g *CookieGenerator) generateWebsitepoptgApiTime() string {
	now := time.Now()
	past := now.AddDate(0, 0, -7)
	ts := past.Unix() + g.rand.Int63n(now.Unix()-past.Unix())
	return fmt.Sprintf("websitepoptg_api_time=%d", ts*1000+int64(g.rand.Intn(1000)))
}

func (g *CookieGenerator) generateStSp() string {
	now := time.Now()
	days := g.rand.Intn(7)
	date := now.AddDate(0, 0, -days)
	hour := g.rand.Intn(24)
	minute := g.rand.Intn(60)
	second := g.rand.Intn(60)
	dt := fmt.Sprintf("%04d-%02d-%02d%%20%02d%%3A%02d%%3A%02d",
		date.Year(), date.Month(), date.Day(), hour, minute, second)
	return fmt.Sprintf("st_sp=%s", dt)
}

func (g *CookieGenerator) generateStInirUrl() string {
	urls := []string{
		"https%3A%2F%2Fdata.eastmoney.com%2Fgphg%2F",
		"https%3A%2F%2Fdata.eastmoney.com%2Fxjllb%2F",
		"https%3A%2F%2Fdata.eastmoney.com%2Fhsgtcg%2F",
	}
	url := urls[g.rand.Intn(len(urls))]
	return fmt.Sprintf("st_inirUrl=%s", url)
}

func (g *CookieGenerator) generateStPsi() string {
	now := time.Now()
	hour := g.rand.Intn(24)
	minute := g.rand.Intn(60)
	second := g.rand.Intn(60)
	ms := g.rand.Intn(1000)
	dateStr := fmt.Sprintf("%04d%02d%02d%02d%02d%02d%03d",
		now.Year(), now.Month(), now.Day(), hour, minute, second, ms)
	suffix1 := g.rand.Int63n(9999999999999) + 1000000000000
	suffix2 := g.rand.Int63n(9999999999) + 1000000000
	return fmt.Sprintf("st_psi=%s-%d-%d", dateStr, suffix1, suffix2)
}

// ========== 新增：浏览器Cookie关键字段生成 ==========

func (g *CookieGenerator) generateNid18() string {
	chars := "0123456789abcdef"
	result := make([]byte, 32)
	for i := range result {
		result[i] = chars[g.rand.Intn(len(chars))]
	}
	return fmt.Sprintf("nid18=%s", string(result))
}

func (g *CookieGenerator) generateNid18CreateTime() string {
	now := time.Now()
	past := now.AddDate(0, 0, -30)
	ts := past.Unix() + g.rand.Int63n(now.Unix()-past.Unix())
	return fmt.Sprintf("nid18_create_time=%d", ts*1000+int64(g.rand.Intn(1000)))
}

func (g *CookieGenerator) generateGviem() string {
	return fmt.Sprintf("gviem=%s", g.randomAlphaNum(26))
}

func (g *CookieGenerator) generateGviemCreateTime() string {
	now := time.Now()
	past := now.AddDate(0, 0, -30)
	ts := past.Unix() + g.rand.Int63n(now.Unix()-past.Unix())
	return fmt.Sprintf("gviem_create_time=%d", ts*1000+int64(g.rand.Intn(1000)))
}

func (g *CookieGenerator) generateCt() string {
	return fmt.Sprintf("ct=%s", g.randomAlphaNum(88))
}

func (g *CookieGenerator) generateUt() string {
	return fmt.Sprintf("ut=%s", g.randomAlphaNum(256))
}

func generatePi() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	uid := r.Int63n(9000000000000000) + 1000000000000000
	name := randomChineseName(r)
	hash := randomAlphaNumFixed(r, 16)
	return fmt.Sprintf("pi=%d%%3Bj%d%%3B%s%%3B%s", uid, uid, name, hash)
}

func generateUidal() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano() + 1))
	uid := r.Int63n(9000000000000000) + 1000000000000000
	name := randomChineseName(r)
	encodedName := urlEncodeChinese(name)
	return fmt.Sprintf("uidal=%d%s", uid, encodedName)
}

func randomChineseName(r *rand.Rand) string {
	surnames := []string{"张", "王", "李", "刘", "陈", "杨", "赵", "黄", "周", "吴",
		"徐", "孙", "胡", "朱", "高", "林", "何", "郭", "马", "罗"}
	givenNames := []string{"伟", "芳", "娜", "敏", "静", "丽", "强", "磊", "军", "洋",
		"勇", "艳", "杰", "娟", "涛", "明", "超", "秀英", "华", "慧"}
	return surnames[r.Intn(len(surnames))] + givenNames[r.Intn(len(givenNames))]
}

func urlEncodeChinese(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff { // CJK统一汉字
			result.WriteString(fmt.Sprintf("%%%X", r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func randomAlphaNumFixed(r *rand.Rand, n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = chars[r.Intn(len(chars))]
	}
	return string(result)
}

func (g *CookieGenerator) randomHex(length int) string {
	const chars = "0123456789abcdef"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[g.rand.Intn(len(chars))]
	}
	return string(result)
}

func (g *CookieGenerator) randomAlphaNum(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[g.rand.Intn(len(chars))]
	}
	return string(result)
}

// GenerateTHSCookie 生成同花顺风格的Cookie
func (g *CookieGenerator) GenerateTHSCookie(hexinV string) string {
	ts := time.Now().Unix()
	return fmt.Sprintf(
		"Hm_lvt_722143063e4892925903024537075d0=%d; "+
			"HMACCOUNT=17C55F0F7B5ABE69; Hm_lvt_929f8b362150b1f77b477230541dbbc2=%d; "+
			"Hm_lvt_78c58f01938e4d85eaf619eae71b4ed1=%d; Hm_lvt_69929b9dce4c22a060bd22d703b2a280=%d; "+
			"spversion=20130314; historystock=600930%%7C*%%7C001208%%7C*%%7C001201; "+
			"Hm_lpvt_929f8b362150b1f77b477230541dbbc2=%d; "+
			"Hm_lpvt_69929b9dce4c22a060bd22d703b2a280=%d; "+
			"Hm_lpvt_722143063e4892925903024537075d0d=%d; "+
			"Hm_lpvt_78c58f01938e4d85eaf619eae71b4ed1=%d; v=%s",
		ts, ts, ts, ts, ts, ts, ts, ts, hexinV,
	)
}
