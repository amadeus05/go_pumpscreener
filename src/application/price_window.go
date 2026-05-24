package application

import (
	"time"

	"pumpscreener/src/domain"
)

type pricePoint struct {
	price float64
	at    time.Time
}

type priceBucket struct {
	start time.Time
	open  float64
	high  float64
	low   float64
	close float64
}

type monotonicQueue struct {
	items []pricePoint
	less  func(a, b float64) bool
}

func newMinQueue() monotonicQueue {
	return monotonicQueue{less: func(a, b float64) bool { return a <= b }}
}

func newMaxQueue() monotonicQueue {
	return monotonicQueue{less: func(a, b float64) bool { return a >= b }}
}

func (q *monotonicQueue) Push(point pricePoint) {
	for len(q.items) > 0 && q.less(point.price, q.items[len(q.items)-1].price) {
		q.items = q.items[:len(q.items)-1]
	}
	q.items = append(q.items, point)
}

func (q *monotonicQueue) Trim(cutoff time.Time) {
	index := 0
	for index < len(q.items) && q.items[index].at.Before(cutoff) {
		index++
	}
	if index > 0 {
		copy(q.items, q.items[index:])
		q.items = q.items[:len(q.items)-index]
	}
}

func (q *monotonicQueue) Value() float64 {
	if len(q.items) == 0 {
		return 0
	}

	return q.items[0].price
}

type symbolWindow struct {
	buckets        []priceBucket
	intervals      map[time.Duration]*intervalWindow
	bucketInterval time.Duration
}

func newSymbolWindow(bucketInterval time.Duration) *symbolWindow {
	return &symbolWindow{
		intervals:      make(map[time.Duration]*intervalWindow),
		bucketInterval: bucketInterval,
	}
}

type intervalWindow struct {
	mins  monotonicQueue
	maxes monotonicQueue
}

func newIntervalWindow() *intervalWindow {
	return &intervalWindow{
		mins:  newMinQueue(),
		maxes: newMaxQueue(),
	}
}

func (w *symbolWindow) SetIntervals(intervals []time.Duration) {
	keep := make(map[time.Duration]struct{}, len(intervals))
	for _, interval := range intervals {
		if interval <= 0 {
			continue
		}
		keep[interval] = struct{}{}
		if _, exists := w.intervals[interval]; exists {
			continue
		}

		window := newIntervalWindow()
		for _, bucket := range w.buckets {
			window.mins.Push(pricePoint{price: bucket.low, at: bucket.start})
			window.maxes.Push(pricePoint{price: bucket.high, at: bucket.start})
		}
		w.intervals[interval] = window
	}

	for interval := range w.intervals {
		if _, ok := keep[interval]; !ok {
			delete(w.intervals, interval)
		}
	}
}

func (w *symbolWindow) Add(tick domain.Tick, maxWindow time.Duration, maxPoints int) {
	bucket, isNewBucket, changedExtremes := w.addToBucket(tick)
	if changedExtremes {
		w.rebuildQueues()
	} else if isNewBucket {
		for _, window := range w.intervals {
			window.mins.Push(pricePoint{price: bucket.low, at: bucket.start})
			window.maxes.Push(pricePoint{price: bucket.high, at: bucket.start})
		}
	}

	cutoff := tick.Time.Add(-maxWindow)
	w.trim(cutoff, maxPoints, tick.Time)
}

func (w *symbolWindow) trim(cutoff time.Time, maxPoints int, now time.Time) {
	index := 0
	for index < len(w.buckets) && w.buckets[index].start.Before(cutoff) {
		index++
	}
	if maxPoints > 0 && len(w.buckets)-index > maxPoints {
		index = len(w.buckets) - maxPoints
	}
	if index > 0 {
		copy(w.buckets, w.buckets[index:])
		w.buckets = w.buckets[:len(w.buckets)-index]
	}

	if len(w.buckets) == 0 {
		for _, window := range w.intervals {
			window.mins.items = window.mins.items[:0]
			window.maxes.items = window.maxes.items[:0]
		}
		return
	}

	for interval, window := range w.intervals {
		windowCutoff := now.Add(-interval)
		if w.buckets[0].start.After(windowCutoff) {
			windowCutoff = w.buckets[0].start
		}
		window.mins.Trim(windowCutoff)
		window.maxes.Trim(windowCutoff)
	}
}

func (w *symbolWindow) addToBucket(tick domain.Tick) (priceBucket, bool, bool) {
	start := tick.Time.Truncate(w.bucketInterval)
	lastIndex := len(w.buckets) - 1
	if lastIndex >= 0 && w.buckets[lastIndex].start.Equal(start) {
		bucket := &w.buckets[lastIndex]
		changedExtremes := false
		if tick.Price > bucket.high {
			bucket.high = tick.Price
			changedExtremes = true
		}
		if tick.Price < bucket.low {
			bucket.low = tick.Price
			changedExtremes = true
		}
		bucket.close = tick.Price
		return *bucket, false, changedExtremes
	}

	bucket := priceBucket{
		start: start,
		open:  tick.Price,
		high:  tick.Price,
		low:   tick.Price,
		close: tick.Price,
	}
	w.buckets = append(w.buckets, bucket)
	return bucket, true, false
}

func (w *symbolWindow) rebuildQueues() {
	for _, window := range w.intervals {
		window.mins.items = window.mins.items[:0]
		window.maxes.items = window.maxes.items[:0]
		for _, bucket := range w.buckets {
			window.mins.Push(pricePoint{price: bucket.low, at: bucket.start})
			window.maxes.Push(pricePoint{price: bucket.high, at: bucket.start})
		}
	}
}

func (w *symbolWindow) Stats(interval time.Duration) domain.WindowStats {
	window := w.intervals[interval]
	if window == nil {
		return domain.WindowStats{}
	}

	return domain.WindowStats{
		MinPrice: window.mins.Value(),
		MaxPrice: window.maxes.Value(),
	}
}

type PriceWindows struct {
	bySymbol       map[string]*symbolWindow
	maxWindow      time.Duration
	maxPoints      int
	intervals      []time.Duration
	bucketInterval time.Duration
}

func NewPriceWindows(maxWindow time.Duration, maxPoints int, bucketInterval time.Duration) *PriceWindows {
	if bucketInterval <= 0 {
		bucketInterval = time.Second
	}
	return &PriceWindows{
		bySymbol:       make(map[string]*symbolWindow),
		maxWindow:      maxWindow,
		maxPoints:      maxPoints,
		bucketInterval: bucketInterval,
	}
}

func (w *PriceWindows) SetIntervals(intervals []time.Duration) {
	w.intervals = intervals
	w.maxWindow = longestDuration(intervals)
	for _, window := range w.bySymbol {
		window.SetIntervals(intervals)
	}
}

func (w *PriceWindows) Add(tick domain.Tick) {
	window := w.bySymbol[tick.Symbol]
	if window == nil {
		window = newSymbolWindow(w.bucketInterval)
		window.SetIntervals(w.intervals)
		w.bySymbol[tick.Symbol] = window
	}

	window.Add(tick, w.maxWindow, w.maxPoints)
}

func (w *PriceWindows) Stats(symbol string, interval time.Duration) domain.WindowStats {
	window := w.bySymbol[symbol]
	if window == nil {
		return domain.WindowStats{}
	}

	return window.Stats(interval)
}

func (w *PriceWindows) Delete(symbol string) {
	delete(w.bySymbol, symbol)
}

func (w *PriceWindows) Symbols() int {
	return len(w.bySymbol)
}

func longestDuration(values []time.Duration) time.Duration {
	var longest time.Duration
	for _, value := range values {
		if value > longest {
			longest = value
		}
	}
	return longest
}
