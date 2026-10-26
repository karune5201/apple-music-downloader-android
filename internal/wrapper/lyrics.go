package wrapper

import (
	"encoding/json"
	"errors"
	"net/url"
)

// ErrLyricsNotFound is returned when wrapper-lite indicates no lyrics are available (code 404).
var ErrLyricsNotFound = errors.New("no lyrics available for this song")

// ErrNoLyrics is an alias for ErrLyricsNotFound.
var ErrNoLyrics = ErrLyricsNotFound

// Lyrics queries the wrapper-lite /lyrics endpoint for the given adamID, language, and syllable setting.
//
// 请求无超时（见包注释），临时故障按 retryRequest 的策略重试；「没有歌词」是正常
// 结果，不作为可重试的失败。
func (c *Client) Lyrics(adamID, language string, syllable bool) (string, error) {
	isSyllable := "0"
	if syllable {
		isSyllable = "1"
	}
	endpoint := "/lyrics?adamId=" + url.QueryEscape(adamID) + "&language=" + url.QueryEscape(language) + "&syllable=" + isSyllable

	var lyrics string
	err := retryRequest(endpointPath(endpoint), c.BaseURL+endpoint, func() error {
		body, err := c.get(endpoint)
		if err != nil {
			return err
		}
		data, err := parseLyrics(body, endpoint)
		if err != nil {
			return err
		}
		lyrics = data.Lyrics
		return nil
	})
	if err != nil {
		return "", err
	}
	return lyrics, nil
}

// parseLyrics 解开 /lyrics 信封，并把 lite-server 表达「没有歌词」的几种写法
// 统一成 ErrLyricsNotFound。
func parseLyrics(body []byte, endpoint string) (LyricsData, error) {
	var raw struct {
		Code any    `json:"code"`
		Msg  string `json:"msg"`
		Data any    `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err == nil {
		if isCode404(raw.Code) || isCode404(raw.Data) {
			return LyricsData{}, ErrLyricsNotFound
		}
		if dataMap, ok := raw.Data.(map[string]any); ok {
			if isCode404(dataMap["code"]) {
				return LyricsData{}, ErrLyricsNotFound
			}
		}
	}

	data, err := decodeEnvelope[LyricsData](body, endpoint)
	if err != nil {
		return LyricsData{}, err
	}
	if data.Code == 404 {
		return LyricsData{}, ErrLyricsNotFound
	}
	return data, nil
}

// GetLyrics queries wrapper-lite's /lyrics endpoint using baseURL.
func GetLyrics(baseURL, adamID, language string, syllable bool) (string, error) {
	return New(baseURL).Lyrics(adamID, language, syllable)
}
