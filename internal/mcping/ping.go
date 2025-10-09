package mcping

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

const (
	// DefaultPort is the standard Minecraft Java Edition port.
	DefaultPort = 25565
	protocolVersion = 47
	stateStatus     = 1
	maxStatusBytes  = 1 << 20
)

// Pinger performs a Server List Ping against a single host.
type Pinger struct {
	Timeout time.Duration
}

func New(timeout time.Duration) *Pinger {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Pinger{Timeout: timeout}
}

func (p *Pinger) Ping(host string, port int) (*Status, error) {
	if port == 0 {
		port = DefaultPort
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), p.Timeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(p.Timeout))

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

	status, err := parseStatus(payload)
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	status.LatencyMS = time.Since(start).Milliseconds()
	return status, nil
}

func writeHandshake(conn net.Conn, host string, port int) error {
	var body bytes.Buffer
	_ = writeVarInt(&body, 0x00)
	_ = writeVarInt(&body, protocolVersion)
	_ = writeString(&body, host)
	_ = binary.Write(&body, binary.BigEndian, uint16(port))
	_ = writeVarInt(&body, stateStatus)
	return writeFramed(conn, body.Bytes())
}

func writeStatusRequest(conn net.Conn) error {
	var body bytes.Buffer
	_ = writeVarInt(&body, 0x00)
	return writeFramed(conn, body.Bytes())
}

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
