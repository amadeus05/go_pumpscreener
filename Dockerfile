FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pumpscreener .

FROM alpine:3.20

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/pumpscreener /app/pumpscreener

ENV PORT=8000
ENV DATABASE_PATH=/app/data/pumpscreener.json

RUN mkdir -p /app/data && chown -R app:app /app

USER app

EXPOSE 8000

CMD ["/app/pumpscreener"]
