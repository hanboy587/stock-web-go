package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stockhunter/internal/models"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Sectors(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `select distinct sector from stocks order by sector`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sectors []string
	for rows.Next() {
		var sector string
		if err := rows.Scan(&sector); err != nil {
			return nil, err
		}
		sectors = append(sectors, sector)
	}
	return sectors, rows.Err()
}

func (r *Repository) Screener(ctx context.Context, filters models.Filters, limit int) ([]models.StockMetric, error) {
	rows, err := r.db.Query(ctx, metricsQuery+`
select *
from scored
where ($1 = '' or sector = $1)
  and ($2::numeric = 0 or market_cap >= $2)
  and ($3::numeric = 0 or market_cap <= $3)
  and ($4::numeric = 0 or revenue_growth >= $4)
  and ($5::numeric = 0 or operating_profit_growth >= $5)
  and ($6::numeric = 0 or per <= $6)
  and ($7::numeric = 0 or pbr <= $7)
  and ($8::boolean = false or distance_from_high >= -10)
  and ($9::boolean = false or institution_net_20 > 0)
order by total_score desc, revenue_growth desc
limit $10`, filters.Sector, filters.MinMarketCap, filters.MaxMarketCap, filters.MinRevenueGrowth,
		filters.MinOperatingGrowth, filters.MaxPER, filters.MaxPBR, filters.NearHighOnly, filters.InstitutionOnly, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetrics(rows)
}

func (r *Repository) Rankings(ctx context.Context, limit int) ([]models.StockMetric, error) {
	rows, err := r.db.Query(ctx, metricsQuery+`
select *
from scored
order by total_score desc, growth_score desc, flow_score desc
limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetrics(rows)
}

func (r *Repository) FindStock(ctx context.Context, code string) (models.StockMetric, []models.PricePoint, error) {
	row := r.db.QueryRow(ctx, metricsQuery+`
select *
from scored
where code = $1`, code)

	metric, err := scanMetric(row)
	if err != nil {
		return models.StockMetric{}, nil, err
	}

	priceRows, err := r.db.Query(ctx, `
select date, close, volume
from prices
where stock_code = $1
order by date desc
limit 30`, code)
	if err != nil {
		return models.StockMetric{}, nil, err
	}
	defer priceRows.Close()

	var prices []models.PricePoint
	for priceRows.Next() {
		var point models.PricePoint
		if err := priceRows.Scan(&point.Date, &point.Close, &point.Volume); err != nil {
			return models.StockMetric{}, nil, err
		}
		prices = append(prices, point)
	}
	return metric, prices, priceRows.Err()
}

func (r *Repository) SectorStrength(ctx context.Context) ([]models.SectorStrength, error) {
	rows, err := r.db.Query(ctx, `
with price_marks as (
	select
		s.sector,
		p.stock_code,
		max(p.close) filter (where p.date = current_date) as close_now,
		max(p.close) filter (where p.date <= current_date - interval '7 days') as close_1w,
		max(p.close) filter (where p.date <= current_date - interval '30 days') as close_1m,
		max(p.close) filter (where p.date <= current_date - interval '90 days') as close_3m
	from stocks s
	join prices p on p.stock_code = s.code
	group by s.sector, p.stock_code
),
flow as (
	select s.sector, sum(i.institution_net + i.foreign_net) as flow_20
	from stocks s
	join investor_flow i on i.stock_code = s.code
	where i.date >= current_date - interval '20 days'
	group by s.sector
)
select
	pm.sector,
	count(*)::int as stock_count,
	coalesce(avg((close_now - close_1w) / nullif(close_1w, 0) * 100), 0) as return_1w,
	coalesce(avg((close_now - close_1m) / nullif(close_1m, 0) * 100), 0) as return_1m,
	coalesce(avg((close_now - close_3m) / nullif(close_3m, 0) * 100), 0) as return_3m,
	coalesce(max(f.flow_20), 0) as flow_20
from price_marks pm
left join flow f on f.sector = pm.sector
group by pm.sector
order by return_1m desc, flow_20 desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sectors []models.SectorStrength
	for rows.Next() {
		var item models.SectorStrength
		if err := rows.Scan(&item.Sector, &item.StockCount, &item.Return1Week, &item.Return1Month, &item.Return3Month, &item.Flow20); err != nil {
			return nil, err
		}
		sectors = append(sectors, item)
	}
	return sectors, rows.Err()
}

func scanMetrics(rows pgx.Rows) ([]models.StockMetric, error) {
	var metrics []models.StockMetric
	for rows.Next() {
		metric, err := scanMetric(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	return metrics, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanMetric(row scanner) (models.StockMetric, error) {
	var m models.StockMetric
	err := row.Scan(
		&m.Code,
		&m.Name,
		&m.Market,
		&m.Sector,
		&m.CurrentPrice,
		&m.MarketCap,
		&m.PER,
		&m.PBR,
		&m.RevenueGrowth,
		&m.OperatingProfitGrowth,
		&m.NetIncome,
		&m.High52Week,
		&m.DistanceFromHigh,
		&m.VolumeRatio,
		&m.InstitutionNet20,
		&m.InstitutionNet60,
		&m.ForeignNet20,
		&m.GrowthScore,
		&m.FlowScore,
		&m.PriceScore,
		&m.StabilityScore,
		&m.TotalScore,
	)
	if err != nil {
		return models.StockMetric{}, fmt.Errorf("scan stock metric: %w", err)
	}
	return m, nil
}

const metricsQuery = `
with latest_price as (
	select distinct on (stock_code)
		stock_code,
		close as current_price,
		volume as current_volume
	from prices
	order by stock_code, date desc
),
price_stats as (
	select
		stock_code,
		max(high) as high_52w,
		avg(volume) filter (where date >= current_date - interval '20 days') as avg_volume_20,
		avg(volume) filter (where date < current_date - interval '20 days' and date >= current_date - interval '60 days') as avg_volume_prev
	from prices
	where date >= current_date - interval '365 days'
	group by stock_code
),
financial_rank as (
	select
		f.*,
		row_number() over (partition by stock_code order by year desc, quarter desc) as rn
	from financials f
),
financial_compare as (
	select
		now.stock_code,
		now.revenue,
		now.operating_profit,
		now.net_income,
		now.equity,
		coalesce((now.revenue - prev.revenue) / nullif(abs(prev.revenue), 0) * 100, 0) as revenue_growth,
		coalesce((now.operating_profit - prev.operating_profit) / nullif(abs(prev.operating_profit), 0) * 100, 0) as operating_profit_growth
	from financial_rank now
	left join financial_rank prev on prev.stock_code = now.stock_code and prev.rn = 2
	where now.rn = 1
),
flow as (
	select
		stock_code,
		coalesce(sum(institution_net) filter (where date >= current_date - interval '20 days'), 0) as institution_net_20,
		coalesce(sum(institution_net) filter (where date >= current_date - interval '60 days'), 0) as institution_net_60,
		coalesce(sum(foreign_net) filter (where date >= current_date - interval '20 days'), 0) as foreign_net_20
	from investor_flow
	group by stock_code
),
metrics as (
	select
		s.code,
		s.name,
		s.market,
		s.sector,
		lp.current_price,
		lp.current_price * s.shares_outstanding as market_cap,
		coalesce((lp.current_price * s.shares_outstanding) / nullif(fc.net_income, 0), 0) as per,
		coalesce((lp.current_price * s.shares_outstanding) / nullif(fc.equity, 0), 0) as pbr,
		fc.revenue_growth,
		fc.operating_profit_growth,
		fc.net_income,
		ps.high_52w,
		coalesce((lp.current_price - ps.high_52w) / nullif(ps.high_52w, 0) * 100, 0) as distance_from_high,
		coalesce(ps.avg_volume_20 / nullif(ps.avg_volume_prev, 0), 1) as volume_ratio,
		coalesce(fl.institution_net_20, 0) as institution_net_20,
		coalesce(fl.institution_net_60, 0) as institution_net_60,
		coalesce(fl.foreign_net_20, 0) as foreign_net_20
	from stocks s
	join latest_price lp on lp.stock_code = s.code
	join price_stats ps on ps.stock_code = s.code
	join financial_compare fc on fc.stock_code = s.code
	left join flow fl on fl.stock_code = s.code
),
scored as (
	select
		code,
		name,
		market,
		sector,
		current_price,
		market_cap,
		per,
		pbr,
		revenue_growth,
		operating_profit_growth,
		net_income,
		high_52w,
		distance_from_high,
		volume_ratio,
		institution_net_20,
		institution_net_60,
		foreign_net_20,
		least(40, greatest(0, revenue_growth * 0.45 + operating_profit_growth * 0.55)) as growth_score,
		least(30, greatest(0, (institution_net_20 + foreign_net_20) / nullif(market_cap, 0) * 100000)) as flow_score,
		least(20, greatest(0, 20 - abs(distance_from_high))) as price_score,
		case
			when net_income > 0 and operating_profit_growth > 0 and per > 0 and per <= 18 then 10
			when net_income > 0 and operating_profit_growth > 0 then 7
			when net_income > 0 then 4
			else 0
		end as stability_score,
		round((
			least(40, greatest(0, revenue_growth * 0.45 + operating_profit_growth * 0.55)) +
			least(30, greatest(0, (institution_net_20 + foreign_net_20) / nullif(market_cap, 0) * 100000)) +
			least(20, greatest(0, 20 - abs(distance_from_high))) +
			case
				when net_income > 0 and operating_profit_growth > 0 and per > 0 and per <= 18 then 10
				when net_income > 0 and operating_profit_growth > 0 then 7
				when net_income > 0 then 4
				else 0
			end
		)::numeric, 2) as total_score
	from metrics
)
`
