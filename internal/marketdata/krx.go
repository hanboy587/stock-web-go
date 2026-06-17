package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"stockhunter/internal/models"
)

const krxBaseURL = "https://data-dbg.krx.co.kr/svc/apis"

type KRXClient struct {
	authKey string
	http    *http.Client
}

func NewKRXClient(authKey string) *KRXClient {
	return &KRXClient{
		authKey: strings.TrimSpace(authKey),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *KRXClient) Enabled() bool {
	return c != nil && c.authKey != ""
}

func (c *KRXClient) Name() string {
	return "KRX Open API 일별매매정보"
}

func (c *KRXClient) FetchDailyPrices(ctx context.Context, date time.Time) ([]models.DailyPrice, error) {
	if !c.Enabled() {
		return nil, nil
	}

	markets := []struct {
		endpoint string
		name     string
	}{
		{endpoint: "/sto/stk_bydd_trd", name: "KOSPI"},
		{endpoint: "/sto/ksq_bydd_trd", name: "KOSDAQ"},
		{endpoint: "/sto/knx_bydd_trd", name: "KONEX"},
	}

	var prices []models.DailyPrice
	for _, market := range markets {
		items, err := c.fetchMarket(ctx, market.endpoint, market.name, date)
		if err != nil {
			return nil, err
		}
		prices = append(prices, items...)
	}
	return prices, nil
}

func (c *KRXClient) fetchMarket(ctx context.Context, endpoint string, fallbackMarket string, date time.Time) ([]models.DailyPrice, error) {
	params := url.Values{}
	params.Set("basDd", date.Format("20060102"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, krxBaseURL+endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("AUTH_KEY", c.authKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("KRX API %s returned %s", endpoint, resp.Status)
	}

	var payload krxResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return parseKRXRows(payload.Rows, fallbackMarket), nil
}

func parseKRXRows(rows []map[string]any, fallbackMarket string) []models.DailyPrice {
	prices := make([]models.DailyPrice, 0, len(rows))
	for _, row := range rows {
		baseDate := pick(row, "BAS_DD", "basDd")
		parsedDate, err := parseKRXDate(baseDate)
		if err != nil {
			continue
		}
		code := pick(row, "ISU_CD", "SRTN_CD", "isuCd", "srtnCd")
		closePrice := parseNumber(pick(row, "TDD_CLSPRC", "CLSPRC", "clpr", "close"))
		if code == "" || closePrice <= 0 {
			continue
		}

		market := pick(row, "MKT_NM", "mrktCtg", "market")
		if market == "" {
			market = fallbackMarket
		}
		marketCap := parseNumber(pick(row, "MKTCAP", "LIST_MKTCAP", "mrktTotAmt"))

		prices = append(prices, models.DailyPrice{
			Code:         code,
			Name:         pick(row, "ISU_NM", "ITMS_NM", "itmsNm", "name"),
			Market:       market,
			Date:         parsedDate,
			Open:         parseNumber(pick(row, "TDD_OPNPRC", "OPNPRC", "mkp", "open")),
			High:         parseNumber(pick(row, "TDD_HGPRC", "HGPRC", "hipr", "high")),
			Low:          parseNumber(pick(row, "TDD_LWPRC", "LWPRC", "lopr", "low")),
			Close:        closePrice,
			Volume:       int64(parseNumber(pick(row, "ACC_TRDVOL", "TRDVOL", "trqu", "volume"))),
			ListedShares: parseNumber(pick(row, "LIST_SHRS", "LIST_SHARES", "lstgStCnt")),
			MarketCap:    marketCap,
		})
	}
	return prices
}

type krxResponse struct {
	Rows []map[string]any `json:"OutBlock_1"`
}

func pick(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func parseKRXDate(value string) (time.Time, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), "-", "")
	return time.Parse("20060102", value)
}
