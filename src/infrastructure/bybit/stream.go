package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"pumpscreener/src/domain"
)

type Stream struct {
	wsURL string
	clock Clock
}

func NewStream(wsURL string, clock Clock) *Stream {
	return &Stream{wsURL: wsURL, clock: clock}
}

func (s *Stream) Run(ctx context.Context, symbols []string, ticks chan<- domain.Tick) error {
	if len(symbols) == 0 {
		return fmt.Errorf("no symbols to subscribe")
	}

	backoff := time.Second

	for {
		log.Printf("bybit websocket connecting: url=%s symbols=%d", s.wsURL, len(symbols))
		err := s.connectOnce(ctx, symbols, ticks)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("bybit websocket reconnecting after error: %v", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (s *Stream) connectOnce(ctx context.Context, symbols []string, ticks chan<- domain.Tick) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, s.wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("bybit websocket connected")

	chunks := chunks(symbols, 50)
	for index, chunk := range chunks {
		if err := conn.WriteJSON(subscribeMessage(chunk)); err != nil {
			return err
		}
		log.Printf("bybit subscribed chunk %d/%d symbols=%d", index+1, len(chunks), len(chunk))
	}
	log.Printf("bybit subscriptions completed: chunks=%d symbols=%d", len(chunks), len(symbols))

	for {
		var message tickerMessage
		if err := conn.ReadJSON(&message); err != nil {
			return err
		}
		if message.Topic == "" || len(message.Data) == 0 {
			continue
		}

		now := s.clock.Now()
		for _, item := range message.Items() {
			price, err := strconv.ParseFloat(item.LastPrice, 64)
			if err != nil || price <= 0 {
				continue
			}
			tickTime := now
			if item.Timestamp > 0 {
				tickTime = time.UnixMilli(item.Timestamp).UTC()
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case ticks <- domain.Tick{Symbol: item.Symbol, Price: price, Time: tickTime}:
			default:
				log.Printf("tick channel full, dropping %s", item.Symbol)
			}
		}
	}
}

func subscribeMessage(symbols []string) map[string]any {
	args := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		args = append(args, "tickers."+symbol)
	}

	return map[string]any{
		"op":   "subscribe",
		"args": args,
	}
}

func chunks(values []string, size int) [][]string {
	if size <= 0 || len(values) == 0 {
		return nil
	}

	result := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		result = append(result, values[start:end])
	}
	return result
}

type tickerMessage struct {
	Topic string
	Data  []tickerData
}

func (m *tickerMessage) UnmarshalJSON(data []byte) error {
	var raw struct {
		Topic string          `json:"topic"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	m.Topic = raw.Topic
	if len(raw.Data) == 0 || string(raw.Data) == "null" {
		return nil
	}

	var many []tickerData
	if err := json.Unmarshal(raw.Data, &many); err == nil {
		m.Data = many
		return nil
	}

	var one tickerData
	if err := json.Unmarshal(raw.Data, &one); err != nil {
		return nil
	}
	m.Data = []tickerData{one}
	return nil
}

type tickerData struct {
	Symbol    string `json:"symbol"`
	LastPrice string `json:"lastPrice"`
	Timestamp int64  `json:"timestamp"`
}

func (m tickerMessage) Items() []tickerData {
	return m.Data
}
