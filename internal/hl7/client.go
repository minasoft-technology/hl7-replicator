package hl7

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"time"
)

type MLLPClient struct {
	host string
	port int
	timeout time.Duration
}

func NewMLLPClient(host string, port int) *MLLPClient {
	return &MLLPClient{
		host: host,
		port: port,
		timeout: 30 * time.Second,
	}
}

func (c *MLLPClient) SendMessage(message []byte) error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	
	// Connect to server
	conn, err := net.DialTimeout("tcp", addr, c.timeout)
	if err != nil {
		return fmt.Errorf("bağlantı hatası %s: %w", addr, err)
	}
	defer conn.Close()
	
	slog.Debug("HL7 sunucusuna bağlandı", "address", addr)
	
	// Wrap message with MLLP if not already wrapped
	wrappedMessage := WrapMLLP(message)
	
	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(c.timeout))
	
	// Send message
	_, err = conn.Write(wrappedMessage)
	if err != nil {
		return fmt.Errorf("mesaj gönderme hatası: %w", err)
	}
	
	slog.Debug("HL7 mesaj gönderildi", "size", len(wrappedMessage))
	
	// Set read deadline for ACK
	conn.SetReadDeadline(time.Now().Add(c.timeout))
	
	// Read ACK
	reader := bufio.NewReader(conn)
	ack, err := c.readMLLPMessage(reader)
	if err != nil {
		return fmt.Errorf("ACK okuma hatası: %w", err)
	}
	
	// Parse ACK
	ackParsed, err := ParseMessage(ack)
	if err != nil {
		return fmt.Errorf("ACK parse hatası: %w", err)
	}
	
	// Check ACK code
	ackCode := c.extractACKCode(ack)
	if ackCode != "AA" && ackCode != "CA" {
		return fmt.Errorf("negatif ACK alındı: %s", ackCode)
	}
	
	slog.Info("HL7 mesaj başarıyla gönderildi",
		"address", addr,
		"messageControlID", ackParsed["message_control_id"],
		"ackCode", ackCode)
	
	return nil
}

func (c *MLLPClient) readMLLPMessage(reader *bufio.Reader) ([]byte, error) {
	// Wait for start block
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == StartBlock {
			break
		}
	}
	
	// Read until end block
	var buffer bytes.Buffer
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		
		if b == EndBlock {
			// Read carriage return
			cr, err := reader.ReadByte()
			if err != nil {
				return nil, err
			}
			if cr != CarriageReturn {
				return nil, fmt.Errorf("MLLP formatı hatası: CR beklendi, %02X alındı", cr)
			}
			break
		}
		
		buffer.WriteByte(b)
	}
	
	return buffer.Bytes(), nil
}

func (c *MLLPClient) extractACKCode(ack []byte) string {
	// Simple MSA segment parser for ACK code
	lines := bytes.Split(ack, []byte("\r"))
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("MSA")) {
			fields := bytes.Split(line, []byte("|"))
			if len(fields) > 1 {
				return string(fields[1])
			}
		}
	}
	return ""
}

// TestConnection tests if the HL7 server is reachable
func (c *MLLPClient) TestConnection() error {
	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("bağlantı testi başarısız %s: %w", addr, err)
	}
	conn.Close()
	return nil
}