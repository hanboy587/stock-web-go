package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"stockhunter/internal/cache"
	"stockhunter/internal/models"
	"stockhunter/internal/news"
	"stockhunter/internal/repository"
)

type Handler struct {
	repo             *repository.Repository
	cache            *cache.Client
	newsClient       *news.Client
	newsQueries      []string
	newsCategories   []models.NewsCategory
	marketDataConfig MarketDataConfig
	funcs            template.FuncMap
}

type MarketDataConfig struct {
	Provider     string
	Status       string
	BackfillDays int
}

type ViewData struct {
	Title              string
	Active             string
	Stocks             []models.StockMetric
	Stock              models.StockMetric
	Prices             []models.PricePoint
	Sectors            []string
	Strengths          []models.SectorStrength
	News               []models.NewsItem
	NewsCategories     []models.NewsCategory
	ActiveNewsCategory string
	Filters            models.Filters
	Generated          time.Time
	Error              string
	ResultPath         string
	MarketDataStatus   string
	PriceStatus        models.PriceStatus
}

func Register(app *fiber.App, repo *repository.Repository, cacheClient *cache.Client, newsClient *news.Client, newsQueries []string, marketDataConfig MarketDataConfig) {
	h := &Handler{
		repo:             repo,
		cache:            cacheClient,
		newsClient:       newsClient,
		newsQueries:      newsQueries,
		newsCategories:   defaultNewsCategories(newsQueries),
		marketDataConfig: marketDataConfig,
		funcs: template.FuncMap{
			"krw":        krw,
			"krwShort":   krwShort,
			"number":     number,
			"percent":    percent,
			"scoreWidth": scoreWidth,
			"scoreTone":  scoreTone,
			"timeAgo":    timeAgo,
			"date":       func(t time.Time) string { return t.Format("2006-01-02") },
		},
	}

	app.Get("/", h.home)
	app.Get("/screener", h.screener)
	app.Get("/rankings", h.rankings)
	app.Get("/sector", h.sector)
	app.Get("/news", h.news)
	app.Get("/stock/:code", h.stock)
	app.Get("/healthz", h.healthz)
	app.Get("/api/status", h.apiStatus)
	app.Get("/api/news", h.apiNews)
}

func (h *Handler) healthz(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(requestContext(c), 2*time.Second)
	defer cancel()

	checks := fiber.Map{
		"database": "ok",
		"redis":    "ok",
	}
	status := fiber.StatusOK
	if err := h.repo.Ping(ctx); err != nil {
		checks["database"] = err.Error()
		status = fiber.StatusServiceUnavailable
	}
	if err := h.cache.Ping(ctx); err != nil {
		checks["redis"] = err.Error()
		status = fiber.StatusServiceUnavailable
	}

	return c.Status(status).JSON(fiber.Map{
		"ok":        status == fiber.StatusOK,
		"checks":    checks,
		"generated": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) apiStatus(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(requestContext(c), 2*time.Second)
	defer cancel()

	priceStatus, err := h.repo.PriceStatus(ctx)
	if err != nil {
		return err
	}
	lastImport := any(nil)
	if run, ok, err := h.repo.LastDataImportRun(ctx); err != nil {
		return err
	} else if ok {
		lastImport = importRunMap(run)
	}
	return c.JSON(fiber.Map{
		"market_data": fiber.Map{
			"provider":      h.marketDataConfig.Provider,
			"status":        h.marketDataConfig.Status,
			"backfill_days": h.marketDataConfig.BackfillDays,
			"last_import":   lastImport,
		},
		"prices": fiber.Map{
			"latest_date": priceStatus.LatestDate.Format("2006-01-02"),
			"stock_count": priceStatus.StockCount,
			"price_count": priceStatus.PriceCount,
		},
		"news": fiber.Map{
			"cache_seconds":  600,
			"category_count": len(h.newsCategories),
			"query_count":    len(h.newsQueries),
		},
		"generated": time.Now().UTC().Format(time.RFC3339),
	})
}

func importRunMap(run models.DataImportRun) fiber.Map {
	return fiber.Map{
		"provider":     run.Provider,
		"mode":         run.Mode,
		"status":       run.Status,
		"started_at":   run.StartedAt.UTC().Format(time.RFC3339),
		"finished_at":  run.FinishedAt.UTC().Format(time.RFC3339),
		"target_start": dateOrEmpty(run.TargetStart),
		"target_end":   dateOrEmpty(run.TargetEnd),
		"rows":         run.Rows,
		"message":      run.Message,
	}
}

func dateOrEmpty(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func (h *Handler) apiNews(c *fiber.Ctx) error {
	ctx := requestContext(c)
	category := h.newsCategory(c.Query("category", "all"))
	limit := parseInt(c.Query("limit"), 36)
	if limit <= 0 || limit > 60 {
		limit = 36
	}
	return c.JSON(fiber.Map{
		"category":  category.Key,
		"label":     category.Label,
		"items":     h.marketNews(ctx, category.Key, limit),
		"generated": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) home(c *fiber.Ctx) error {
	ctx := requestContext(c)
	var stocks []models.StockMetric
	if !h.cache.GetJSON(ctx, "home:rankings", &stocks) {
		var err error
		stocks, err = h.repo.Rankings(ctx, 8)
		if err != nil {
			return err
		}
		h.cache.SetJSON(ctx, "home:rankings", stocks, 5*time.Minute)
	}
	newsItems := h.marketNews(ctx, "all", 8)
	return h.render(c, "home.html", ViewData{
		Title:     "오늘의 시장 이슈",
		Active:    "home",
		Stocks:    stocks,
		News:      newsItems,
		Generated: time.Now(),
	})
}

func (h *Handler) screener(c *fiber.Ctx) error {
	ctx := requestContext(c)
	filters := parseFilters(c)

	stocks, err := h.repo.Screener(ctx, filters, 50)
	if err != nil {
		return err
	}
	sectors, err := h.repo.Sectors(ctx)
	if err != nil {
		return err
	}

	data := ViewData{
		Title:      "종목 검색기",
		Active:     "screener",
		Stocks:     stocks,
		Sectors:    sectors,
		Filters:    filters,
		Generated:  time.Now(),
		ResultPath: "/screener",
	}
	if isHTMX(c) {
		return h.renderPartial(c, "stock_table.html", "stock_table", data)
	}
	return h.render(c, "screener.html", data)
}

func (h *Handler) rankings(c *fiber.Ctx) error {
	ctx := requestContext(c)
	var stocks []models.StockMetric
	if !h.cache.GetJSON(ctx, "rankings:top50", &stocks) {
		var err error
		stocks, err = h.repo.Rankings(ctx, 50)
		if err != nil {
			return err
		}
		h.cache.SetJSON(ctx, "rankings:top50", stocks, 5*time.Minute)
	}
	return h.render(c, "rankings.html", ViewData{
		Title:     "종합 랭킹",
		Active:    "rankings",
		Stocks:    stocks,
		Generated: time.Now(),
	})
}

func (h *Handler) sector(c *fiber.Ctx) error {
	ctx := requestContext(c)
	var strengths []models.SectorStrength
	if !h.cache.GetJSON(ctx, "sector:strength", &strengths) {
		var err error
		strengths, err = h.repo.SectorStrength(ctx)
		if err != nil {
			return err
		}
		h.cache.SetJSON(ctx, "sector:strength", strengths, 5*time.Minute)
	}
	return h.render(c, "sector.html", ViewData{
		Title:     "섹터 분석",
		Active:    "sector",
		Strengths: strengths,
		Generated: time.Now(),
	})
}

func (h *Handler) news(c *fiber.Ctx) error {
	ctx := requestContext(c)
	category := h.newsCategory(c.Query("category", "all"))
	return h.render(c, "news.html", ViewData{
		Title:              "시장 이슈",
		Active:             "news",
		News:               h.marketNews(ctx, category.Key, 36),
		NewsCategories:     h.newsCategories,
		ActiveNewsCategory: category.Key,
		Generated:          time.Now(),
	})
}

func (h *Handler) stock(c *fiber.Ctx) error {
	ctx := requestContext(c)
	stock, prices, err := h.repo.FindStock(ctx, strings.ToUpper(c.Params("code")))
	if err != nil {
		return h.render(c, "stock.html", ViewData{
			Title:  "종목 상세",
			Active: "stock",
			Error:  "종목을 찾을 수 없습니다.",
		})
	}
	return h.render(c, "stock.html", ViewData{
		Title:     stock.Name,
		Active:    "stock",
		Stock:     stock,
		Prices:    prices,
		News:      h.stockNews(ctx, stock.Name, stock.Code, 8),
		Generated: time.Now(),
	})
}

func (h *Handler) marketNews(ctx context.Context, categoryKey string, limit int) []models.NewsItem {
	category := h.newsCategory(categoryKey)
	var items []models.NewsItem
	cacheKey := "news:market:" + category.Key
	if h.cache.GetJSON(ctx, cacheKey, &items) && len(items) > 0 {
		if limit > 0 && len(items) > limit {
			return items[:limit]
		}
		return items
	}
	items, err := h.newsClient.FetchMarketNews(ctx, category.Queries, 48)
	if err != nil {
		return nil
	}
	h.cache.SetJSON(ctx, cacheKey, items, 10*time.Minute)
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (h *Handler) newsCategory(key string) models.NewsCategory {
	key = strings.TrimSpace(strings.ToLower(key))
	for _, category := range h.newsCategories {
		if category.Key == key {
			return category
		}
	}
	return h.newsCategories[0]
}

func defaultNewsCategories(baseQueries []string) []models.NewsCategory {
	return []models.NewsCategory{
		{
			Key:         "all",
			Label:       "전체",
			Description: "시장, 수급, 섹터, 실적 이슈를 한 번에 봅니다.",
			Queries:     baseQueries,
		},
		{
			Key:         "market",
			Label:       "시장",
			Description: "코스피, 코스닥, 금리, 환율 등 지수와 거시 환경 중심입니다.",
			Queries: []string{
				"코스피 코스닥 증시",
				"국내 주식 시장 이슈",
				"금리 환율 국내 증시",
			},
		},
		{
			Key:         "flow",
			Label:       "수급",
			Description: "기관, 외국인, 공매도, 거래대금 흐름을 모읍니다.",
			Queries: []string{
				"기관 외국인 순매수 국내 주식",
				"공매도 수급 국내 증시",
				"거래대금 급증 국내 주식",
			},
		},
		{
			Key:         "sector",
			Label:       "섹터",
			Description: "반도체, 자동차, 방산, 전력, 바이오, 조선 등 주도 업종 이슈입니다.",
			Queries: []string{
				"반도체 자동차 방산 전력 바이오 조선 주식",
				"AI 반도체 전력기기 조선 방산 국내 주식",
				"바이오 배터리 로봇 국내 주식",
			},
		},
		{
			Key:         "earnings",
			Label:       "실적",
			Description: "상장사 실적 발표, 전망, 어닝 서프라이즈를 추적합니다.",
			Queries: []string{
				"국내 상장사 실적 전망",
				"어닝 서프라이즈 국내 주식",
				"영업이익 증가 국내 상장사",
			},
		},
	}
}

func (h *Handler) stockNews(ctx context.Context, name string, code string, limit int) []models.NewsItem {
	key := "news:stock:" + code
	var items []models.NewsItem
	if h.cache.GetJSON(ctx, key, &items) && len(items) > 0 {
		if limit > 0 && len(items) > limit {
			return items[:limit]
		}
		return items
	}
	items, err := h.newsClient.FetchStockNews(ctx, name, code, 16)
	if err != nil {
		return nil
	}
	h.cache.SetJSON(ctx, key, items, 10*time.Minute)
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (h *Handler) render(c *fiber.Ctx, page string, data ViewData) error {
	if data.MarketDataStatus == "" {
		data.MarketDataStatus = h.marketDataConfig.Status
		if status, err := h.repo.PriceStatus(requestContext(c)); err == nil {
			data.PriceStatus = status
			data.MarketDataStatus = withPriceStatus(h.marketDataConfig.Status, status)
		}
	}
	tpl, err := template.New("layout.html").Funcs(h.funcs).ParseFiles(
		"templates/layout.html",
		"templates/"+page,
		"templates/stock_table.html",
	)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		return err
	}
	c.Type("html", "utf-8")
	return c.Send(buf.Bytes())
}

func withPriceStatus(base string, status models.PriceStatus) string {
	if status.StockCount == 0 || status.LatestDate.IsZero() || status.LatestDate.Year() <= 1 {
		return base + " 현재 DB에 가격 데이터가 없습니다."
	}
	return fmt.Sprintf("%s 현재 DB 기준일은 %s, 저장 종목은 %s개입니다.", base, status.LatestDate.Format("2006-01-02"), number(status.StockCount))
}

func (h *Handler) renderPartial(c *fiber.Ctx, file string, name string, data ViewData) error {
	tpl, err := template.New(file).Funcs(h.funcs).ParseFiles("templates/" + file)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	c.Type("html", "utf-8")
	return c.Send(buf.Bytes())
}

func parseFilters(c *fiber.Ctx) models.Filters {
	return models.Filters{
		Query:              strings.TrimSpace(c.Query("q")),
		Sector:             c.Query("sector"),
		MinMarketCap:       parseFloat(c.Query("min_market_cap")),
		MaxMarketCap:       parseFloat(c.Query("max_market_cap")),
		MinRevenueGrowth:   parseFloat(c.Query("min_revenue_growth")),
		MinOperatingGrowth: parseFloat(c.Query("min_operating_growth")),
		MaxPER:             parseFloat(c.Query("max_per")),
		MaxPBR:             parseFloat(c.Query("max_pbr")),
		NearHighOnly:       parseBool(c.Query("near_high")),
		InstitutionOnly:    parseBool(c.Query("institution_only")),
	}
}

func parseFloat(value string) float64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseBool(value string) bool {
	return value == "on" || value == "true" || value == "1"
}

func isHTMX(c *fiber.Ctx) bool {
	return c.Get("HX-Request") == "true"
}

func requestContext(c *fiber.Ctx) context.Context {
	ctx := c.UserContext()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func krw(value float64) string {
	return fmt.Sprintf("%s원", number(value))
}

func krwShort(value float64) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1_0000_0000_0000:
		return fmt.Sprintf("%.1f조", value/1_0000_0000_0000)
	case abs >= 1_0000_0000:
		return fmt.Sprintf("%.1f억", value/1_0000_0000)
	case abs >= 1_0000:
		return fmt.Sprintf("%.1f만", value/1_0000)
	default:
		return number(value)
	}
}

func number(value any) string {
	var rounded int64
	switch v := value.(type) {
	case int:
		rounded = int64(v)
	case int64:
		rounded = v
	case float64:
		rounded = int64(math.Round(v))
	case float32:
		rounded = int64(math.Round(float64(v)))
	default:
		return fmt.Sprint(value)
	}
	sign := ""
	if rounded < 0 {
		sign = "-"
		rounded = -rounded
	}
	raw := strconv.FormatInt(rounded, 10)
	if len(raw) <= 3 {
		return sign + raw
	}
	var out []byte
	prefix := len(raw) % 3
	if prefix == 0 {
		prefix = 3
	}
	out = append(out, raw[:prefix]...)
	for i := prefix; i < len(raw); i += 3 {
		out = append(out, ',')
		out = append(out, raw[i:i+3]...)
	}
	return sign + string(out)
}

func percent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func timeAgo(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	delta := time.Since(value)
	switch {
	case delta < time.Minute:
		return "방금 전"
	case delta < time.Hour:
		return fmt.Sprintf("%d분 전", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%d시간 전", int(delta.Hours()))
	default:
		return value.Format("01-02 15:04")
	}
}

func scoreWidth(score float64) string {
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return fmt.Sprintf("%.0f%%", score)
}

func scoreTone(score float64) string {
	switch {
	case score >= 75:
		return "bg-emerald-500"
	case score >= 55:
		return "bg-sky-500"
	case score >= 35:
		return "bg-amber-500"
	default:
		return "bg-slate-400"
	}
}
