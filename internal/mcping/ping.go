package mcping

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultPort is the standard Minecraft Java Edition port.
	DefaultPort = 25565
	// protocolVersion is sent in the handshake. For a status ping the server
	// does not care which version we claim; 47 (1.8) is widely accepted.
	protocolVersion = 47
	stateStatus     = 1
	// guard against a server advertising an absurd payload length.
	maxStatusBytes = 1 << 20
)

// Pinger performs a Server List Ping against a single host.
type Pinger struct {
	Timeout    time.Duration
	ResolveSRV bool
}

// New returns a Pinger with sane defaults.
func New(timeout time.Duration) *Pinger {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Pinger{Timeout: timeout, ResolveSRV: true}
}

// Ping opens a TCP connection to host:port, runs the handshake and status
// exchange, and returns the parsed status. host may be a bare hostname; when
// ResolveSRV is set a _minecraft._tcp SRV record is honoured first.
func (p *Pinger) Ping(host string, port int) (*Status, error) {
	if port == 0 {
		port = DefaultPort
	}

	dialHost, dialPort := host, port
	if p.ResolveSRV {
		if h, pt, ok := lookupSRV(host); ok {
			dialHost, dialPort = h, pt
		}
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(dialHost, strconv.Itoa(dialPort)), p.Timeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(p.Timeout))

	// the address written into the handshake is the one the client "typed", so
	// virtualhost routing on the server still resolves the right instance.
	if err := writeHandshake(conn, host, port); err != nil {
		return nil, err
	}
	if err := writeStatusRequest(conn); err != nil {
		return nil, err
	}

	start := time.Now()
	payload, err := readStatusResponse(bufio.NewReader(conn))
	if err != nil {
		return nil, err
	}
	latency := time.Since(start)

	status, err := parseStatus(payload)
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	status.LatencyMS = latency.Milliseconds()
	return status, nil
}

func writeHandshake(conn net.Conn, host string, port int) error {
	var body bytes.Buffer
	_ = writeVarInt(&body, 0x00) // packet id
	_ = writeVarInt(&body, protocolVersion)
	_ = writeString(&body, host)
	_ = binary.Write(&body, binary.BigEndian, uint16(port))
	_ = writeVarInt(&body, stateStatus)
	return writeFramed(conn, body.Bytes())
}

func writeStatusRequest(conn net.Conn) error {
	var body bytes.Buffer
	_ = writeVarInt(&body, 0x00) // status request, empty body
	return writeFramed(conn, body.Bytes())
}

// each packet on the wire is a varint length followed by that many bytes.
func writeFramed(w io.Writer, body []byte) error {
	if err := writeVarInt(w, int32(len(body))); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func writeString(w io.Writer, s string) error {
	if err := writeVarInt(w, int32(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

func readStatusResponse(r *bufio.Reader) ([]byte, error) {
	length, err := readVarInt(r)
	if err != nil {
		return nil, err
	}
	if length <= 0 || length > maxStatusBytes {
		return nil, fmt.Errorf("mcping: bad packet length %d", length)
	}
	packet := make([]byte, length)
	if _, err := io.ReadFull(r, packet); err != nil {
		return nil, err
	}

	pr := bytes.NewReader(packet)
	packetID, err := readVarInt(pr)
	if err != nil {
		return nil, err
	}
	if packetID != 0x00 {
		return nil, fmt.Errorf("mcping: unexpected packet id 0x%02x", packetID)
	}
	return readString(pr)
}

func readString(r *bytes.Reader) ([]byte, error) {
	n, err := readVarInt(r)
	if err != nil {
		return nil, err
	}
	if n < 0 || int(n) > r.Len() {
		return nil, fmt.Errorf("mcping: bad string length %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func lookupSRV(host string) (string, int, bool) {
	_, records, err := net.LookupSRV("minecraft", "tcp", host)
	if err != nil || len(records) == 0 {
		return "", 0, false
	}
	target := strings.TrimSuffix(records[0].Target, ".")
	if target == "" {
		return "", 0, false
	}
	return target, int(records[0].Port), true
}
