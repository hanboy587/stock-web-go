create table if not exists stocks (
	code text primary key,
	name text not null,
	market text not null,
	sector text not null,
	shares_outstanding numeric(20, 0) not null check (shares_outstanding > 0)
);

create table if not exists prices (
	stock_code text not null references stocks(code) on delete cascade,
	date date not null,
	open numeric(14, 2) not null,
	high numeric(14, 2) not null,
	low numeric(14, 2) not null,
	close numeric(14, 2) not null,
	volume bigint not null,
	primary key (stock_code, date)
);

create table if not exists financials (
	stock_code text not null references stocks(code) on delete cascade,
	year int not null,
	quarter int not null check (quarter between 1 and 4),
	revenue numeric(20, 0) not null,
	operating_profit numeric(20, 0) not null,
	net_income numeric(20, 0) not null,
	equity numeric(20, 0) not null,
	primary key (stock_code, year, quarter)
);

create table if not exists investor_flow (
	stock_code text not null references stocks(code) on delete cascade,
	date date not null,
	foreign_net numeric(20, 0) not null,
	institution_net numeric(20, 0) not null,
	primary key (stock_code, date)
);

create index if not exists prices_stock_date_desc on prices(stock_code, date desc);
create index if not exists investor_flow_stock_date_desc on investor_flow(stock_code, date desc);
create index if not exists stocks_sector_idx on stocks(sector);

insert into stocks (code, name, market, sector, shares_outstanding) values
	('005930', '삼성전자', 'KOSPI', '반도체', 5969782550),
	('000660', 'SK하이닉스', 'KOSPI', '반도체', 728002365),
	('005380', '현대차', 'KOSPI', '자동차', 211531506),
	('012450', '한화에어로스페이스', 'KOSPI', '방산', 50630000),
	('068270', '셀트리온', 'KOSPI', '바이오', 220290520),
	('329180', 'HD현대중공업', 'KOSPI', '조선', 88773116),
	('010120', 'LS ELECTRIC', 'KOSPI', '전력', 30000000),
	('079550', 'LIG넥스원', 'KOSPI', '방산', 22000000)
on conflict (code) do nothing;

insert into prices (stock_code, date, open, high, low, close, volume)
select
	v.code,
	current_date - g.n,
	round(p.close * 0.992, 2),
	round(p.close * case when g.n = v.high_day then 1.12 else 1.021 end, 2),
	round(p.close * 0.981, 2),
	round(p.close, 2),
	(v.base_volume + (90 - g.n) * v.volume_step + (g.n % 6) * 50000)::bigint
from (
	values
		('005930', 82000::numeric, 95::numeric, 9000000::bigint, 12000::bigint, 18),
		('000660', 236000::numeric, 820::numeric, 4200000::bigint, 18000::bigint, 6),
		('005380', 286000::numeric, 310::numeric, 900000::bigint, 8000::bigint, 45),
		('012450', 422000::numeric, 1650::numeric, 650000::bigint, 7000::bigint, 4),
		('068270', 181000::numeric, -110::numeric, 560000::bigint, 5000::bigint, 70),
		('329180', 314000::numeric, 1250::numeric, 410000::bigint, 6500::bigint, 12),
		('010120', 196000::numeric, 760::numeric, 340000::bigint, 4500::bigint, 9),
		('079550', 232000::numeric, 980::numeric, 280000::bigint, 4200::bigint, 7)
) as v(code, base, trend, base_volume, volume_step, high_day)
cross join generate_series(0, 90) as g(n)
cross join lateral (
	select greatest(
		1000::numeric,
		v.base - (g.n * v.trend) +
		case
			when g.n % 11 in (0, 1, 2) then v.base * 0.015
			when g.n % 7 in (0, 1) then v.base * -0.008
			else v.base * 0.004
		end
	) as close
) p
on conflict (stock_code, date) do nothing;

insert into financials (stock_code, year, quarter, revenue, operating_profit, net_income, equity) values
	('005930', 2025, 4, 278000000000000, 26500000000000, 24000000000000, 372000000000000),
	('005930', 2026, 1, 315000000000000, 38400000000000, 33800000000000, 384000000000000),
	('000660', 2025, 4, 66000000000000, 21000000000000, 16800000000000, 82000000000000),
	('000660', 2026, 1, 82400000000000, 30200000000000, 24600000000000, 91000000000000),
	('005380', 2025, 4, 174000000000000, 15200000000000, 12800000000000, 112000000000000),
	('005380', 2026, 1, 188000000000000, 16800000000000, 13900000000000, 119000000000000),
	('012450', 2025, 4, 11800000000000, 980000000000, 680000000000, 6200000000000),
	('012450', 2026, 1, 15800000000000, 1620000000000, 1180000000000, 7400000000000),
	('068270', 2025, 4, 3520000000000, 210000000000, 115000000000, 19500000000000),
	('068270', 2026, 1, 3760000000000, 315000000000, 196000000000, 20100000000000),
	('329180', 2025, 4, 14400000000000, 810000000000, 460000000000, 7600000000000),
	('329180', 2026, 1, 18100000000000, 1320000000000, 840000000000, 8500000000000),
	('010120', 2025, 4, 4380000000000, 358000000000, 252000000000, 2450000000000),
	('010120', 2026, 1, 5820000000000, 612000000000, 442000000000, 2860000000000),
	('079550', 2025, 4, 3100000000000, 248000000000, 181000000000, 2100000000000),
	('079550', 2026, 1, 4180000000000, 436000000000, 311000000000, 2480000000000)
on conflict (stock_code, year, quarter) do nothing;

insert into investor_flow (stock_code, date, foreign_net, institution_net)
select
	v.code,
	current_date - g.n,
	round(v.foreign_bias + case when g.n % 5 in (0, 1) then abs(v.foreign_bias) * 0.35 else abs(v.foreign_bias) * -0.15 end, 0),
	round(v.inst_bias + case when g.n % 4 in (0, 1) then abs(v.inst_bias) * 0.4 else abs(v.inst_bias) * -0.12 end, 0)
from (
	values
		('005930', 42000000000::numeric, 36000000000::numeric),
		('000660', 68000000000::numeric, 84000000000::numeric),
		('005380', 12000000000::numeric, 18000000000::numeric),
		('012450', 32000000000::numeric, 52000000000::numeric),
		('068270', -9000000000::numeric, 6000000000::numeric),
		('329180', 18000000000::numeric, 27000000000::numeric),
		('010120', 22000000000::numeric, 33000000000::numeric),
		('079550', 14000000000::numeric, 26000000000::numeric)
) as v(code, foreign_bias, inst_bias)
cross join generate_series(0, 70) as g(n)
on conflict (stock_code, date) do nothing;
