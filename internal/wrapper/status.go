package wrapper

// Status queries the wrapper-lite /status endpoint and returns the supported regions.
//
// 请求无超时（见包注释），临时故障有界重试。
func (c *Client) Status() ([]string, error) {
	data, err := getEnvelope[StatusData](c, "/status")
	if err != nil {
		return nil, err
	}
	return data.Regions, nil
}

// GetStatus queries wrapper-lite's /status endpoint using baseURL and returns the supported regions.
func GetStatus(baseURL string) ([]string, error) {
	return New(baseURL).Status()
}
