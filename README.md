# StockHunter

국내 주식의 일별 종가, 실적 성장, 수급 지표, 섹터 강도를 DB 기반으로 계산하고 시장 뉴스/이슈를 보기 좋게 모으는 MVP 웹 서비스입니다.

## 구성

- Go 1.25 + Fiber
- PostgreSQL
- Redis
- HTMX + Alpine.js + TailwindCSS CDN
- Docker Compose + Nginx

## 실행

```powershell
docker compose up --build
```

브라우저에서 `http://localhost:8080`을 엽니다. 기본 종가 데이터는 샘플이며, 뉴스 피드는 Google News RSS를 10분 캐시로 가져옵니다.

PostgreSQL 초기 스키마와 샘플 데이터는 `migrations/001_init.sql`에서 자동 로드됩니다. 이미 생성된 볼륨이 있으면 초기 SQL이 다시 실행되지 않으므로 데이터를 초기화할 때는 아래 명령을 사용합니다.

```powershell
docker compose down -v
docker compose up --build
```

## 운영 배포

기존 Caddy 리버스 프록시가 있는 서버에서는 운영용 compose 파일을 사용합니다.

```bash
POSTGRES_PASSWORD=<secret> docker compose -f docker-compose.prod.yml up -d --build
```

Caddy에서 `stock.168.107.12.17.sslip.io`를 `stockhunter:8080`으로 프록시하면 외부 접속이 가능합니다.

일별 종가 자동 업데이트는 공공데이터포털 금융위원회 주식시세정보 서비스 키가 있을 때 활성화됩니다.

```bash
PUBLIC_DATA_SERVICE_KEY=<data.go.kr service key>
NEWS_QUERIES='코스피 OR 코스닥 증시|국내 주식 시장 이슈|기관 외국인 순매수'
```

## 주요 페이지

- `/` 오늘의 발굴 종목
- `/screener` 종목 검색기
- `/rankings` 종합 랭킹
- `/sector` 섹터 분석
- `/news` 시장 이슈 피드
- `/stock/{code}` 종목 상세

## 점수 계산

- 성장성: 매출 성장률과 영업이익 성장률 기반, 최대 40점
- 수급: 최근 20일 기관/외국인 순매수와 시가총액 비율 기반, 최대 30점
- 가격 위치: 최근 일별 종가의 52주 고점 근접도 기반, 최대 20점
- 실적 안정성: 순이익, 영업이익 성장, PER 조건 기반, 최대 10점

투자 추천이 아니라 데이터 기반 발굴과 이슈 모니터링 도구로 설계했습니다. 실시간 시세 재배포는 라이선스 이슈가 있으므로 공개 화면은 일별 종가와 뉴스/이슈 중심으로 운영합니다.
