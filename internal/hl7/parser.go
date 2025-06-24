package hl7

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

const (
	// MLLP frame characters
	StartBlock     = 0x0B
	EndBlock       = 0x1C
	CarriageReturn = 0x0D
)

// ParseMessage parses an HL7 message and extracts key fields
func ParseMessage(data []byte) (map[string]string, error) {
	// Remove MLLP wrapper if present
	data = bytes.TrimPrefix(data, []byte{StartBlock})
	data = bytes.TrimSuffix(data, []byte{EndBlock, CarriageReturn})

	lines := strings.Split(string(data), "\r")
	if len(lines) == 0 {
		return nil, fmt.Errorf("boş mesaj")
	}

	result := make(map[string]string)

	// Parse MSH segment
	if !strings.HasPrefix(lines[0], "MSH") {
		return nil, fmt.Errorf("geçersiz HL7 mesajı: MSH segmenti bulunamadı")
	}

	mshFields := strings.Split(lines[0], "|")
	if len(mshFields) < 12 {
		return nil, fmt.Errorf("eksik MSH alanları")
	}

	result["message_type"] = mshFields[8]
	result["message_control_id"] = mshFields[9]
	result["sending_application"] = mshFields[2]
	result["receiving_application"] = mshFields[4]

	// Parse PID segment if exists
	for _, line := range lines {
		if strings.HasPrefix(line, "PID") {
			pidFields := strings.Split(line, "|")
			if len(pidFields) > 3 {
				result["patient_id"] = pidFields[3]
			}
			if len(pidFields) > 5 {
				// Format: LastName^FirstName^MiddleName
				nameComponents := strings.Split(pidFields[5], "^")
				if len(nameComponents) > 0 {
					result["patient_name"] = strings.Join(nameComponents, " ")
				}
			}
			break
		}
	}

	return result, nil
}

// CreateACK creates an HL7 ACK message
func CreateACK(originalMessage []byte, ackCode string) []byte {
	parsed, _ := ParseMessage(originalMessage)

	timestamp := time.Now().Format("20060102150405")
	messageControlID := parsed["message_control_id"]
	if messageControlID == "" {
		messageControlID = fmt.Sprintf("ACK%d", time.Now().Unix())
	}

	ack := fmt.Sprintf("MSH|^~\\&|HL7_REPLICATOR|MINASOFT|%s|%s|%s||ACK^%s|%s|P|2.5\rMSA|%s|%s",
		parsed["sending_application"],
		parsed["sending_facility"],
		timestamp,
		parsed["message_type"],
		messageControlID,
		ackCode,
		parsed["message_control_id"])

	// Add MLLP wrapper
	return append([]byte{StartBlock}, append([]byte(ack+"\r"), EndBlock, CarriageReturn)...)
}

// WrapMLLP adds MLLP wrapper to message
func WrapMLLP(message []byte) []byte {
	if len(message) == 0 {
		return message
	}

	// Check if already wrapped
	if message[0] == StartBlock {
		return message
	}

	return append([]byte{StartBlock}, append(message, EndBlock, CarriageReturn)...)
}

// UnwrapMLLP removes MLLP wrapper from message
func UnwrapMLLP(message []byte) []byte {
	message = bytes.TrimPrefix(message, []byte{StartBlock})
	message = bytes.TrimSuffix(message, []byte{EndBlock, CarriageReturn})
	return message
}
