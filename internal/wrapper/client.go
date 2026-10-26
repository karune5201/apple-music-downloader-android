// Package wrapper 是 lite-server HTTP API 的客户端。
//
// 请求一律**不带超时**：lite-server 后面的代理是 QEMU 模拟的进程，冷启动可能
// 慢到几十秒甚至更久。产品决定是「宁可一直等，也不要因为超时杀掉本来会成功的
// 请求」，所以本包不给 client 设整体超时、不用带期限的 context、不留任何请求
// 期限。可恢复的临时故障靠下面的有界重试兜底，而不是靠超时。
package wrapper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"amdl/internal/download"
	"amdl/internal/logx"
)

// 重试策略：一次调用最多试 maxAttempts 次，第 n 次失败后等 n*retryBaseDelay
// 再试（即 500ms、1s）。只重试 transient 判定的临时性错误。
const (
	maxAttempts    = 3
	retryBaseDelay = 500 * time.Millisecond
)

// ErrNotConfigured is returned when the wrapper-lite server URL is empty.
var ErrNotConfigured = errors.New("lite-server is not configured")

// Client interacts with the wrapper-lite HTTP API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New creates a new Client with default HTTP client.
func New(baseURL string) *Client {
	httpClient := download.Client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return NewWithHTTPClient(baseURL, httpClient)
}

// NewWithHTTPClient creates a new Client with a custom HTTP client.
//
// 传进来的 client 原样使用：本包不设任何期限（见包注释）。
func NewWithHTTPClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		HTTPClient: httpClient,
	}
}

// httpClient 返回真正要用的 client，不设期限、不挂 context deadline。
//
// 产品决定：请求故意不设期限，卡住的上游可以一直等下去，短超时反而会把本来
// 能成功的请求杀掉。所以这里既不新建带期限的 client，也不改写调用方给的 client。
func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	if download.Client != nil {
		return download.Client
	}
	return http.DefaultClient
}

// ---------------------------------------------------------------- 重试

// retryRequest 按包内策略执行一次 lite-server 调用。
//
// op 是「发请求 + 解析信封」的整体，这样 code=-1 这类上游临时错误也会被重试。
// 非临时错误立刻返回（不重试）；临时错误试满 maxAttempts 次后返回最后一个
// 错误。label 是端点路径，rawURL 是完整 URL，两者只用于日志。
func retryRequest(label, rawURL string, op func() error) error {
	var lastErr error
	urlLogged := false
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = op()
		if lastErr == nil {
			return nil
		}
		if errors.Is(lastErr, ErrNotConfigured) || !transient(lastErr) {
			return lastErr
		}
		// 重试时把完整 URL 打一次出来（细节级别），方便对着 lite-server 日志排查。
		if !urlLogged {
			logx.DetailT("net.lite.url", rawURL)
			urlLogged = true
		}
		if attempt == maxAttempts {
			break
		}
		logx.WarnT("net.lite.retry", label, attempt, maxAttempts, lastErr)
		time.Sleep(time.Duration(attempt) * retryBaseDelay)
	}
	logx.ErrorT("net.lite.giveup", label, maxAttempts, lastErr)
	return lastErr
}

// transient 判断 err 是否属于值得重试的临时故障。
//
// 覆盖：EOF / 连接被重置 / 管道破裂 / net.Error（含超时、连接被拒绝等）、
// 以及上游 QEMU 代理吐出来的若干固定说法（upstream error、code=-1、
// http 502/503/504 ……）。其余错误一律不重试，直接失败。
func transient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range transientNeedles {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// transientNeedles 是上游临时故障的错误信息特征（统一按小写比较）。
var transientNeedles = []string{
	"eof",
	"connection reset",
	"broken pipe",
	"econnreset",
	"epipe",
	"upstream error",
	"connection refused",
	"timeout",
	"temporarily unavailable",
	"code=-1",
	"http 502",
	"http 503",
	"http 504",
}

// ---------------------------------------------------------------- 请求

// get 发一次 GET（不含重试，重试由 retryRequest 包在外面）。
func (c *Client) get(endpoint string) ([]byte, error) {
	if c.BaseURL == "" {
		return nil, ErrNotConfigured
	}
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}
	return c.roundTrip(endpoint, req)
}

// post 发一次 POST（不含重试，重试由 retryRequest 包在外面）。
func (c *Client) post(endpoint string, reqBody any) ([]byte, error) {
	if c.BaseURL == "" {
		return nil, ErrNotConfigured
	}

	var bodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return c.roundTrip(endpoint, req)
}

// roundTrip 发送请求并读完响应体。
//
// 刻意不挂带期限的 context、不给 client 设整体超时：上游卡住就一直等下去。
func (c *Client) roundTrip(endpoint string, req *http.Request) ([]byte, error) {
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		trimmed := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusNotFound && strings.HasPrefix(endpoint, "/lyrics") {
			var check struct {
				Code any `json:"code"`
				Data any `json:"data"`
			}
			if json.Unmarshal(body, &check) == nil {
				if isCode404(check.Code) || isCode404(check.Data) {
					return body, nil
				}
				if m, ok := check.Data.(map[string]any); ok && isCode404(m["code"]) {
					return body, nil
				}
			}
		}
		if trimmed != "" {
			return nil, fmt.Errorf("lite-server %s returned %s (http %d): %s", endpointPath(endpoint), resp.Status, resp.StatusCode, trimmed)
		}
		return nil, fmt.Errorf("lite-server %s returned %s (http %d)", endpointPath(endpoint), resp.Status, resp.StatusCode)
	}

	return body, nil
}

// getEnvelope 取一次 GET 结果并解开信封，整个过程带重试。
func getEnvelope[T any](c *Client, endpoint string) (T, error) {
	var data T
	err := retryRequest(endpointPath(endpoint), c.BaseURL+endpoint, func() error {
		body, err := c.get(endpoint)
		if err != nil {
			return err
		}
		decoded, err := decodeEnvelope[T](body, endpoint)
		if err != nil {
			return err
		}
		data = decoded
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return data, nil
}

// postEnvelope 取一次 POST 结果并解开信封，整个过程带重试。
func postEnvelope[T any](c *Client, endpoint string, reqBody any) (T, error) {
	var data T
	err := retryRequest(endpointPath(endpoint), c.BaseURL+endpoint, func() error {
		body, err := c.post(endpoint, reqBody)
		if err != nil {
			return err
		}
		decoded, err := decodeEnvelope[T](body, endpoint)
		if err != nil {
			return err
		}
		data = decoded
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return data, nil
}

func endpointPath(endpoint string) string {
	if idx := strings.IndexByte(endpoint, '?'); idx != -1 {
		return endpoint[:idx]
	}
	return endpoint
}

func decodeEnvelope[T any](body []byte, endpoint string) (T, error) {
	var zero T
	var envelope Response[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return zero, err
	}
	if envelope.Code != 0 {
		return zero, fmt.Errorf("lite-server %s returned code=%d msg=%s", endpointPath(endpoint), envelope.Code, envelope.Msg)
	}
	return envelope.Data, nil
}

func isCode404(v any) bool {
	switch val := v.(type) {
	case float64:
		return int(val) == 404
	case int:
		return val == 404
	case string:
		return strings.TrimSpace(val) == "404"
	default:
		return false
	}
}
