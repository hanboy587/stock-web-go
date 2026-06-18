# StockHunter

국내 주식의 일별 종가를 DB에 누적하고, 시장 뉴스와 종목 관련 이슈를 빠르게 모아 보여주는 웹 서비스입니다.

실시간 시세 재배포는 라이선스/약관 리스크가 있어 사용하지 않습니다. 가격 데이터는 공식 일별 종가 API만 사용하고, 실시간성은 뉴스/이슈 피드에 집중합니다.

## 구성

- Go + Fiber
- PostgreSQL
- Redis
- HTMX + Alpine.js + TailwindCSS CDN
- Docker Compose

## 실행

```powershell
docker compose up --build
```

브라우저에서 `http://localhost:8080`을 엽니다. API 키가 없으면 기본 데이터로 화면이 뜨고, 뉴스 피드는 Google News RSS를 10분 캐시로 가져옵니다.

## 가격 데이터

공식 API 키가 있으면 앱 시작 직후 최근 `DAILY_CLOSE_BACKFILL_DAYS`일을 백필하고, 평일 18:35(KST)에 최신 거래일의 일별 종가를 가져와 `prices` 테이블에 누적합니다.

우선순위:

1. KRX Open API 일별매매정보 (`KRX_AUTH_KEY`)
2. 공공데이터포털 금융위원회 주식시세정보 (`PUBLIC_DATA_SERVICE_KEY`)

```bash
KRX_AUTH_KEY=<KRX Open API key>
PUBLIC_DATA_SERVICE_KEY=<data.go.kr service key>
DAILY_CLOSE_BACKFILL_DAYS=10
NEWS_QUERIES='코스피 OR 코스닥 증시|국내 주식 시장 이슈|기관 외국인 순매수'
```

## 주요 페이지

- `/` 오늘의 시장 이슈와 종가 랭킹
- `/screener` 종가 기반 검색기
- `/rankings` 종합 랭킹
- `/sector` 섹터 분석
- `/news` 시장 이슈 피드
- `/news?category=flow` 수급 이슈 피드
- `/stock/{code}` 종목 상세
- `/status` 데이터 기준일, 수집 이력, 운영 확인 화면
- `/api/status` 데이터 소스와 가격 DB 상태 JSON
- `/api/news?category=sector&limit=24` 카테고리별 뉴스 JSON
- `/healthz` DB/Redis 헬스체크 JSON

투자 추천이 아니라 데이터 기반 발굴과 이슈 모니터링 도구입니다.
