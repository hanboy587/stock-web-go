package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

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
	scheduler := batch.Start(repo, cacheClient, cfg.PublicDataServiceKey)
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

	app.Static("/static", "./static")
	web.Register(app, repo, cacheClient, news.New(), cfg.NewsQueries)

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
