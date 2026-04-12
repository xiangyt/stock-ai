package service

import (
	"fmt"
	"math/rand"
	"time"

	"stock-ai/internal/model"
)

// ========== Mock 数据源适配器 (开发测试用) ==========

type mockAdapter struct{}

func newMockAdapter() *mockAdapter {
	return &mockAdapter{}
}

func (m *mockAdapter) Name() string {
	return "mock"
}

func (m *mockAdapter) Init(config string) error { return nil }

func (m *mockAdapter) GetStockList() ([]model.Stock, error) {
	rand.Seed(time.Now().UnixNano())
	
	mockData := []struct{
		Code         string
		Name         string
		Exchange     string
		ListingBoard string
		IssuePrice   float64
		IssuePE      float64
		ListDate     string
		Industry     string
	}{
		{"000001", "平安银行", model.ExchangeSZSE, model.BoardMain, 10.8, 38.86, "1991-04-03", "银行"},
		{"000002", "万科A", model.ExchangeSZSE, model.BoardMain, 14.58, 22.68, "1991-01-29", "房地产"},
		{"000063", "中兴通讯", model.ExchangeSZSE, model.BoardMain, 6.81, 18.56, "1997-11-18", "通信"},
		{"000100", "TCL科技", model.ExchangeSZSE, model.BoardMain, 1.0, 15.28, "2004-01-30", "电子"},
		{"000333", "美的集团", model.ExchangeSZSE, model.BoardMain, 16.16, 19.47, "2013-09-18", "家电"},
		{"000568", "泸州老窖", model.ExchangeSZSE, model.BoardMain, 5.25, 20.07, "1994-05-09", "白酒"},
		{"000651", "格力电器", model.ExchangeSZSE, model.BoardMain, 2.5, 22.65, "1996-11-18", "家电"},
		{"000725", "京东方A", model.ExchangeSZSE, model.BoardMain, 1.03, 15.75, "2001-01-12", "电子"},
		{"000858", "五粮液", model.ExchangeSZSE, model.BoardMain, 9.48, 17.33, "1998-04-21", "白酒"},
		{"002001", "新和成", model.ExchangeSZSE, model.BoardChiNext, 13.41, 24.37, "2004-06-25", "化工"},
		{"002230", "科大讯飞", model.ExchangeSZSE, model.BoardChiNext, 21.67, 139.73, "2008-05-12", "AI"},
		{"002415", "海康威视", model.ExchangeSZSE, model.BoardMain, 30.0, 49.88, "2010-05-28", "安防"},
		{"002460", "赣锋锂业", model.ExchangeSZSE, model.BoardMain, 28.0, 28.89, "2010-08-10", "新能源"},
		{"002475", "立讯精密", model.ExchangeSZSE, model.BoardMain, 23.8, 82.14, "2010-09-15", "电子"},
		{"002594", "比亚迪", model.ExchangeSZSE, model.BoardMain, 18.2, 26.62, "2011-06-30", "汽车"},
		{"300014", "亿纬锂能", model.ExchangeSZSE, model.BoardChiNext, 18.0, 27.08, "2009-10-30", "新能源"},
		{"300059", "东方财富", model.ExchangeSZSE, model.BoardChiNext, 40.58, 116.93, "2010-03-19", "金融"},
		{"300122", "智飞生物", model.ExchangeSZSE, model.BoardChiNext, 11.0, 51.83, "2010-09-28", "医药"},
		{"300124", "汇川技术", model.ExchangeSZSE, model.BoardChiNext, 71.0, 87.69, "2010-09-28", "工业"},
		{"300274", "阳光电源", model.ExchangeSZSE, model.BoardChiNext, 60.28, 72.11, "2011-11-02", "新能源"},
		{"300750", "宁德时代", model.ExchangeSZSE, model.BoardChiNext, 25.14, 39.31, "2018-06-11", "新能源"},
		{"600000", "浦发银行", model.ExchangeSSE, model.BoardMain, 10.0, 19.59, "1999-11-10", "银行"},
		{"600009", "上海机场", model.ExchangeSSE, model.BoardMain, 12.43, 29.95, "1998-02-02", "交通"},
		{"600016", "民生银行", model.ExchangeSSE, model.BoardMain, 11.8, 21.45, "2000-12-19", "银行"},
		{"600028", "中国石化", model.ExchangeSSE, model.BoardMain, 3.92, 19.98, "2001-08-08", "石油"},
		{"600030", "中信证券", model.ExchangeSSE, model.BoardMain, 4.50, 36.99, "2003-01-06", "证券"},
		{"600031", "三一重工", model.ExchangeSSE, model.BoardMain, 10.51, 22.34, "2003-07-03", "机械"},
		{"600036", "招商银行", model.ExchangeSSE, model.BoardMain, 7.03, 21.63, "2002-04-09", "银行"},
		{"600048", "保利发展", model.ExchangeSSE, model.BoardMain, 2.85, 55.40, "2006-07-31", "房地产"},
		{"600050", "中国联通", model.ExchangeSSE, model.BoardMain, 2.84, 203.00, "2002-10-09", "通信"},
		{"600104", "上汽集团", model.ExchangeSSE, model.BoardMain, 5.66, 36.61, "1997-11-25", "汽车"},
		{"600196", "复星医药", model.ExchangeSSE, model.BoardMain, 7.35, 53.57, "1998-08-07", "医药"},
		{"600276", "恒瑞医药", model.ExchangeSSE, model.BoardMain, 11.98, 44.70, "2000-10-18", "医药"},
		{"600309", "万华化学", model.ExchangeSSE, model.BoardMain, 11.78, 32.54, "2001-01-05", "化工"},
		{"600519", "贵州茅台", model.ExchangeSSE, model.BoardMain, 31.35, 20.97, "2001-08-27", "白酒"},
		{"600585", "海螺水泥", model.ExchangeSSE, model.BoardMain, 4.10, 12.80, "2002-02-07", "建材"},
		{"600588", "用友网络", model.ExchangeSSE, model.BoardMain, 36.68, 180.76, "2001-05-18", "软件"},
		{"600690", "海尔智家", model.ExchangeSSE, model.BoardMain, 7.49, 22.74, "1993-11-19", "家电"},
		{"600703", "三安光电", model.ExchangeSSE, model.BoardMain, 8.0, 42.96, "2003-07-08", "半导体"},
		{"600809", "山西汾酒", model.ExchangeSSE, model.BoardMain, 4.70, 46.94, "1994-01-21", "白酒"},
		{"600887", "伊利股份", model.ExchangeSSE, model.BoardMain, 8.79, 28.52, "1996-03-12", "食品"},
		{"600900", "长江电力", model.ExchangeSSE, model.BoardMain, 4.30, 33.39, "2003-11-18", "电力"},
		{"601012", "隆基绿能", model.ExchangeSSE, model.BoardMain, 2.12, 22.91, "2012-04-11", "光伏"},
		{"601088", "中国神华", model.ExchangeSSE, model.BoardMain, 7.97, 18.64, "2007-10-09", "煤炭"},
		{"601111", "中国国航", model.ExchangeSSE, model.BoardMain, 2.95, 18.67, "2006-08-18", "航空"},
		{"601138", "工业富联", model.ExchangeSSE, model.BoardMain, 13.77, 16.09, "2015-06-01", "制造"},
		{"601166", "兴业银行", model.ExchangeSSE, model.BoardMain, 2.76, 15.99, "2007-02-05", "银行"},
		{"601225", "陕西煤业", model.ExchangeSSE, model.BoardMain, 5.60, 8.89, "2014-01-28", "煤炭"},
		{"601228", "广州港", model.ExchangeSSE, model.BoardMain, 2.29, 42.93, "2017-03-29", "港口"},
		{"601288", "农业银行", model.ExchangeSSE, model.BoardMain, 2.68, 14.56, "2010-07-15", "银行"},
		{"601318", "中国平安", model.ExchangeSSE, model.BoardMain, 23.20, 15.19, "2007-03-01", "保险"},
		{"601328", "交通银行", model.ExchangeSSE, model.BoardMain, 3.10, 23.60, "2007-05-15", "银行"},
		{"601390", "中铁工业", model.ExchangeSSE, model.BoardMain, 3.18, 15.99, "2007-12-03", "建筑"},
		{"601398", "工商银行", model.ExchangeSSE, model.BoardMain, 3.23, 14.43, "2006-10-27", "银行"},
		{"601618", "中国中冶", model.ExchangeSSE, model.BoardMain, 2.10, 20.56, "2009-09-21", "建筑"},
		{"601628", "中国人寿", model.ExchangeSSE, model.BoardMain, 18.88, 31.97, "2007-01-09", "保险"},
		{"601633", "长城汽车", model.ExchangeSSE, model.BoardMain, 13.0, 63.60, "2011-09-28", "汽车"},
		{"601668", "中国建筑", model.ExchangeSSE, model.BoardMain, 4.18, 4.50, "2009-07-29", "建筑"},
		{"601688", "华泰证券", model.ExchangeSSE, model.BoardMain, 20.0, 33.28, "2010-02-26", "证券"},
		{"601728", "中国电信", model.ExchangeSSE, model.BoardMain, 2.26, 9.50, "2021-08-20", "通信"},
		{"601766", "中国中车", model.ExchangeSSE, model.BoardMain, 5.53, 38.87, "2008-08-18", "铁路设备"},
		{"601799", "星宇股份", model.ExchangeSSE, model.BoardMain, 26.00, 22.92, "2011-02-01", "汽配"},
		{"601816", "京沪高铁", model.ExchangeSSE, model.BoardMain, 4.88, 33.56, "2020-01-16", "铁路"},
		{"601838", "成都银行", model.ExchangeSSE, model.BoardMain, 6.60, 9.44, "2018-01-31", "银行"},
		{"601857", "中国石油", model.ExchangeSSE, model.BoardMain, 16.70, 22.44, "2007-11-05", "石油"},
		{"601888", "中国中免", model.ExchangeSSE, model.BoardMain, 15.50, 20.24, "2009-10-23", "旅游"},
		{"601899", "紫金矿业", model.ExchangeSSE, model.BoardMain, 7.13, 25.01, "2008-04-25", "矿业"},
		{"601919", "中远海控", model.ExchangeSSE, model.BoardMain, 2.40, 26.99, "2007-06-26", "航运"},
		{"601985", "中国核电", model.ExchangeSSE, model.BoardMain, 3.39, 26.02, "2015-06-10", "核电"},
		{"601989", "中国人民保", model.ExchangeSSE, model.BoardMain, 3.34, 48.03, "2018-11-16", "保险"},
		{"603019", "中科曙光", model.ExchangeSSE, model.BoardStar, 11.50, 22.99, "2014-01-21", "计算机"},
		{"603160", "汇顶科技", model.ExchangeSSE, model.BoardStar, 19.42, 22.99, "2016-10-19", "芯片"},
		{"603259", "药明康德", model.ExchangeSSE, model.BoardMain, 21.60, 22.99, "2018-05-08", "医药"},
		{"603288", "海天味业", model.ExchangeSSE, model.BoardMain, 51.28, 49.96, "2014-02-11", "食品"},
		{"603290", "斯达半导", model.ExchangeSSE, model.BoardStar, 12.74, 46.74, "2020-01-14", "芯片"},
		{"603501", "韦尔股份", model.ExchangeSSE, model.BoardStar, 28.01, 55.52, "2017-05-04", "芯片"},
		{"603799", "华友钴业", model.ExchangeSSE, model.BoardMain, 4.24, 20.67, "2015-01-29", "有色"},
		{"603833", "欧派家居", model.ExchangeSSE, model.BoardMain, 115.0, 22.99, "2017-03-28", "家具"},
		{"603986", "兆易创新", model.ExchangeSSE, model.BoardStar, 23.26, 55.40, "2016-08-18", "芯片"},
		{"603993", "洛阳钼业", model.ExchangeSSE, model.BoardMain, 2.91, 13.42, "2012-09-24", "有色"},
		{"688001", "华兴源创", model.ExchangeSSE, model.BoardStar, 24.26, 46.74, "2019-07-22", "检测"},
		{"688005", "容百科技", model.ExchangeSSE, model.BoardStar, 26.62, 37.46, "2019-07-22", "电池材料"},
		{"688007", "光峰科技", model.ExchangeSSE, model.BoardStar, 14.88, 47.89, "2019-07-22", "光学"},
		{"688012", "中微公司", model.ExchangeSSE, model.BoardStar, 29.01, 170.75, "2019-07-22", "半导体设备"},
		{"688016", "心脉医疗", model.ExchangeSSE, model.BoardStar, 46.23, 50.70, "2019-07-22", "医疗器械"},
		{"688018", "乐鑫科技", model.ExchangeSSE, model.BoardStar, 62.60, 56.94, "2019-07-22", "芯片"},
		{"688019", "安集科技", model.ExchangeSSE, model.BoardStar, 43.93, 59.32, "2019-07-22", "化学材料"},
		{"688023", "安恒信息", model.ExchangeSSE, model.BoardStar, 37.88, 45.02, "2019-11-06", "网络安全"},
		{"688025", "杰普特", model.ExchangeSSE, model.BoardStar, 30.53, 49.69, "2019-10-30", "激光"},
		{"688037", "芯源微", model.ExchangeSSE, model.BoardStar, 26.97, 55.62, "2019-12-16", "半导体设备"},
		{"688039", "当升科技", model.ExchangeSSE, model.BoardStar, 4.54, 23.30, "2020-07-17", "电池材料"},
		{"688041", "海光信息", model.ExchangeSSE, model.BoardStar, 36.35, 333.33, "2022-08-12", "CPU"},
		{"688056", "莱伯泰科", model.ExchangeSSE, model.BoardStar, 28.50, 72.94, "2020-09-25", "仪器"},
		{"688061", "精测电子", model.ExchangeSSE, model.BoardStar, 33.56, 156.59, "2021-01-28", "检测"},
		{"688066", "航天宏图", model.ExchangeSSE, model.BoardStar, 22.36, 77.15, "2019-07-22", "遥感"},
		{"688072", "拓荆科技", model.ExchangeSSE, model.BoardStar, 164.92, 609.52, "2022-04-20", "半导体设备"},
		{"688099", "晶晨股份", model.ExchangeSSE, model.BoardStar, 38.5, 167.03, "2019-08-08", "芯片"},
		{"688111", "金山办公", model.ExchangeSSE, model.BoardStar, 45.86, 208.12, "2019-11-18", "软件"},
		{"688117", "正德科技", model.ExchangeSSE, model.BoardStar, 25.90, 125.44, "2020-03-16", "新材料"},
		{"688120", "华丰测控", model.ExchangeSSE, model.BoardStar, 107.41, 152.87, "2020-07-22", "检测"},
		{"688122", "西部超导", model.ExchangeSSE, model.BoardStar, 15.16, 72.01, "2019-07-22", "材料"},
		{"688123", "聚辰股份", model.ExchangeSSE, model.BoardStar, 33.16, 53.69, "2019-12-23", "存储芯片"},
		{"688126", "沪硅产业", model.ExchangeSSE, model.BoardStar, 3.89, -142.03, "2020-04-20", "硅片"},
		{"688127", "蓝特光学", model.ExchangeSSE, model.BoardStar, 15.41, 35.59, "2020-09-21", "光学"},
		{"688128", "中国电研", model.ExchangeSSE, model.BoardStar, 18.68, 28.76, "2019-11-05", "电器检测"},
		{"688130", "晶方科技", model.ExchangeSSE, model.BoardStar, 31.67, 60.17, "2019-12-06", "封测"},
		{"688136", "科兴制药", model.ExchangeSSE, model.BoardStar, 22.41, 103.03, "2020-08-17", "生物药"},
		{"688137", "近岸蛋白", model.ExchangeSSE, model.BoardStar, 106.58, 172.74, "2022-09-29", "试剂"},
		{"688140", "明冠新材", model.ExchangeSSE, model.BoardStar, 14.79, 55.29, "2020-11-27", "膜材料"},
		{"688145", "敏芯股份", model.ExchangeSSE, model.BoardStar, 62.60, 244.06, "2020-08-10", "MEMS"},
		{"688146", "中微公司", model.ExchangeSSE, model.BoardStar, 29.01, 170.75, "2019-07-22", "刻蚀机"},
		{"688150", "华润微", model.ExchangeSSE, model.BoardStar, 12.69, 83.48, "2020-02-27", "功率器件"},
		{"688151", "华兴源创", model.ExchangeSSE, model.BoardStar, 24.26, 46.74, "2019-07-22", "检测"},
		{"688153", "唯捷创芯", model.ExchangeSSE, model.BoardStar, 66.69, -183.07, "2022-04-12", "射频"},
		{"688155", "先惠技术", model.ExchangeSSE, model.BoardStar, 34.99, 73.62, "2020-08-04", "智能装备"},
		{"688157", "松井股份", model.ExchangeSSE, model.BoardStar, 14.56, 49.85, "2020-06-09", "涂层材料"},
		{"688158", "优刻得", model.ExchangeSSE, model.BoardStar, 33.23, -1818.59, "2020-01-20", "云计算"},
		{"688159", "有方科技", model.ExchangeSSE, model.BoardStar, 20.35, 49.31, "2020-07-22", "物联网模组"},
		{"688161", "天南电力", model.ExchangeSSE, model.BoardStar, 14.93, 46.74, "2020-12-18", "电缆附件"},
		{"688162", "中巨芯", model.ExchangeSSE, model.BoardStar, 5.69, -151.89, "2023-09-01", "湿电子化学品"},
		{"688163", "君实生物-U", model.ExchangeSSE, model.BoardStar, 55.5, -47.90, "2020-07-15", "创新药"},
		{"688165", "埃夫特-U", model.ExchangeSSE, model.BoardStar, 6.35, -245.74, "2020-07-15", "机器人"},
		{"688166", "博瑞医药", model.ExchangeSSE, model.BoardStar, 12.71, 110.56, "2019-11-08", "CDMO"},
		{"688169", "石头科技", model.ExchangeSSE, model.BoardStar, 271.12, 58.98, "2020-02-21", "扫地机器人"},
		{"688171", "纬德信息", model.ExchangeSSE, model.BoardStar, 28.49, 45.85, "2021-03-16", "软件"},
		{"688173", "德龙激光", model.ExchangeSSE, model.BoardStar, 30.18, 245.53, "2022-04-29", "激光"},
		{"688175", "高凌信息", model.ExchangeSSE, model.BoardStar, 14.07, 163.86, "2022-03-15", "安全产品"},
		{"688176", "亚虹医药-U", model.ExchangeSSE, model.BoardStar, 18.63, -38.74, "2022-02-08", "创新药"},
		{"688177", "百奥泰-U", model.ExchangeSSE, model.BoardStar, 32.76, -415.53, "2020-02-21", "生物药"},
		{"688178", "万德斯", model.ExchangeSSE, model.BoardStar, 12.12, 59.33, "2020-01-14", "环保"},
		{"688179", "阿拉丁", model.ExchangeSSE, model.BoardStar, 19.47, 53.01, "2020-10-30", "科研服务"},
		{"688180", "君派生物", model.ExchangeSSE, model.BoardStar, 30.56, -89.35, "2020-12-22", "体外诊断"},
		{"688181", "八亿时空", model.ExchangeSSE, model.BoardStar, 20.43, 39.99, "2020-01-23", "液晶材料"},
		{"688182", "灿勤科技", model.ExchangeSSE, model.BoardStar, 8.49, 136.74, "2020-11-18", "滤波器"},
		{"688185", "康希诺-U", model.ExchangeSSE, model.BoardStar, 209.71, -581.79, "2020-08-13", "疫苗"},
		{"688186", "广大特材", model.ExchangeSSE, model.BoardStar, 17.16, 46.74, "2020-02-11", "特殊合金"},
		{"688187", "时代电气", model.ExchangeSSE, model.BoardStar, 31.38, 23.04, "2021-09-07", "IGBT"},
		{"688188", "柏楚电子", model.ExchangeSSE, model.BoardStar, 168.53, 49.83, "2019-08-08", "激光控制"},
		{"688189", "南京新工", model.ExchangeSSE, model.BoardStar, 22.53, 22.99, "2020-03-30", "投资"},
		{"688190", "云路股份", model.ExchangeSSE, model.BoardStar, 46.63, 22.99, "2020-11-26", "非晶合金"},
		{"688192", "嘉必优", model.ExchangeSSE, model.BoardStar, 23.7, 46.74, "2020-10-22", "营养品"},
		{"688195", "首药控股-U", model.ExchangeSSE, model.BoardStar, 39.9, -33.75, "2022-03-28", "创新药"},
		{"688198", "佳驾科技", model.ExchangeSSE, model.BoardStar, 43.93, 89.62, "2020-07-22", "智能驾驶"},
		{"688199", "久日新材", model.ExchangeSSE, model.BoardStar, 54.7, 44.03, "2019-11-05", "光引发剂"},
		{"688200", "华峰测控", model.ExchangeSSE, model.BoardStar, 107.73, 60.08, "2020-02-18", "测试设备"},
	}

	stocks := make([]model.Stock, len(mockData))
	for i, d := range mockData {
		exchangeName := ""
		boardName := ""
		
		switch d.Exchange {
		case model.ExchangeSSE:
			exchangeName = "上海证券交易所"
		case model.ExchangeSZSE:
			exchangeName = "深圳证券交易所"
		case model.ExchangeBSE:
			exchangeName = "北京证券交易所"
		}
		
		switch d.ListingBoard {
		case model.BoardMain:
			boardName = "主板"
		case model.BoardChiNext:
			boardName = "创业板"
		case model.BoardStar:
			boardName = "科创板"
		case model.BoardBSE:
			boardName = "北交所"
		}
		
		stocks[i] = model.Stock{
			Code:            d.Code,
			Name:            d.Name,
			FullName:        d.Name,
			Exchange:        d.Exchange,
			ExchangeName:    exchangeName,
			ListingBoard:    d.ListingBoard,
			BoardName:       boardName,
			ListDate:        d.ListDate,
			IssuePrice:      d.IssuePrice,
			IssuePE:        d.IssuePE,
			IssuePB:        roundTo(d.IssuePrice / rand.Float64()*5+0.5, 2),
			IssueShares:     int64(rand.Intn(50000)+5000),
			Industry:        d.Industry,
			Sector:          "",
			Concept:         "",
			Status:          "normal",
			UpdateTime:      time.Now().Format("2006-01-02 15:04:05"),
			DataSources:     `[{"name":"mock","time":"` + time.Now().Format("2006-01-02 15:04:05") + `"}]`,
		}
	}
	
	return stocks, nil
}

func (m *mockAdapter) GetStockPrice(code string, date string) (*model.StockPrice, error) {
	if date == "" || date == "today" {
		date = getLatestTradeDay()
	}
	
	basePrice := float64(rand.Intn(200)+1) + rand.Float64()
	change := (rand.Float64() - 0.48) * 0.1 // 偏涨
	
	price := &model.StockPrice{
		StockCode:       code,
		Date:            date,
		Open:           basePrice * (1 + (rand.Float64()-0.5)*0.02),
		Close:          basePrice * (1 + change),
		High:           basePrice * (1 + change + rand.Float64()*0.03),
		Low:            basePrice * (1 + change - rand.Float64()*0.03),
		Volume:         int64(rand.Intn(10000000) + 100000),
		Amount:         basePrice * float64(rand.Intn(10000000)+100000) / 100,
		TurnoverRate:   rand.Float64() * 15,
		PreClose:       basePrice,
		Change:         basePrice * change,
		ChangePct:      change * 100,
		Amplitude:      rand.Float64() * 8,
		TotalMarketCap: basePrice * float64(rand.Intn(50000)+1000),
		CirculateMarketCap: basePrice * float64(rand.Intn(40000)+500),
		MA5:            basePrice * (1 + (rand.Float64()-0.5)*0.05),
		MA10:           basePrice * (1 + (rand.Float64()-0.5)*0.08),
		MA20:           basePrice * (1 + (rand.Float64()-0.5)*0.12),
		MACD:           (rand.Float64() - 0.5) * 2,
		MACDSignal:     (rand.Float64() - 0.5) * 2,
		MACDHist:       (rand.Float64() - 0.5) * 1,
		KDJ_K:          rand.Float64() * 100,
		KDJ_D:          rand.Float64() * 100,
		KDJ_J:          rand.Float64() * 100,
		RSI6:           rand.Float64() * 100,
		RSI12:          rand.Float64() * 100,
		BOLLUpper:      basePrice * 1.05,
		BOLLMid:         basePrice,
		BOLLLower:      basePrice * 0.95,
		SourceName:     "mock",
		CreatedAt:      time.Now(),
	}
	
	return price, nil
}

func (m *mockAdapter) GetAllPrices(codes []string) (map[string][]model.StockPrice, error) {
	result := make(map[string][]model.StockPrice)
	
	for _, code := range codes {
		prices := make([]model.StockPrice, 0, 30)
		basePrice := float64(rand.Intn(200)+1) + rand.Float64()
		
		for i := 0; i < 30; i++ {
			date := time.Now().AddDate(0, 0, -(30-i))
			change := (rand.Float64() - 0.48) * 0.1
			basePrice = basePrice * (1 + change*0.3)
			
			prices = append(prices, model.StockPrice{
				StockCode: code,
				Date:      date.Format("2006-01-02"),
				Open:      basePrice * (1 + (rand.Float64()-0.5)*0.02),
				Close:     basePrice,
				High:      basePrice * (1 + rand.Float64()*0.03),
				Low:       basePrice * (1 - rand.Float64()*0.03),
				Volume:    int64(rand.Intn(10000000) + 100000),
				Amount:    basePrice * float64(rand.Intn(10000000)) / 100,
				ChangePct: change * 100,
				SourceName: "mock",
			})
		}
		
		result[code] = prices
	}
	
	return result, nil
}

func (m *mockAdapter) CheckHealth() error { return nil }

// ========== 占位符：其他数据源适配器（待实现）==========

// TushareAdapter Tushare Pro 适配器
type tushareAdapter struct{ config string }

func newTushareAdapter(config string) *tushareAdapter   { return &tushareAdapter{config: config} }
func (a *tushareAdapter) Name() string                   { return "tushare" }
func (a *tushareAdapter) Init(config string) error        { return nil }
func (a *tushareAdapter) GetStockList() ([]model.Stock, error) {
	return []model.Stock{}, fmt.Errorf("tushare adapter not implemented")
}
func (a *tushareAdapter) GetStockPrice(code string, date string) (*model.StockPrice, error) {
	return nil, fmt.Errorf("tushare adapter not implemented")
}
func (a *tushareAdapter) GetAllPrices(codes []string) (map[string][]model.StockPrice, error) {
	return nil, fmt.Errorf("tushare adapter not implemented")
}
func (a *tushareAdapter) CheckHealth() error               { return nil }

// EastMoneyAdapter 东方财富适配器
type eastmoneyAdapter struct{ config string }

func newEastMoneyAdapter(config string) *eastmoneyAdapter { return &eastmoneyAdapter{config: config} }
func (a *eastmoneyAdapter) Name() string                  { return "eastmoney" }
func (a *eastmoneyAdapter) Init(config string) error      { return nil }
func (a *eastmoneyAdapter) GetStockList() ([]model.Stock, error) {
	return []model.Stock{}, fmt.Errorf("eastmoney adapter not implemented")
}
func (a *eastmoneyAdapter) GetStockPrice(code string, date string) (*model.StockPrice, error) {
	return nil, fmt.Errorf("eastmoney adapter not implemented")
}
func (a *eastmoneyAdapter) GetAllPrices(codes []string) (map[string][]model.StockPrice, error) {
	return nil, fmt.Errorf("eastmoney adapter not implemented")
}
func (a *eastmoneyAdapter) CheckHealth() error             { return nil }

// AkshareAdapter AKShare 适配器
type akshareAdapter struct{ config string }

func newAkshareAdapter(config string) *akshareAdapter { return &akshareAdapter{config: config} }
func (a *akshareAdapter) Name() string              { return "akshare" }
func (a *akshareAdapter) Init(config string) error  { return nil }
func (a *akshareAdapter) GetStockList() ([]model.Stock, error) {
	return []model.Stock{}, fmt.Errorf("akshare adapter not implemented")
}
func (a *akshareAdapter) GetStockPrice(code string, date string) (*model.StockPrice, error) {
	return nil, fmt.Errorf("akshare adapter not implemented")
}
func (a *akshareAdapter) GetAllPrices(codes []string) (map[string][]model.StockPrice, error) {
	return nil, fmt.Errorf("akshare adapter not implemented")
}
func (a *akshareAdapter) CheckHealth() error           { return nil }

// 辅助函数
func roundTo(v float64, decimals int) float64 {
	multiplier := 1.0
	for i := 0; i < decimals; i++ {
		multiplier *= 10
	}
	return float64(int(v*multiplier+0.5)) / multiplier
}

func getLatestTradeDay() string {
	today := time.Now()
	switch today.Weekday() {
	case time.Sunday:
		return today.AddDate(0, 0, -2).Format("2006-01-02")
	case time.Saturday:
		return today.AddDate(0, 0, -1).Format("2006-01-02")
	default:
		return today.Format("2006-01-02")
	}
}
