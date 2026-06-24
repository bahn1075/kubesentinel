package collector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PrometheusClient는 Prometheus HTTP API(/api/v1/query)로 instant 쿼리를 수행합니다.
// (architecture.md §4.1 — 엔드포인트는 설정으로 주입)
type PrometheusClient struct {
	baseURL string
	client  *http.Client
}

// NewPrometheusClient는 baseURL이 비어 있으면 nil을 반환합니다(수집 건너뜀).
func NewPrometheusClient(baseURL string) *PrometheusClient {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	return &PrometheusClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// promResponse는 Prometheus query API 응답의 일부입니다.
type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"` // [ <unix_ts>, "<sample_value>" ]
		} `json:"result"`
	} `json:"data"`
}

// QueryInstant는 PromQL instant 쿼리를 실행하고 결과 벡터를 반환합니다.
func (c *PrometheusClient) QueryInstant(query string) ([]map[string]interface{}, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s", c.baseURL, url.QueryEscape(query))
	resp, err := c.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus query returned status %d", resp.StatusCode)
	}

	var pr promResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("failed to decode prometheus response: %w", err)
	}
	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus query status: %s", pr.Status)
	}

	out := make([]map[string]interface{}, 0, len(pr.Data.Result))
	for _, r := range pr.Data.Result {
		sample := map[string]interface{}{"metric": r.Metric}
		if len(r.Value) == 2 {
			sample["value"] = r.Value[1]
		}
		out = append(out, sample)
	}
	return out, nil
}
