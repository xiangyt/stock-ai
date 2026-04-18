package eastmoney

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"stock-ai/internal/adapter"
)

// ========== 基本面数据（F10） ==========

// GetStockDetail 获取股票基本资料（F10接口 RPT_F10_BASIC_ORGINFO）
//
// 数据来源: datacenter.eastmoney.com/securities/api/data/v1/get
// 报告名: RPT_F10_BASIC_ORGINFO (HSF10)
func (a *Adapter) GetStockDetail(ctx context.Context, code string) (*adapter.StockBasic, error) {
	secucode := buildSecucode(code)
	baseURL := "https://datacenter.eastmoney.com/securities/api/data/v1/get"
	params := url.Values{
		"reportName":   {"RPT_F10_BASIC_ORGINFO"},
		"columns":      {"ALL"},
		"quoteColumns": {""},
		"filter":       {fmt.Sprintf(`(SECUCODE="%s")`, secucode)},
		"pageNumber":   {"1"},
		"pageSize":     {"1"},
		"sortTypes":    {""},
		"sortColumns":  {""},
		"source":       {"HSF10"},
		"client":       {"PC"},
	}

	urlStr := baseURL + "?" + params.Encode()
	refer := "https://emweb.securities.eastmoney.com/"
	body, err := a.makeGetRequest(urlStr, refer)
	if err != nil {
		return nil, err
	}

	var resp basicOrgInfoResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}
	if !resp.Success || len(resp.Result.Data) == 0 {
		return nil, fmt.Errorf("未找到股票 %s 的基本资料信息", code)
	}

	item := resp.Result.Data[0]
	exchange, listingBoard := detectExchangeAndBoard(code)

	listDate := truncateDate(item.ListingDate)
	foundDate := truncateDate(item.FoundDate)

	basic := &adapter.StockBasic{
		Code:          item.SecurityCode,
		Name:          item.SecurityNameAbbr,
		FullName:      item.OrgName,
		FullNameEn:    strOrEmpty(item.OrgNameEn),
		FormerName:    strOrEmpty(item.FormerName),
		Exchange:      exchange,
		ListingBoard:  listingBoard,
		ListDate:      listDate,
		FoundDate:     foundDate,
		SecurityType:  item.SecurityType,
		Industry:      item.IndustryCSRC1,
		Sector:        item.EM2016,
		Province:      item.Province,
		Address:       item.Address,
		RegAddress:    item.RegAddress,
		RegCapital:    item.RegCapital,
		EmpNum:        item.EmpNum,
		President:     item.President,
		LegalPerson:   item.LegalPerson,
		Secretary:     item.Secretary,
		OrgTel:        item.OrgTel,
		OrgEmail:      item.OrgEmail,
		OrgWeb:        item.OrgWeb,
		OrgProfile:    trimSpaces(item.OrgProfile),
		BusinessScope: item.BusinessScope,
		MainBusiness:  item.MainBusiness,
		ActualHolder:  strOrEmpty(item.ActualHolder),
		Currency:      item.Currency,
	}

	if err := a.fillIPOInfo(basic, code); err != nil {
		log.Printf("[eastmoney] %s IPO信息获取失败(非致命): %v", code, err)
	}

	return basic, nil
}

// ========== F10 IPO 发行信息 (RPT_PCF10_ORG_ISSUEINFO) ==========

// issueInfoItem IPO发行单条记录
type issueInfoItem struct {
	Secucode        string  `json:"SECUCODE"`
	SecurityCode    string  `json:"SECURITY_CODE"`
	FoundDate       string  `json:"FOUND_DATE"`
	ListingDate     string  `json:"LISTING_DATE"`
	AfterIssuePE    float64 `json:"AFTER_ISSUE_PE"`
	OnlineIssueDate string  `json:"ONLINE_ISSUE_DATE"`
	IssueWay        string  `json:"ISSUE_WAY"`
	ParValue        float64 `json:"PAR_VALUE"`
	TotalIssueNum   int64   `json:"TOTAL_ISSUE_NUM"`
	IssuePrice      float64 `json:"ISSUE_PRICE"`
	DecSumIssueFee  float64 `json:"DEC_SUMISSUEFEE"`
	TotalFunds      float64 `json:"TOTAL_FUNDS"`
	NetRaiseFunds   float64 `json:"NET_RAISE_FUNDS"`
	OpenPrice       float64 `json:"OPEN_PRICE"`
	ClosePrice      float64 `json:"CLOSE_PRICE"`
	TurnoverRate    float64 `json:"TURNOVERRATE"`
	HighPrice       float64 `json:"HIGH_PRICE"`
	OfflineVapRatio float64 `json:"OFFLINE_VAP_RATIO"`
	OnlineIssueLwr  float64 `json:"ONLINE_ISSUE_LWR"`
	SecurityType    string  `json:"SECURITY_TYPE"`
	Overalllotment  float64 `json:"OVERALLOTMENT"`
	Type            string  `json:"TYPE"`
	TradeMarketCode string  `json:"TRADE_MARKET_CODE"`
	StrZhuchengxiao string  `json:"STR_ZHUCHENGXIAO"`
	StrBaojian      string  `json:"STR_BAOJIAN"`
}

func (a *Adapter) fillIPOInfo(basic *adapter.StockBasic, code string) error {
	secucode := buildSecucode(code)
	baseURL := "https://datacenter.eastmoney.com/securities/api/data/v1/get"
	params := url.Values{
		"reportName":   {"RPT_PCF10_ORG_ISSUEINFO"},
		"columns":      {"ALL"},
		"quoteColumns": {""},
		"filter":       {fmt.Sprintf(`(SECUCODE="%s")`, secucode)},
		"pageNumber":   {"1"},
		"pageSize":     {"1"},
		"sortTypes":    {""},
		"sortColumns":  {""},
		"source":       {"HSF10"},
		"client":       {"PC"},
	}

	urlStr := baseURL + "?" + params.Encode()
	refer := "https://emweb.securities.eastmoney.com/"
	body, err := a.makeGetRequest(urlStr, refer)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Result  struct {
			Pages int             `json:"pages"`
			Data  []issueInfoItem `json:"data"`
			Count int             `json:"count"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return fmt.Errorf("JSON解析失败: %w", err)
	}
	if !resp.Success || len(resp.Result.Data) == 0 {
		return fmt.Errorf("未找到 %s 的IPO信息", code)
	}

	item := resp.Result.Data[0]
	basic.IssuePrice = item.IssuePrice
	basic.IssuePE = item.AfterIssuePE
	basic.ParValue = item.ParValue
	basic.TotalIssueNum = item.TotalIssueNum
	basic.OnlineIssueDate = truncateDate(item.OnlineIssueDate)
	basic.IssueWay = item.IssueWay
	basic.Sponsor = item.StrZhuchengxiao
	basic.Underwriter = item.StrBaojian

	return nil
}

// ========== F10 基本资料响应结构体 ==========

type basicOrgInfoResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Result  basicOrgInfoResultData `json:"result"`
}

type basicOrgInfoResultData struct {
	Pages int                `json:"pages"`
	Data  []basicOrgInfoItem `json:"data"`
	Count int                `json:"count"`
}

// basicOrgInfoItem F10基本资料单条记录 (RPT_F10_BASIC_ORGINFO)
type basicOrgInfoItem struct {
	Secucode         string  `json:"SECUCODE"`
	SecurityCode     string  `json:"SECURITY_CODE"`
	SecurityNameAbbr string  `json:"SECURITY_NAME_ABBR"`
	OrgCode          string  `json:"ORG_CODE"`
	OrgName          string  `json:"ORG_NAME"`
	OrgNameEn        *string `json:"ORG_NAME_EN"`
	FormerName       *string `json:"FORMERNAME"`
	StrCodeA         string  `json:"STR_CODEA"`
	StrNameA         string  `json:"STR_NAMEA"`
	SecurityType     string  `json:"SECURITY_TYPE"`
	EM2016           string  `json:"EM2016"`
	TradeMarket      string  `json:"TRADE_MARKET"`
	IndustryCSRC1    string  `json:"INDUSTRYCSRC1"`
	President        string  `json:"PRESIDENT"`
	LegalPerson      string  `json:"LEGAL_PERSON"`
	Secretary        string  `json:"SECRETARY"`
	Chairman         string  `json:"CHAIRMAN"`
	OrgTel           string  `json:"ORG_TEL"`
	OrgEmail         string  `json:"ORG_EMAIL"`
	OrgFax           string  `json:"ORG_FAX"`
	OrgWeb           string  `json:"ORG_WEB"`
	Address          string  `json:"ADDRESS"`
	RegAddress       string  `json:"REG_ADDRESS"`
	Province         string  `json:"PROVINCE"`
	AddressPostcode  string  `json:"ADDRESS_POSTCODE"`
	RegCapital       float64 `json:"REG_CAPITAL"`
	RegNum           string  `json:"REG_NUM"`
	EmpNum           int     `json:"EMP_NUM"`
	TatolNumber      int     `json:"TATOLNUMBER"`
	LawFirm          string  `json:"LAW_FIRM"`
	AccountfirmName  string  `json:"ACCOUNTFIRM_NAME"`
	OrgProfile       string  `json:"ORG_PROFILE"`
	BusinessScope    string  `json:"BUSINESS_SCOPE"`
	ListingDate      string  `json:"LISTING_DATE"`
	FoundDate        string  `json:"FOUND_DATE"`
	MainBusiness     string  `json:"MAIN_BUSINESS"`
	HostBroker       *string `json:"HOST_BROKER"`
	TransferWay      *string `json:"TRANSFER_WAY"`
	ActualHolder     *string `json:"ACTUAL_HOLDER"`
	Currency         string  `json:"CURRENCY"`
	BoardNameLevel   string  `json:"BOARD_NAME_LEVEL"`
}
