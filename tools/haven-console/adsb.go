package main

// The ADS-B tab: nodes running dump1090 with its SkyAware map (port 8080 on a
// node with an RTL-SDR; see integrations/adsb). The tab reads live aircraft
// from /adsb-node/<ip>/data/aircraft.json and shows the offline map from
// /adsb-node/<ip>/.

import "encoding/json"

func newADSBFinder(col *collector) *serviceFinder {
	return &serviceFinder{
		col: col, port: "8080", probe: "/data/receiver.json", prefix: "/adsb-node/",
		fields: func(body []byte) (map[string]any, bool) {
			var rc struct {
				Refresh *float64 `json:"refresh"`
				Lat     *float64 `json:"lat"`
				Lon     *float64 `json:"lon"`
			}
			if json.Unmarshal(body, &rc) != nil || rc.Refresh == nil {
				return nil, false
			}
			out := map[string]any{}
			if rc.Lat != nil && rc.Lon != nil {
				out["lat"], out["lon"] = *rc.Lat, *rc.Lon
			}
			return out, true
		},
	}
}
