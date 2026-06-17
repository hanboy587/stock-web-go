package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"stockhunter/internal/models"
)

type PublicDataClient struct {
	serviceKey string
	http       *http.Client
}

func NewPublicDataClient(serviceKey string) *PublicDataClient {
	return &PublicDataClient{
		serviceKey: strings.TrimSpace(serviceKey),
		http:       &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *PublicDataClient) Enabled() bool {
	return c != nil && c.serviceKey != ""
}

func (c *PublicDataClient) FetchDailyPrices(ctx context.Context, date time.Time) ([]models.DailyPrice, error) {
	if !c.Enabled() {
		return nil, nil
	}

	params := url.Values{}
	params.Set("serviceKey", c.serviceKey)
	params.Set("resultType", "json")
	params.Set("pageNo", "1")
	params.Set("numOfRows", "5000")
	params.Set("basDt", date.Format("20060102"))

	endpoint := "https://apis.data.go.kr/1160100/service/GetStockSecuritiesInfoService/getStockPriceInfo?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("public data API returned %s", resp.Status)
	}

	var payload publicDataResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	items := payload.Response.Body.Items.Item
	prices := make([]models.DailyPrice, 0, len(items))
	for _, item := range items {
		parsedDate, err := time.Parse("20060102", item.BaseDate)
		if err != nil {
			continue
		}
		prices = append(prices, models.DailyPrice{
			Code:         item.ShortCode,
			Name:         item.Name,
			Market:       item.Market,
			Date:         parsedDate,
			Open:         parseNumber(item.Open),
			High:         parseNumber(item.High),
			Low:          parseNumber(item.Low),
			Close:        parseNumber(item.Close),
			Volume:       int64(parseNumber(item.Volume)),
			ListedShares: parseNumber(item.ListedShares),
			MarketCap:    parseNumber(item.MarketCap),
		})
	}
	return prices, nil
}

func parseNumber(value string) float64 {
	value = strings.ReplaceAll(value, ",", "")
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

type publicDataResponse struct {
	Response struct {
		Body struct {
			Items struct {
				Item []publicDataItem `json:"item"`
			} `json:"items"`
		} `json:"body"`
	} `json:"response"`
}

type publicDataItem struct {
	BaseDate     string `json:"basDt"`
	ShortCode    string `json:"srtnCd"`
	Name         string `json:"itmsNm"`
	Market       string `json:"mrktCtg"`
	Open         string `json:"mkp"`
	High         string `json:"hipr"`
	Low          string `json:"lopr"`
	Close        string `json:"clpr"`
	Volume       string `json:"trqu"`
	ListedShares string `json:"lstgStCnt"`
	MarketCap    string `json:"mrktTotAmt"`
}
