package domain

import "time"

type Tick struct {
	Symbol string
	Price  float64
	Time   time.Time
}

type Signal struct {
	Rule      Rule
	Symbol    string
	Price     float64
	Move      float64
	Window    time.Duration
	DailyNo   int
	Triggered time.Time
}
