package batch

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"stockhunter/internal/cache"
	"stockhunter/internal/marketdata"
	"stockhunter/internal/repository"
)

func Start(repo *repository.Repository, cacheClient *cache.Client, publicDataKey string) *cron.Cron {
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

	publicDataClient := marketdata.NewPublicDataClient(publicDataKey)
	if publicDataClient.Enabled() {
		_, err = c.AddFunc("CRON_TZ=Asia/Seoul 35 18 * * 1-5", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()

			prices, err := publicDataClient.FetchDailyPrices(ctx, time.Now().In(time.FixedZone("KST", 9*60*60)))
			if err != nil {
				log.Printf("daily close fetch failed: %v", err)
				return
			}
			if err := repo.UpsertDailyPrices(ctx, prices); err != nil {
				log.Printf("daily close upsert failed: %v", err)
				return
			}
			cacheClient.Delete(ctx, "home:rankings", "rankings:top50", "sector:strength")
			log.Printf("daily close updated: %d rows", len(prices))
		})
		if err != nil {
			log.Printf("daily close scheduler disabled: %v", err)
		}
	} else {
		log.Printf("daily close updater disabled: PUBLIC_DATA_SERVICE_KEY is empty")
	}

	c.Start()
	return c
}
