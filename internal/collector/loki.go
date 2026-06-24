package collector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LokiClient는 Loki HTTP API(/loki/api/v1/query_range)로 로그를 조회합니다.
// (architecture.md §4.1 — 엔드포인트는 설정으로 주입)
type LokiClient struct {
	baseURL string
	client  *http.Client
}

// NewLokiClient는 baseURL이 비어 있으면 nil을 반환합니다(수집 건너뜀).
func NewLokiClient(baseURL string) *LokiClient {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	return &LokiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

type lokiResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"` // [ [ "<ns>", "<line>" ], ... ]
		} `json:"result"`
	} `json:"data"`
}

// QueryRecent는 최근 1시간 동안의 로그 라인을 LogQL selector로 조회합니다.
func (c *LokiClient) QueryRecent(logQL string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now()
	start := now.Add(-1 * time.Hour)

	q := url.Values{}
	q.Set("query", logQL)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	q.Set("end", strconv.FormatInt(now.UnixNano(), 10))
	q.Set("direction", "backward")

	u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", c.baseURL, q.Encode())
	resp, err := c.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("loki query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loki query returned status %d", resp.StatusCode)
	}

	var lr lokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, fmt.Errorf("failed to decode loki response: %w", err)
	}

	lines := make([]string, 0, limit)
	for _, r := range lr.Data.Result {
		for _, v := range r.Values {
			if len(v) == 2 {
				lines = append(lines, v[1])
			}
		}
	}
	return lines, nil
}
