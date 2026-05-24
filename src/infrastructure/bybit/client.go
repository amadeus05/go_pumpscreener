package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) LinearUSDTPerpetuals(ctx context.Context) ([]string, error) {
	values := url.Values{}
	values.Set("category", "linear")
	values.Set("limit", "1000")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v5/market/instruments-info?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bybit instruments returned status %d", resp.StatusCode)
	}

	var payload instrumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.RetCode != 0 {
		return nil, fmt.Errorf("bybit instruments error %d: %s", payload.RetCode, payload.RetMsg)
	}

	symbols := make([]string, 0, len(payload.Result.List))
	for _, item := range payload.Result.List {
		if item.Status != "Trading" {
			continue
		}
		if item.ContractType != "LinearPerpetual" {
			continue
		}
		if item.QuoteCoin != "USDT" {
			continue
		}
		symbols = append(symbols, item.Symbol)
	}
	sort.Strings(symbols)

	return symbols, nil
}

func (c *Client) ServerTime(ctx context.Context) (time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v5/market/time", nil)
	if err != nil {
		return time.Time{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("bybit time returned status %d", resp.StatusCode)
	}

	var payload timeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return time.Time{}, err
	}
	if payload.RetCode != 0 {
		return time.Time{}, fmt.Errorf("bybit time error %d: %s", payload.RetCode, payload.RetMsg)
	}

	if len(payload.Result.TimeNano) >= 13 {
		millis, err := strconv.ParseInt(payload.Result.TimeNano[:13], 10, 64)
		if err == nil {
			return time.UnixMilli(millis).UTC(), nil
		}
	}

	seconds, err := strconv.ParseInt(payload.Result.TimeSecond, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(seconds, 0).UTC(), nil
}

type instrumentsResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []instrument `json:"list"`
	} `json:"result"`
}

type timeResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		TimeSecond string `json:"timeSecond"`
		TimeNano   string `json:"timeNano"`
	} `json:"result"`
}

type instrument struct {
	Symbol       string `json:"symbol"`
	Status       string `json:"status"`
	ContractType string `json:"contractType"`
	QuoteCoin    string `json:"quoteCoin"`
}
