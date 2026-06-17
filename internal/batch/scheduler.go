package batch

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"stockhunter/internal/cache"
	"stockhunter/internal/repository"
)

func Start(repo *repository.Repository, cacheClient *cache.Client) *cron.Cron {
	c := cron.New()
	_, err := c.AddFunc("@every 15m", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cacheClient.Delete(ctx, "home:rankings", "rankings:top50", "sector:strength")
		if _, err := repo.Rankings(ctx, 5); err != nil {
			log.Printf("scheduled ranking warmup failed: %v", err)
		}
	})
	if err != nil {
		log.Printf("scheduler disabled: %v", err)
		return c
	}
	c.Start()
	return c
}
