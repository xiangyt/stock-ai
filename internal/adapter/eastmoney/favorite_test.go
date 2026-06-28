package eastmoney

import (
	"context"
	"testing"
	"time"
)

// 测试用 Cookie（从浏览器复制完整 Cookie 字符串）
// 注意：Cookie 有时效性，过期后需重新获取
const testCookie = `qgqp_b_id=9cc772eb709a39db1c4b1d3b2b495a61; st_nvi=ltwZEMD4HvR_q5mX6umSU4614; nid18=01cf360e98c63eceacc1f7098d943859; nid18_create_time=1775478458565; gviem=6qOe7IlpawxD2gb3rfcq62305; gviem_create_time=1775478458565; mtp=1; ct=BOgpG1F2DxdHWnpE33yl_xkaOevAXay_Pp97Y_19I-N3Fq7jAaraFDd8OILtGh3PCmas4Mm196sa6MZSIQb_yFiETwVoB9CDST9eWn_VCNhkUFpWUUnqR1keoqO684zO-JmJQQ1NiQQXn02ayNGu0HEp6DtlFK-8iLoi_GiHrvU; ut=FobyicMgeV6ApU-UaaD9aB6DD5lKyTOArrY0pwbZqKUO0LKOFNd_-9gK0n5oV9VbF_FtpESb8lOhhClVbSzeg9ycTourI2SBNBp68-YSlr7i6QzGN-ONDIQIqs86Bb1StZnZJkJGTwIucY9oAUv0e1FwWpNyosSL3AEMRf-qWkPQ08DIvtHKFyIz35hMnqoOZJNajBOojGDq4TZNpAIIwL4FnTAp3Xnvic0ifBtvet5eQ3Gx8jt-hFAqsdko-v2d2-zf_Vvn3DDKVYZxwpJtj4qbLcOfU-qW852vZLwqLemXXi8KoGiLpuSZveV31M8o2NGvNtvtiO1SJnmoO1EuIdjejp4-n_GMWpe_zZMsPwOzOH_XN5GKl9j16jH4Y15EBjMflndKyrMeJxqTPBT8QcZ8XoJiVEVGX-HC9wKbW7z4H6moqZXDt8oCtd4nOornr_hGSUloE27cTYBDHXadxx9ubVBENfxIIau5_TpFhwB_n6eI4iCM1Q; pi=7742345711885460%3Bj7742345711885460%3B%E5%9C%A8%E5%8E%9F%E4%B8%83%E6%B5%B7%3BWGXyu7X%2BrATAHWUOUpIUGcH1n7VGTnbyzWBO4JETteoPHbdxJGEXQX4HG8hSoSr9iKaZr3BTiU45dz7blbWboO5RaaRMCHv8IQXwRx0guTUiBL2MD5mA7DRvLCjmaLRAQnQqlVgVpNeCsOase5npe7qvjI6G81ChbhWE04afom8%2FUOt2m9DCBBDoP%2FIG8w62MiAsaqVC%3BcOl7gUwbDlZchS0NiCSFSb7IsLe0PfK3%2F8VM2aNKBmO%2BPihGihtxfBQ4jpcVh5bXTcZ3Gj7Xy3ihiTPop3iYD%2B0ELGXhw1IG4xUcaZ%2F1dqXoklay%2BTxRytQ9FUpSD6FjQRqPZ6oVESu2Yn843DO%2BXMQhFfnvnw%3D%3D; uidal=7742345711885460%e5%9c%a8%e5%8e%9f%e4%b8%83%e6%b5%b7; sid=139173658; vtpst=|; emshistory=%5B%22%E6%B9%96%E5%8C%97%E5%AE%9C%E5%8C%96%22%2C%22%E5%B9%BF%E4%B8%9C%E5%BB%BA%E7%A7%91%22%2C%22%E5%AF%8C%E4%B9%90%E5%BE%B7%22%5D; st_si=88718431337976; st_asi=delete; rskey=FwWKPalJheWRCalZRbUUzNjBLRld3NVJpQT09SGLMF; st_pvi=75524296013643; st_sp=2026-05-16%2020%3A52%3A15; st_inirUrl=https%3A%2F%2Fwww.google.com%2F; st_sn=156; st_psi=20260628204707363-113200301201-3889760888`

// TestCreateGroup 测试新建自选分组。
// 运行方式: go test -run TestCreateGroup -v
func TestCreateGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过连通性测试（使用 -short 跳过）")
	}

	a := newTestAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	name := "策略13选20260628"
	result, err := a.CreateGroup(ctx, testCookie, name)
	if err != nil {
		t.Fatalf("新建分组失败: %v", err)
	}
	t.Logf("新建分组成功: gid=%s, gname=%s", result.GID, result.GName)
}

// TestAddToGroup 测试将股票加入自选分组。
// 运行方式: go test -run TestAddToGroup -v
func TestAddToGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过连通性测试（使用 -short 跳过）")
	}

	a := newTestAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 注意：gid 需替换为实际存在的分组 ID
	gid := "17"
	stockCode := "688060" // 科创板 → 自动用 1$688060
	err := a.AddToGroup(ctx, testCookie, gid, stockCode)
	if err != nil {
		t.Fatalf("加入自选失败: %v", err)
	}
	t.Logf("加入自选成功: gid=%s, code=%s", gid, stockCode)
}

// TestFormatStockCode 测试股票代码格式化（单元测试，不依赖网络）。
func TestFormatStockCode(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"601318", "1$601318"},
		{"603259", "1$603259"},
		{"600519", "1$600519"},
		{"688060", "1$688060"},
		{"689009", "1$689009"},
		{"000001", "0$000001"},
		{"300750", "0$300750"},
	}
	for _, tt := range tests {
		got := formatStockCode(tt.code)
		if got != tt.want {
			t.Errorf("formatStockCode(%s) = %s; want %s", tt.code, got, tt.want)
		}
	}
}

// TestListGroups 测试获取自选分组列表。
// 运行方式: go test -run TestListGroups -v
func TestListGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过连通性测试（使用 -short 跳过）")
	}

	a := newTestAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	groups, err := a.ListGroups(ctx, testCookie)
	if err != nil {
		t.Fatalf("获取分组列表失败: %v", err)
	}
	t.Logf("共 %d 个分组:", len(groups))
	for _, g := range groups {
		t.Logf("  gid=%s gname=%s ver=%d", g.GID, g.GName, g.Ver)
	}
}

// TestRemoveFromGroup 测试将股票从自选分组中移除。
// 运行方式: go test -run TestRemoveFromGroup -v
func TestRemoveFromGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过连通性测试（使用 -short 跳过）")
	}

	a := newTestAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 注意：gid 需替换为实际存在的分组 ID
	gid := "17"
	stockCode := "002404"
	err := a.RemoveFromGroup(ctx, testCookie, gid, stockCode)
	if err != nil {
		t.Fatalf("移出自选失败: %v", err)
	}
	t.Logf("移出自选成功: gid=%s, code=%s", gid, stockCode)
}

// TestDeleteGroup 测试删除自选分组。
// 运行方式: go test -run TestDeleteGroup -v
func TestDeleteGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过连通性测试（使用 -short 跳过）")
	}

	a := newTestAdapter()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 注意：gid 需替换为实际存在的分组 ID
	gid := "12"
	err := a.DeleteGroup(ctx, testCookie, gid)
	if err != nil {
		t.Fatalf("删除分组失败: %v", err)
	}
	t.Logf("删除分组成功: gid=%s", gid)
}
