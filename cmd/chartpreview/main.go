package main

import (
	"log"
	"math"
	"os"
	"time"

	"pumpscreener/src/domain"
	"pumpscreener/src/infrastructure/chart"
)

func main() {
	now := time.Now().UTC().Truncate(time.Minute)
	candles := make([]chart.Candle, 0, 30)
	price := 85.8

	for i := 0; i < 30; i++ {
		start := now.Add(time.Duration(i-29) * time.Minute)
		wave := math.Sin(float64(i)*0.7) * 0.28
		trend := float64(i) * 0.018
		open := price
		close := 85.4 + trend + wave
		high := math.Max(open, close) + 0.18 + math.Sin(float64(i))*0.05
		low := math.Min(open, close) - 0.16 - math.Cos(float64(i))*0.04

		if i == 19 {
			close += 0.9
			high = close + 0.35
		}
		if i == 20 {
			open = close + 0.2
			close -= 0.45
			high = open + 0.2
		}
		if i == 28 {
			close -= 0.55
			low = close - 0.25
		}

		candles = append(candles, chart.Candle{
			Start: start,
			Open:  open,
			High:  high,
			Low:   low,
			Close: close,
		})
		price = close
	}

	signal := chart.Signal{
		DomainSignal: domain.Signal{
			Rule: domain.Rule{
				ID:        1,
				Direction: domain.DirectionUp,
				Percent:   3,
				Interval:  30 * time.Minute,
				Cooldown:  3 * time.Minute,
				Mode:      domain.RuleModeHold,
				Hold:      3 * time.Minute,
				Enabled:   true,
			},
			Symbol:    "BTCUSDT",
			Price:     candles[len(candles)-1].Close,
			Move:      3.24,
			Window:    30 * time.Minute,
			DailyNo:   2,
			Triggered: candles[len(candles)-1].Start,
		},
		Candles: candles,
	}

	imageBytes, err := chart.RenderSignal(signal)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll("charts", 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("charts/sample_signal.png", imageBytes, 0644); err != nil {
		log.Fatal(err)
	}

	log.Printf("sample chart saved: charts/sample_signal.png")
}
