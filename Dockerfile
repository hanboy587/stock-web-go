FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod ./

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/stockhunter ./cmd/server

FROM alpine:3.21

WORKDIR /app
RUN adduser -D -u 10001 appuser
COPY --from=build /out/stockhunter /app/stockhunter
COPY templates /app/templates
COPY static /app/static

USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/stockhunter"]
