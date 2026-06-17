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
	"stockhunter/internal/repository"
)

type Handler struct {
	repo  *repository.Repository
	cache *cache.Client
	funcs template.FuncMap
}

type ViewData struct {
	Title      string
	Active     string
	Stocks     []models.StockMetric
	Stock      models.StockMetric
	Prices     []models.PricePoint
	Sectors    []string
	Strengths  []models.SectorStrength
	Filters    models.Filters
	Generated  time.Time
	Error      string
	ResultPath string
}

func Register(app *fiber.App, repo *repository.Repository, cacheClient *cache.Client) {
	h := &Handler{
		repo:  repo,
		cache: cacheClient,
		funcs: template.FuncMap{
			"krw":        krw,
			"krwShort":   krwShort,
			"number":     number,
			"percent":    percent,
			"scoreWidth": scoreWidth,
			"scoreTone":  scoreTone,
			"date":       func(t time.Time) string { return t.Format("2006-01-02") },
		},
	}

	app.Get("/", h.home)
	app.Get("/screener", h.screener)
	app.Get("/rankings", h.rankings)
	app.Get("/sector", h.sector)
	app.Get("/stock/:code", h.stock)
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
	return h.render(c, "home.html", ViewData{
		Title:     "오늘의 발굴 종목",
		Active:    "home",
		Stocks:    stocks,
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
		Generated: time.Now(),
	})
}

func (h *Handler) render(c *fiber.Ctx, page string, data ViewData) error {
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

func number(value float64) string {
	rounded := int64(math.Round(value))
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
