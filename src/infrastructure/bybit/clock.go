package bybit

import "time"

type Clock struct {
	offset time.Duration
}

func NewClock(exchangeNow, localNow time.Time) Clock {
	return Clock{offset: exchangeNow.Sub(localNow)}
}

func (c Clock) Now() time.Time {
	return time.Now().UTC().Add(c.offset)
}

func (c Clock) Offset() time.Duration {
	return c.offset
}
