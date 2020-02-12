package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"
)

const eco = "https://api.ecobee.com"
const pin = "ecobeePin"
const scope = "smartWrite"

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	Type        string `json:"token_type"`
	Expires     int    `json:"expires_in"`
	Refresh     string `json:"refresh_token"`
	Scope       string `json:"scope"`
	// error state
	Error     string `json:"error"`
	ErrorDesc string `json:"error_description"`
	ErrorURI  string `json:"error_uri"`
}

type PinResponse struct {
	Pin      string `json:"ecobeePin"`
	Code     string `json:"code"`
	Scope    string `json:"scope"`
	Expires  int    `json:"expires_in"`
	Interval int    `json:"interval"`
}

// creates a new PIN for the user to enter into their ecobee portal
func MakePin(appKey string) (PinResponse, error) {
	pr := PinResponse{}
	client := httpClient(30 * time.Second) // generous timeout, since ecobee servers are occasionally... sluggish

	params := url.Values{}
	params.Add("response_type", pin)
	params.Add("client_id", appKey)
	params.Add("scope", scope)

	res, err := client.Get(eco + "/authorize?" + params.Encode())
	if err != nil {
		return pr, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return pr, err
	}

	if err := json.Unmarshal(body, &pr); err != nil {
		return pr, err
	}
	return pr, nil
}

// after successful pin entry, converts the associated pin code into usable tokens
func MakeToken(appKey string, code string) (TokenResponse, error) {
	tr := TokenResponse{}
	client := httpClient(30 * time.Second) // generous timeout, since ecobee servers are occasionally... sluggish

	params := url.Values{}
	params.Add("grant_type", pin)
	params.Add("client_id", appKey)
	params.Add("code", code)

	res, err := client.Post(eco+"/token?"+params.Encode(), "application/json", nil)
	if err != nil {
		return tr, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return tr, err
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return tr, err
	}
	if tr.Error != "" {
		return tr, fmt.Errorf("%s: %s", tr.Error, tr.ErrorDesc)
	}
	return tr, nil
}

func Refresh(appKey string, token string) (TokenResponse, error) {
	tr := TokenResponse{}
	client := httpClient(30 * time.Second) // generous timeout, since ecobee servers are occasionally... sluggish

	params := url.Values{}
	params.Add("grant_type", "refresh_token")
	params.Add("refresh_token", token)
	params.Add("client_id", appKey)

	res, err := client.Post(eco+"/token?"+params.Encode(), "application/json", nil)
	if err != nil {
		return tr, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return tr, err
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return tr, err
	}
	if tr.Error != "" {
		return tr, fmt.Errorf("%s: %s", tr.Error, tr.ErrorDesc)
	}
	return tr, nil
}

func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Dial:                (&net.Dialer{Timeout: timeout}).Dial,
			TLSHandshakeTimeout: timeout,
		},
	}
}
