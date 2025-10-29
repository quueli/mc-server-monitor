package mcping

import (
	"bufio"
	"bytes"
	"errors"
	"testing"
)

func TestVarIntRoundTrip(t *testing.T) {
	cases := []struct {
		value int32
		bytes []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xff, 0x01}},
		{25565, []byte{0xdd, 0xc7, 0x01}},
		{2147483647, []byte{0xff, 0xff, 0xff, 0xff, 0x07}},
		{-1, []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
	}

	for _, c := range cases {
		var buf [maxVarIntBytes]byte
		n := putVarInt(buf[:], c.value)
		if !bytes.Equal(buf[:n], c.bytes) {
			t.Errorf("encode %d: got % x, want % x", c.value, buf[:n], c.bytes)
		}

		got, err := readVarInt(bytes.NewReader(c.bytes))
		if err != nil {
			t.Errorf("decode %d: %v", c.value, err)
			continue
		}
		if got != c.value {
			t.Errorf("decode % x: got %d, want %d", c.bytes, got, c.value)
		}
	}
}

func TestReadVarIntRejectsOverlong(t *testing.T) {
	// six continuation bytes never terminate within the 5-byte limit.
	_, err := readVarInt(bytes.NewReader([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80}))
	if !errors.Is(err, errVarIntTooLong) {
		t.Fatalf("want errVarIntTooLong, got %v", err)
	}
}

// static status packet for {"players":{"online":7,"max":20}}.
// 0x23 = total inner length (35), 0x00 = packet id, 0x21 = json length (33).
func TestReadStatusResponse(t *testing.T) {
	jsonBody := []byte(`{"players":{"online":7,"max":20}}`)
	packet := append([]byte{0x23, 0x00, 0x21}, jsonBody...)

	payload, err := readStatusResponse(bufio.NewReader(bytes.NewReader(packet)))
	if err != nil {
		t.Fatalf("readStatusResponse: %v", err)
	}
	if !bytes.Equal(payload, jsonBody) {
		t.Fatalf("payload mismatch:\n got %s\nwant %s", payload, jsonBody)
	}

	status, err := parseStatus(payload)
	if err != nil {
		t.Fatalf("parseStatus: %v", err)
	}
	if status.Players.Online != 7 || status.Players.Max != 20 {
		t.Fatalf("players: got %d/%d, want 7/20", status.Players.Online, status.Players.Max)
	}
}

func TestParseStatus(t *testing.T) {
	cases := []struct {
		name     string
		json     string
		wantMOTD string
		wantVer  string
	}{
		{
			name:     "plain string motd",
			json:     `{"version":{"name":"1.20.1","protocol":763},"players":{"online":5,"max":100},"description":"A Minecraft Server"}`,
			wantMOTD: "A Minecraft Server",
			wantVer:  "1.20.1",
		},
		{
			name:     "chat component motd with colour codes",
			json:     `{"version":{"name":"Paper 1.21","protocol":767},"players":{"online":42,"max":500},"description":{"text":"","extra":[{"text":"§aWelcome "},{"text":"home"}]},"favicon":"data:image/png;base64,AAAA"}`,
			wantMOTD: "Welcome home",
			wantVer:  "Paper 1.21",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, err := parseStatus([]byte(c.json))
			if err != nil {
				t.Fatalf("parseStatus: %v", err)
			}
			if got := status.MOTD.Clean(); got != c.wantMOTD {
				t.Errorf("motd: got %q, want %q", got, c.wantMOTD)
			}
			if status.Version.Name != c.wantVer {
				t.Errorf("version: got %q, want %q", status.Version.Name, c.wantVer)
			}
		})
	}
}
