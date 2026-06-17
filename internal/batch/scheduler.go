package batch

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"stockhunter/internal/cache"
	"stockhunter/internal/marketdata"
	"stockhunter/internal/models"
	"stockhunter/internal/repository"
)

type dailyPriceProvider interface {
	Enabled() bool
	Name() string
	FetchDailyPrices(context.Context, time.Time) ([]models.DailyPrice, error)
}

func Start(repo *repository.Repository, cacheClient *cache.Client, krxAuthKey string, publicDataKey string, backfillDays int) *cron.Cron {
	c := cron.New()
	_, err := c.AddFunc("@every 15m", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cacheClient.Delete(ctx, "home:rankings", "rankings:top50", "sector:strength", "news:market")
		if _, err := repo.Rankings(ctx, 5); err != nil {
			log.Printf("scheduled ranking warmup failed: %v", err)
		}
	})
	if err != nil {
		log.Printf("scheduler disabled: %v", err)
		return c
	}

	provider := firstEnabledProvider(
		marketdata.NewKRXClient(krxAuthKey),
		marketdata.NewPublicDataClient(publicDataKey),
	)
	if provider != nil {
		run := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
			defer cancel()
			runDailyCloseUpdate(ctx, repo, cacheClient, provider)
		}

		_, err = c.AddFunc("CRON_TZ=Asia/Seoul 35 18 * * 1-5", run)
		if err != nil {
			log.Printf("daily close scheduler disabled: %v", err)
		}
		go func() {
			time.Sleep(3 * time.Second)
			timeout := time.Duration(backfillDays) * 45 * time.Second
			if timeout < 150*time.Second {
				timeout = 150 * time.Second
			}
			if timeout > 15*time.Minute {
				timeout = 15 * time.Minute
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			runDailyCloseBackfill(ctx, repo, cacheClient, provider, backfillDays)
		}()
		log.Printf("daily close updater enabled: %s", provider.Name())
	} else {
		log.Printf("daily close updater disabled: KRX_AUTH_KEY and PUBLIC_DATA_SERVICE_KEY are empty")
	}

	c.Start()
	return c
}

func firstEnabledProvider(providers ...dailyPriceProvider) dailyPriceProvider {
	for _, provider := range providers {
		if provider.Enabled() {
			return provider
		}
	}
	return nil
}

func runDailyCloseBackfill(ctx context.Context, repo *repository.Repository, cacheClient *cache.Client, provider dailyPriceProvider, days int) {
	if days <= 1 {
		runDailyCloseUpdate(ctx, repo, cacheClient, provider)
		return
	}

	now := time.Now().In(time.FixedZone("KST", 9*60*60))
	var updatedDates int
	var updatedRows int
	var lastErr error
	for dayOffset := 0; dayOffset < days; dayOffset++ {
		if ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}
		date := now.AddDate(0, 0, -dayOffset)
		prices, err := provider.FetchDailyPrices(ctx, date)
		if err != nil {
			lastErr = err
			continue
		}
		if len(prices) == 0 {
			continue
		}
		if err := repo.UpsertDailyPrices(ctx, prices); err != nil {
			log.Printf("daily close backfill upsert failed for %s: %v", date.Format("2006-01-02"), err)
			return
		}
		updatedDates++
		updatedRows += len(prices)
	}

	if updatedRows > 0 {
		cacheClient.Delete(ctx, "home:rankings", "rankings:top50", "sector:strength")
		log.Printf("daily close backfill updated from %s: %d dates, %d rows", provider.Name(), updatedDates, updatedRows)
		return
	}
	if lastErr != nil {
		log.Printf("daily close backfill failed from %s: %v", provider.Name(), lastErr)
		return
	}
	log.Printf("daily close backfill returned no rows from %s for %d days", provider.Name(), days)
}

func runDailyCloseUpdate(ctx context.Context, repo *repository.Repository, cacheClient *cache.Client, provider dailyPriceProvider) {
	now := time.Now().In(time.FixedZone("KST", 9*60*60))
	prices, date, err := fetchRecentDailyPrices(ctx, provider, now)
	if err != nil {
		log.Printf("daily close fetch failed from %s: %v", provider.Name(), err)
		return
	}
	if len(prices) == 0 {
		log.Printf("daily close fetch returned no rows from %s", provider.Name())
		return
	}
	if err := repo.UpsertDailyPrices(ctx, prices); err != nil {
		log.Printf("daily close upsert failed: %v", err)
		return
	}
	cacheClient.Delete(ctx, "home:rankings", "rankings:top50", "sector:strength")
	log.Printf("daily close updated from %s: %s %d rows", provider.Name(), date.Format("2006-01-02"), len(prices))
}

func fetchRecentDailyPrices(ctx context.Context, provider dailyPriceProvider, now time.Time) ([]models.DailyPrice, time.Time, error) {
	var lastErr error
	for dayOffset := 0; dayOffset < 8; dayOffset++ {
		date := now.AddDate(0, 0, -dayOffset)
		prices, err := provider.FetchDailyPrices(ctx, date)
		if err != nil {
			lastErr = err
			continue
		}
		if len(prices) > 0 {
			return prices, date, nil
		}
	}
	if lastErr != nil {
		return nil, time.Time{}, lastErr
	}
	return nil, time.Time{}, nil
}
