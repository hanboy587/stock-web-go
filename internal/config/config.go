package config

import (
	"os"
	"strings"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	RedisAddr            string
	PublicDataServiceKey string
	NewsQueries          []string
}

func Load() Config {
	return Config{
		Port:                 env("PORT", "8080"),
		DatabaseURL:          env("DATABASE_URL", "postgres://stockhunter:stockhunter@localhost:5432/stockhunter?sslmode=disable"),
		RedisAddr:            env("REDIS_ADDR", "localhost:6379"),
		PublicDataServiceKey: env("PUBLIC_DATA_SERVICE_KEY", ""),
		NewsQueries: splitEnv("NEWS_QUERIES", []string{
			"코스피 OR 코스닥 증시",
			"국내 주식 시장 이슈",
			"기관 외국인 순매수",
			"반도체 자동차 방산 전력 바이오 조선 주식",
		}),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitEnv(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	var items []string
	for _, item := range strings.Split(value, "|") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	if len(items) == 0 {
		return fallback
	}
	return items
}
