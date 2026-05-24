package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
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

type instrumentsResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []instrument `json:"list"`
	} `json:"result"`
}

type instrument struct {
	Symbol       string `json:"symbol"`
	Status       string `json:"status"`
	ContractType string `json:"contractType"`
	QuoteCoin    string `json:"quoteCoin"`
}
