package elastic

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
)

func (c Client) CreateIndexFromFile(idx string, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return c.CreateIndex(idx, f)
}

func (c Client) CreateIndex(idx string, body io.Reader) error {
	req, err := http.NewRequest("PUT", c.Host+"/"+idx, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}

	return nil
}

func (c Client) IndexExists(idx string) bool {
	res, err := c.http.Get(path.Join(c.Host, idx))
	if err != nil {
		return true
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return true
	}
	return false
}
