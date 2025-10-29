package mcping

import (
	"encoding/json"
	"strings"
)

// Status is the decoded status response, mirroring the JSON a server sends for
// the modern (1.7+) Server List Ping.
type Status struct {
	Version   Version     `json:"version"`
	Players   Players     `json:"players"`
	MOTD      Description `json:"description"`
	Favicon   string      `json:"favicon"`
	LatencyMS int64       `json:"-"`
}

type Version struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type Players struct {
	Online int            `json:"online"`
	Max    int            `json:"max"`
	Sample []PlayerSample `json:"sample,omitempty"`
}

type PlayerSample struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// Description is the MOTD. Servers send it either as a plain string or as a
// chat component object with nested "extra" parts, so it needs a custom
// unmarshaller that flattens both into text.
type Description struct {
	Text string
}

func (d *Description) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		d.Text = s
		return nil
	}
	var c chatComponent
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	d.Text = c.flatten()
	return nil
}

type chatComponent struct {
	Text  string          `json:"text"`
	Extra []chatComponent `json:"extra"`
}

func (c chatComponent) flatten() string {
	var b strings.Builder
	b.WriteString(c.Text)
	for _, e := range c.Extra {
		b.WriteString(e.flatten())
	}
	return b.String()
}

// Clean returns the MOTD with the section-sign (§) colour and style codes removed.
func (d Description) Clean() string {
	return stripFormatting(d.Text)
}

func stripFormatting(s string) string {
	const section = '§'
	var b strings.Builder
	b.Grow(len(s))
	skip := false
	for _, r := range s {
		if skip {
			skip = false
			continue
		}
		if r == section {
			skip = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func parseStatus(payload []byte) (*Status, error) {
	var s Status
	if err := json.Unmarshal(payload, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
