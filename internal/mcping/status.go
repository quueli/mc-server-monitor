package mcping

import "encoding/json"

// Status is the decoded status response from a server.
type Status struct {
	Version   Version `json:"version"`
	Players   Players `json:"players"`
	MOTD      string  `json:"description"`
	LatencyMS int64   `json:"-"`
}

type Version struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type Players struct {
	Online int `json:"online"`
	Max    int `json:"max"`
}

func parseStatus(payload []byte) (*Status, error) {
	var s Status
	if err := json.Unmarshal(payload, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
