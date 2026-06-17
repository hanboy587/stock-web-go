package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"stockhunter/internal/batch"
	"stockhunter/internal/cache"
	"stockhunter/internal/config"
	"stockhunter/internal/database"
	"stockhunter/internal/news"
	"stockhunter/internal/repository"
	"stockhunter/internal/web"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	cacheClient := cache.Connect(ctx, cfg.RedisAddr)
	defer cacheClient.Close()

	repo := repository.New(pool)
	scheduler := batch.Start(repo, cacheClient, cfg.KRXAuthKey, cfg.PublicDataServiceKey)
	defer scheduler.Stop()

	app := fiber.New(fiber.Config{
		AppName:      "StockHunter",
		ReadTimeout: 10 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if fiberErr, ok := err.(*fiber.Error); ok {
				return c.Status(fiberErr.Code).SendString(fiberErr.Message)
			}
			log.Printf("request error: %v", err)
			return c.Status(fiber.StatusInternalServerError).SendString("internal server error")
		},
	})
	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))

	app.Static("/static", "./static")
	web.Register(app, repo, cacheClient, news.New(), cfg.NewsQueries, marketDataStatus(cfg))

	go func() {
		if err := app.Listen(":" + cfg.Port); err != nil {
			log.Printf("server stopped: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func marketDataStatus(cfg config.Config) string {
	switch {
	case cfg.KRXAuthKey != "":
		return "공식 KRX Open API로 일별 종가를 DB에 누적 중입니다. 뉴스/이슈는 10분 캐시로 빠르게 갱신됩니다."
	case cfg.PublicDataServiceKey != "":
		return "공공데이터포털 금융위원회 주식시세정보로 일별 종가를 DB에 누적 중입니다. 뉴스/이슈는 10분 캐시로 빠르게 갱신됩니다."
	default:
		return "공식 종가 API 키가 아직 없어 가격 랭킹은 기본 데이터로 표시됩니다. 뉴스/이슈는 실시간 RSS 기반으로 빠르게 갱신됩니다."
	}
}
