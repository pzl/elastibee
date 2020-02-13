package eco

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// https://www.ecobee.com/home/developer/api/documentation/v1/operations/get-runtime-report.shtml

type sensor struct {
	ID    string `json:"sensorId"`
	Name  string `json:"sensorName"`
	Type  string `json:"sensorType"`
	Usage string `json:"sensorUsage"`
}

type ecoRuntimeResponse struct {
	Columns    string `json:"columns"`
	ReportList []struct {
		ID       string   `json:"thermostatIdentifier"`
		RowCount int      `json:"rowCount"`
		Rows     []string `json:"rowList"`
	} `json:"reportList"`
	SensorList []struct {
		ID      string   `json:"thermostatIdentifier"`
		Sensors []sensor `json:"sensors"`
		Columns []string `json:"columns"`
		Data    []string `json:"data"`
	} `json:"sensorList"`
	StartDate string `json:"startDate"`
	StartIntv int    `json:"startInterval"`
	EndDate   string `json:"endDate"`
	EndIntv   int    `json:"endInterval"`
	RequestStatus
}

type RuntimeData struct {
	Data       []map[string]interface{} `json:"data"`
	SensorData []map[string]interface{} `json:"sensor_data"`
}

func (a *App) GetRuntimeData(start string, end string) (RuntimeData, error) {

	if len(a.Thermostats) == 0 {
		ts, err := a.GetThermostats()
		if err != nil {
			return RuntimeData{}, fmt.Errorf("no saved thermostat IDs. Got error when fetching registered thermostats: %w", err)
		}
		a.Thermostats = make([]string, len(ts))
		for i := range ts {
			a.Thermostats[i] = ts[i].ID
		}
		a.Save() // nolint
	}

	req, err := json.Marshal(map[string]interface{}{
		"startDate": start,
		"endDate":   end,
		"columns": strings.Join([]string{
			"auxHeat1", "auxHeat2", "auxHeat3", "compCool1", "compCool2",
			"compHeat1", "compHeat2", "dehumidifier", "dmOffset", "economizer",
			"fan", "humidifier", "hvacMode", "outdoorHumidity", "outdoorTemp", "sky",
			"ventilator", "wind", "zoneAveTemp", "zoneCalendarEvent", "zoneClimate",
			"zoneCoolTemp", "zoneHeatTemp", "zoneHumidity", "zoneHumidityHigh",
			"zoneHumidityLow", "zoneHvacMode", "zoneOccupancy",
		}, ","),
		"includeSensors": true,
		"selection": map[string]string{
			"selectionType":  "thermostats",
			"selectionMatch": strings.Join(a.Thermostats, ","),
		},
	})
	if err != nil {
		return RuntimeData{}, err
	}

	params := url.Values{}
	params.Add("format", "json")
	params.Add("body", string(req))

	body, err := a.fetch("GET", "/1/runtimeReport?"+params.Encode(), nil)
	if err != nil {
		return RuntimeData{}, err
	}
	return parseRuntime(body)

}

func parseRuntime(d []byte) (RuntimeData, error) {
	var res ecoRuntimeResponse
	if err := json.Unmarshal(d, &res); err != nil {
		return RuntimeData{}, err
	}

	rd := RuntimeData{
		Data:       make([]map[string]interface{}, 0, len(res.ReportList[0].Rows)*len(res.ReportList)),
		SensorData: make([]map[string]interface{}, 0, len(res.SensorList[0].Data)*len(res.SensorList)),
	}

	cols := strings.Split(res.Columns, ",")
	for _, rl := range res.ReportList {
		for _, r := range rl.Rows {
			fields := strings.Split(r, ",")
			data := map[string]interface{}{
				"date":       fields[0],
				"time":       fields[1],
				"@timestamp": fields[0] + "T" + fields[1],
				"type":       "thermostat",
			}
			fields = fields[2:]
			for j, c := range cols {
				switch strings.ToLower(c) {
				case "auxheat1", "auxheat2", "auxheat3", "compcool1", "compcool2", "compheat1", "compheat2", "dehumidifier",
					"economizer", "fan", "humidifier", "outdoorhumidity", "ventilator", "wind", "zonehumidity", "zonehumidityhigh",
					"zonehumiditylow", "zone":
					if num, err := strconv.Atoi(fields[j]); err == nil {
						data[c] = num
					} else { // failed to convert
						data[c] = fields[j]
					}
				case "dmoffset", "outdoortemp", "zoneavetemp", "zonecooltemp", "zoneheattemp":
					if num, err := strconv.ParseFloat(fields[j], 64); err == nil {
						data[c] = num
					} else {
						data[c] = fields[j]
					}
				case "zoneoccupancy":
					if fields[j] == "0" {
						data[c] = false
					} else {
						data[c] = true
					}
				default:
					data[c] = fields[j]
				}
			}
			rd.Data = append(rd.Data, data)
		}
	}

	// arrangement:
	// sensors: [ { id: "rs:100:1", name: "Bedroom", type: "occupancy" }, ... ]
	// columns: [ "date", "time", "rs:100:1", "rs:100:2", "rs2:100:1", ... ]
	// data: [ "2020-02-09,19:00:00,71..4,...", ... ]
	//
	// need to split data, match to column index, and if it's a sensor ID, match to sensor

	for _, sl := range res.SensorList {
		ss := make(map[string]sensor)
		for _, s := range sl.Sensors {
			ss[s.ID] = s
		}

		for _, s := range sl.Data {
			fields := strings.Split(s, ",")

			date := fields[0]
			tm := fields[1]
			fields = fields[2:]
			columns := sl.Columns[2:]

			for i, f := range fields {
				if sensor, ok := ss[columns[i]]; ok {
					data := map[string]interface{}{
						"date":       date,
						"time":       tm,
						"type":       "sensor",
						"@timestamp": date + "T" + tm,
						"sensor": map[string]string{
							"id":    sensor.ID,
							"name":  sensor.Name,
							"type":  sensor.Type,
							"usage": sensor.Usage,
						},
					}

					// https://www.ecobee.com/home/developer/api/documentation/v1/objects/RuntimeSensorMetadata.shtml
					switch sensor.Type {
					case "occupancy", "dryContact":
						if f == "0" {
							data[sensor.Type] = false
						} else {
							data[sensor.Type] = true
						}
					case "temperature":
						if num, err := strconv.ParseFloat(f, 64); err == nil {
							data[sensor.Type] = num
						} else {
							data[sensor.Type] = f
						}
					case "co2", "ctclamp", "humidity", "plug", "pulsedElectricityMeter":
						if num, err := strconv.Atoi(f); err == nil {
							data[sensor.Type] = num
						} else { // failed to convert
							data[sensor.Type] = f
						}
					default:
						data[sensor.Type] = f
					}

					rd.SensorData = append(rd.SensorData, data)
				}
			}
		}
	}

	return rd, nil
}
