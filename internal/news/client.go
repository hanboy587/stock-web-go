package news

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"stockhunter/internal/models"
)

type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{
		http: &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *Client) FetchMarketNews(ctx context.Context, queries []string, limit int) ([]models.NewsItem, error) {
	seen := map[string]bool{}
	var items []models.NewsItem
	for _, query := range queries {
		queryItems, err := c.fetchGoogleNewsRSS(ctx, query)
		if err != nil {
			continue
		}
		for _, item := range queryItems {
			key := normalizedNewsKey(item)
			if seen[key] {
				continue
			}
			seen[key] = true
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (c *Client) FetchStockNews(ctx context.Context, name string, code string, limit int) ([]models.NewsItem, error) {
	query := strings.TrimSpace(name + " " + code + " 주식 실적 수급")
	items, err := c.fetchGoogleNewsRSS(ctx, query)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	deduped := make([]models.NewsItem, 0, len(items))
	for _, item := range items {
		key := normalizedNewsKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, item)
	}
	items = deduped
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (c *Client) fetchGoogleNewsRSS(ctx context.Context, query string) ([]models.NewsItem, error) {
	endpoint := "https://news.google.com/rss/search?q=" + url.QueryEscape(query) + "&hl=ko&gl=KR&ceid=KR:ko"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "StockHunter/0.1 (+https://stock.168.107.12.17.sslip.io)")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var feed rssFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	items := make([]models.NewsItem, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		title := cleanTitle(item.Title, item.Source.Name)
		link := strings.TrimSpace(item.Link)
		if title == "" || link == "" {
			continue
		}
		published, _ := time.Parse(time.RFC1123Z, item.PubDate)
		if published.IsZero() {
			published, _ = time.Parse(time.RFC1123, item.PubDate)
		}
		source := strings.TrimSpace(item.Source.Name)
		if source == "" {
			source = "Google News"
		}
		items = append(items, models.NewsItem{
			Title:       title,
			Link:        link,
			Source:      source,
			PublishedAt: published,
			Query:       query,
		})
	}
	return items, nil
}

func cleanTitle(title string, source string) string {
	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	source = strings.TrimSpace(source)
	if source != "" {
		title = strings.TrimSuffix(title, " - "+source)
	}
	return strings.TrimSpace(title)
}

func normalizedNewsKey(item models.NewsItem) string {
	title := strings.ToLower(strings.Join(strings.Fields(item.Title), " "))
	title = strings.TrimSpace(title)
	if title == "" {
		return strings.ToLower(strings.TrimSpace(item.Link))
	}
	return title
}

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string    `xml:"title"`
	Link    string    `xml:"link"`
	PubDate string    `xml:"pubDate"`
	Source  rssSource `xml:"source"`
}

type rssSource struct {
	Name string `xml:",chardata"`
}
