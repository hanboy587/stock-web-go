package marketdata

import "testing"

func TestParseKRXRows(t *testing.T) {
	rows := []map[string]any{
		{
			"BAS_DD":     "20260617",
			"ISU_CD":     "005930",
			"ISU_NM":     "삼성전자",
			"MKT_NM":     "KOSPI",
			"TDD_OPNPRC": "82,000",
			"TDD_HGPRC":  "83,100",
			"TDD_LWPRC":  "81,700",
			"TDD_CLSPRC": "82,800",
			"ACC_TRDVOL": "12,345,678",
			"LIST_SHRS":  "5,969,782,550",
			"MKTCAP":     "494,297,595,400,000",
		},
		{
			"BAS_DD":     "bad-date",
			"ISU_CD":     "000000",
			"TDD_CLSPRC": "10,000",
		},
		{
			"BAS_DD":     "2026-06-17",
			"ISU_CD":     "",
			"TDD_CLSPRC": "10,000",
		},
	}

	prices := parseKRXRows(rows, "KOSPI")
	if len(prices) != 1 {
		t.Fatalf("expected 1 parsed price, got %d", len(prices))
	}
	price := prices[0]
	if price.Code != "005930" || price.Name != "삼성전자" || price.Market != "KOSPI" {
		t.Fatalf("unexpected identity fields: %#v", price)
	}
	if price.Date.Format("2006-01-02") != "2026-06-17" {
		t.Fatalf("unexpected date: %s", price.Date.Format("2006-01-02"))
	}
	if price.Open != 82000 || price.High != 83100 || price.Low != 81700 || price.Close != 82800 {
		t.Fatalf("unexpected OHLC: %#v", price)
	}
	if price.Volume != 12345678 {
		t.Fatalf("unexpected volume: %d", price.Volume)
	}
	if price.ListedShares != 5969782550 || price.MarketCap != 494297595400000 {
		t.Fatalf("unexpected capitalization fields: %#v", price)
	}
}
