package wrapper

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// License sends a license request to /license and returns the decoded binary license.
//
// 请求无超时（见包注释），临时故障有界重试。
func (c *Client) License(req LicenseRequest) ([]byte, error) {
	data, err := postEnvelope[LicenseData](c, "/license", req)
	if err != nil {
		return nil, err
	}
	if data.License == "" {
		return nil, errors.New("empty license in lite-server response")
	}
	license, err := base64.StdEncoding.DecodeString(data.License)
	if err != nil {
		return nil, fmt.Errorf("failed to decode license: %w", err)
	}
	return license, nil
}

// PlayReadyLicense sends a PlayReady DRM license challenge to /license and returns the decoded license XML.
func (c *Client) PlayReadyLicense(adamID, challenge, uri string) ([]byte, error) {
	req := LicenseRequest{
		AdamId:    adamID,
		Challenge: challenge,
		URI:       uri,
		DRMType:   "pr",
	}
	return c.License(req)
}

// WidevineLicense sends a Widevine DRM license challenge (Base64-encoded) to /license and returns the decoded license.
func (c *Client) WidevineLicense(adamID, uri, challengeBase64 string) ([]byte, error) {
	req := LicenseRequest{
		AdamId:    adamID,
		Challenge: challengeBase64,
		URI:       uri,
	}
	return c.License(req)
}

// GetPlayReadyLicense posts a PlayReady license challenge to wrapper-lite /license using baseURL.
func GetPlayReadyLicense(baseURL, adamID, challenge, uri string) ([]byte, error) {
	return New(baseURL).PlayReadyLicense(adamID, challenge, uri)
}

// GetWidevineLicense posts a Widevine license challenge to wrapper-lite /license using baseURL.
func GetWidevineLicense(baseURL, adamID, uri, challengeBase64 string) ([]byte, error) {
	return New(baseURL).WidevineLicense(adamID, uri, challengeBase64)
}

// WidevineBeforeRequest is a resty hook that posts the license challenge to wrapper-lite /license.
//
// 这里只把调用方给的 ctx 原样透传，不从中派生 deadline、也不给 resty client 设超时：
// 本包刻意让请求无期限（上游 QEMU 代理冷启动很慢）。临时故障有界重试。
func WidevineBeforeRequest(cl *resty.Client, ctx context.Context, url string, body []byte) (*resty.Response, error) {
	uri := ctx.Value("uriPrefix").(string) + "," + ctx.Value("pssh").(string)
	jsondata := map[string]interface{}{
		"challenge": base64.StdEncoding.EncodeToString(body),
		"uri":       uri,
		"adamId":    ctx.Value("adamId").(string),
	}

	var resp *resty.Response
	err := retryRequest("/license", url, func() error {
		r, err := cl.R().
			SetContext(ctx).
			SetBody(jsondata).
			Post(url)
		if err != nil {
			return err
		}
		resp = r
		return nil
	})
	if err != nil {
		return resp, err
	}
	return resp, nil
}

// WidevineAfterRequest is a resty hook that unwraps and decodes the lite-server license response.
func WidevineAfterRequest(response *resty.Response) ([]byte, error) {
	var responseData PlaybackLicense
	if err := json.Unmarshal(response.Body(), &responseData); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}
	if responseData.Code != 0 {
		return nil, fmt.Errorf("lite-server /license returned code=%d msg=%s", responseData.Code, responseData.Msg)
	}
	if responseData.Data.License == "" {
		return nil, errors.New("empty license in lite-server response")
	}
	license, err := base64.StdEncoding.DecodeString(responseData.Data.License)
	if err != nil {
		return nil, fmt.Errorf("failed to decode license: %w", err)
	}
	return license, nil
}
