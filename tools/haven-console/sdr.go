package main

// The SDR tab: nodes running the SDR agent (sdr-agent/, port 8090 on a node
// with an RTL-SDR). The tab streams waterfall rows from /sdr-node/<ip>/sdr/....

import "encoding/json"

func newSDRFinder(col *collector) *serviceFinder {
	return &serviceFinder{
		col: col, port: "8090", probe: "/sdr/status", prefix: "/sdr-node/",
		fields: func(body []byte) (map[string]any, bool) {
			var st struct {
				SDR    bool   `json:"sdr"`
				Device string `json:"device"`
				Busy   bool   `json:"busy"`
			}
			if json.Unmarshal(body, &st) != nil {
				return nil, false
			}
			return map[string]any{"device": st.Device, "busy": st.Busy}, true
		},
	}
}
