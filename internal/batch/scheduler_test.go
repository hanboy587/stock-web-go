package batch

import (
	"context"
	"errors"
	"testing"
	"time"

	"stockhunter/internal/models"
)

type fakeDailyPriceProvider struct {
	byDate map[string][]models.DailyPrice
	errs   map[string]error
}

func (p fakeDailyPriceProvider) Enabled() bool { return true }
func (p fakeDailyPriceProvider) Name() string  { return "fake" }
func (p fakeDailyPriceProvider) FetchDailyPrices(_ context.Context, date time.Time) ([]models.DailyPrice, error) {
	key := date.Format("2006-01-02")
	if err := p.errs[key]; err != nil {
		return nil, err
	}
	return p.byDate[key], nil
}

func TestFetchRecentDailyPricesSkipsEmptyDays(t *testing.T) {
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	provider := fakeDailyPriceProvider{
		byDate: map[string][]models.DailyPrice{
			"2026-06-16": {{Code: "005930", Date: now.AddDate(0, 0, -2), Close: 82800}},
		},
		errs: map[string]error{},
	}

	prices, date, err := fetchRecentDailyPrices(context.Background(), provider, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if date.Format("2006-01-02") != "2026-06-16" {
		t.Fatalf("expected fallback date 2026-06-16, got %s", date.Format("2006-01-02"))
	}
	if len(prices) != 1 || prices[0].Code != "005930" {
		t.Fatalf("unexpected prices: %#v", prices)
	}
}

func TestFetchRecentDailyPricesReturnsLastError(t *testing.T) {
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	expected := errors.New("provider down")
	provider := fakeDailyPriceProvider{
		byDate: map[string][]models.DailyPrice{},
		errs: map[string]error{
			"2026-06-18": expected,
		},
	}

	_, _, err := fetchRecentDailyPrices(context.Background(), provider, now)
	if !errors.Is(err, expected) {
		t.Fatalf("expected provider error, got %v", err)
	}
}
