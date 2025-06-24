package hl7

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/minasoft/hl7-replicator/internal/db"
	"github.com/nats-io/nats.go/jetstream"
)

type MLLPServer struct {
	port      int
	direction string // "order" or "report"
	js        jetstream.JetStream
	listener  net.Listener
}

func NewMLLPServer(port int, direction string, js jetstream.JetStream) *MLLPServer {
	return &MLLPServer{
		port:      port,
		direction: direction,
		js:        js,
	}
}

func (s *MLLPServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port dinlenemedi %s: %w", addr, err)
	}
	s.listener = listener

	slog.Info("HL7 MLLP sunucu başlatıldı",
		"port", s.port,
		"direction", s.direction,
		"address", addr)

	go s.acceptConnections(ctx)
	return nil
}

func (s *MLLPServer) acceptConnections(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("Bağlantı kabul hatası", "error", err)
				continue
			}

			go s.handleConnection(ctx, conn)
		}
	}
}

func (s *MLLPServer) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	slog.Info("Yeni HL7 bağlantısı", "remoteAddr", remoteAddr, "direction", s.direction)

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Set read timeout
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			// Read MLLP message
			message, err := s.readMLLPMessage(reader)
			if err != nil {
				if err == io.EOF {
					slog.Info("Bağlantı kapatıldı", "remoteAddr", remoteAddr)
					return
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				slog.Error("Mesaj okuma hatası", "error", err, "remoteAddr", remoteAddr)
				return
			}

			// Process message
			if err := s.processMessage(message, remoteAddr); err != nil {
				slog.Error("Mesaj işleme hatası", "error", err)
				// Send NACK
				conn.Write(CreateACK(message, "AE"))
			} else {
				// Send ACK
				conn.Write(CreateACK(message, "AA"))
			}
		}
	}
}

func (s *MLLPServer) readMLLPMessage(reader *bufio.Reader) ([]byte, error) {
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

func (s *MLLPServer) processMessage(rawMessage []byte, sourceAddr string) error {
	// Parse HL7 message
	parsed, err := ParseMessage(rawMessage)
	if err != nil {
		return fmt.Errorf("mesaj parse hatası: %w", err)
	}

	// Create message object
	msg := &db.HL7Message{
		ID:               uuid.New().String(),
		Timestamp:        time.Now(),
		Direction:        s.direction,
		SourceAddr:       sourceAddr,
		MessageType:      parsed["message_type"],
		MessageControlID: parsed["message_control_id"],
		PatientID:        parsed["patient_id"],
		PatientName:      parsed["patient_name"],
		RawMessage:       rawMessage,
		Status:           "pending",
		CreatedAt:        time.Now(),
	}

	// Set destination based on direction
	if s.direction == "order" {
		msg.DestinationAddr = "194.187.253.34:2575" // ZenPACS
	} else {
		// This will be configured from environment
		msg.DestinationAddr = "hospital_his"
	}

	// Publish to NATS JetStream
	subject := fmt.Sprintf("hl7.%ss.%s", s.direction, msg.ID)

	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("mesaj serialize hatası: %w", err)
	}

	_, err = s.js.Publish(context.Background(), subject, msgData)
	if err != nil {
		return fmt.Errorf("NATS publish hatası: %w", err)
	}

	slog.Info("HL7 mesaj alındı ve kuyruğa eklendi",
		"id", msg.ID,
		"direction", s.direction,
		"messageType", msg.MessageType,
		"patientID", msg.PatientID,
		"source", sourceAddr)

	return nil
}

func (s *MLLPServer) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
