package eco

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/pzl/elastibee/pkg/auth"
)

const base = "https://api.ecobee.com"
const filename = "app.json"

type App struct {
	AppKey       string   `json:"app_key"`
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	Thermostats  []string `json:"thermostats,omitempty"`
}

type RequestStatus struct {
	Status struct {
		Code    ResponseCode `json:"code"`
		Message string       `json:"message"`
	} `json:"status"`
}
type ResponseCode int

const (
	StatusSuccess       ResponseCode = 0
	StatusAuthFail      ResponseCode = 1
	StatusNotAuth       ResponseCode = 2
	StatusProcessErr    ResponseCode = 3
	StatusSerializeErr  ResponseCode = 4
	StatusInvalidReqFmt ResponseCode = 5
	StatusTooManyTherm  ResponseCode = 6
	StatusValidationErr ResponseCode = 7
	StatusInvalidFn     ResponseCode = 8
	StatusInvalidSel    ResponseCode = 9
	StatusInvalidPage   ResponseCode = 10
	StatusFnErr         ResponseCode = 11
	StatusNoPOST        ResponseCode = 12
	StatusNoGET         ResponseCode = 13
	StatusTokenExpired  ResponseCode = 14
	StatusDupData       ResponseCode = 15
	StatusDeauth        ResponseCode = 16
)

func (r ResponseCode) String() string {
	switch r {
	case StatusSuccess:
		return "Your request was successfully received and processed."
	case StatusAuthFail:
		return "Invalid credentials supplied to the registration request, or invalid token. Request registration again."
	case StatusNotAuth:
		return "Attempted to access resources which user is not authorized for. Ensure the thermostat identifiers requested are correct."
	case StatusProcessErr:
		return "General catch-all error for a number of internal errors. Additional info may be provided in the message. Check your request. Contact support if persists."
	case StatusSerializeErr:
		return "An internal error mapping data to or from the API transmission format. Contact support."
	case StatusInvalidReqFmt:
		return "An error mapping the request data to internal data objects. Ensure that the properties being sent match properties in the specification."
	case StatusTooManyTherm:
		return "Too many identifiers are specified in the Selecton.selectionMatch property. Current limit is 25 per request."
	case StatusValidationErr:
		return "The update request contained values out of range or too large for the field being updated. See the additional message information as to what caused the validation failure."
	case StatusInvalidFn:
		return "The \"type\" property of the function does not match an available function. Check your request parameters."
	case StatusInvalidSel:
		return "The Selection.selectionType property contains an invalid value."
	case StatusInvalidPage:
		return "The page requested in the request is invalid. Occurs if the page is less than 1 or more than the number of available pages for the request."
	case StatusFnErr:
		return "An error occurred processing a function. Ensure required properties are provided."
	case StatusNoPOST:
		return "The request URL does not support POST."
	case StatusNoGET:
		return "The request URL does not support GET."
	case StatusTokenExpired:
		return "Token expired. Please refresh."
	case StatusDupData:
		return "Fix the data which is duplicated and re-post."
	case StatusDeauth:
		return "Token has been deauthorized by user. You must re-request authorization."
	}
	return fmt.Sprintf("unknown response code: %d", r)
}

func rawfetch(method string, url string, body io.Reader, token string) ([]byte, error) {
	req, err := http.NewRequest(method, base+url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+token)

	c := httpClient(90 * time.Second)
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return buf, nil

}

func (a *App) fetch(method string, url string, body io.Reader) ([]byte, error) {
	buf, err := rawfetch(method, url, body, a.AccessToken)
	if err != nil {
		return nil, err
	}

	var status RequestStatus
	if err := json.Unmarshal(buf, &status); err != nil {
		return nil, err
	}

	if status.Status.Code != StatusSuccess {
		switch status.Status.Code {
		case StatusTokenExpired:
			if err := a.Refresh(); err != nil {
				return nil, fmt.Errorf("access token expired. Got error when refreshing: %w", err)
			}
			return a.fetch(method, url, body)
		default:
			return nil, fmt.Errorf("Code %d (%s): %s", status.Status.Code, status.Status.Code, status.Status.Message)
		}
	}

	return buf, nil
}

func (a *App) Refresh() error {
	tk, err := auth.Refresh(a.AppKey, a.RefreshToken)
	if err != nil {
		return err
	}
	if tk.AccessToken == "" || tk.Refresh == "" {
		return errors.New("empty tokens in response")
	}
	a.AccessToken = tk.AccessToken
	a.RefreshToken = tk.Refresh
	return a.Save()
}

func (a App) Save() error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0600)
}

func Open() (*App, error) {
	var a App

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}

	return &a, nil
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
