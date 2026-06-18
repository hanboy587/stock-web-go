package models

import "time"

type Filters struct {
	Query              string
	Sector             string
	MinMarketCap       float64
	MaxMarketCap       float64
	MinRevenueGrowth   float64
	MinOperatingGrowth float64
	MaxPER             float64
	MaxPBR             float64
	NearHighOnly       bool
	InstitutionOnly    bool
}

type StockMetric struct {
	Code                  string
	Name                  string
	Market                string
	Sector                string
	CurrentPrice          float64
	MarketCap             float64
	PER                   float64
	PBR                   float64
	RevenueGrowth         float64
	OperatingProfitGrowth float64
	NetIncome             float64
	High52Week            float64
	DistanceFromHigh      float64
	VolumeRatio           float64
	InstitutionNet20      float64
	InstitutionNet60      float64
	ForeignNet20          float64
	GrowthScore           float64
	FlowScore             float64
	PriceScore            float64
	StabilityScore        float64
	TotalScore            float64
}

type SectorStrength struct {
	Sector       string
	StockCount   int
	Return1Week  float64
	Return1Month float64
	Return3Month float64
	Flow20       float64
}

type PricePoint struct {
	Date   time.Time
	Close  float64
	Volume int64
}

type PriceStatus struct {
	LatestDate time.Time
	StockCount int
	PriceCount int
}

type DataImportRun struct {
	Provider    string
	Mode        string
	Status      string
	StartedAt   time.Time
	FinishedAt  time.Time
	TargetStart time.Time
	TargetEnd   time.Time
	Rows        int
	Message     string
}

type NewsItem struct {
	Title       string
	Link        string
	Source      string
	PublishedAt time.Time
	Query       string
}

type NewsCategory struct {
	Key         string
	Label       string
	Description string
	Queries     []string
}

type DailyPrice struct {
	Code         string
	Name         string
	Market       string
	Date         time.Time
	Open         float64
	High         float64
	Low          float64
	Close        float64
	Volume       int64
	ListedShares float64
	MarketCap    float64
}
