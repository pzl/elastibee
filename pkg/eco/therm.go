package eco

import (
	"encoding/json"
	"net/url"
)

type Thermostat struct {
	ID         string `json:"identifier"`
	Name       string `json:"name"`
	Revision   string `json:"thermostatRev"`
	Registered bool   `json:"isRegistered"`
	ModelNo    string `json:"modelNumber"`
	Brand      string `json:"brand"`
	Features   string `json:"features"`
	LastMod    string `json:"lastModified"`
	ThermTime  string `json:"thermostatTime"`
	UTC        string `json:"utcTime"`
}

type thermostatResponse struct {
	Thermostats []Thermostat `json:"thermostatList"`
	RequestStatus
}

func (a *App) GetThermostats() ([]Thermostat, error) {

	req, err := json.Marshal(map[string]map[string]string{
		"selection": map[string]string{
			"selectionType": "registered",
		},
	})
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Add("json", string(req))

	body, err := a.fetch("GET", "/1/thermostat?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var res thermostatResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	return res.Thermostats, nil

}
