package elastic

import (
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	Host string
	http *http.Client
}

func New(host string) Client {
	timeout := 20 * time.Second

	return Client{
		Host: host,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Dial:                (&net.Dialer{Timeout: timeout}).Dial,
				TLSHandshakeTimeout: timeout,
			},
		},
	}
}

func (c Client) Bulk(idx string, body io.Reader) error {
	req, err := http.NewRequest("POST", c.Host+"/"+idx+"/_bulk", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// todo, check return values, statuses

	return nil
}
