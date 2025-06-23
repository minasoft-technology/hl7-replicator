package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/minasoft/hl7-replicator/internal/config"
	"github.com/minasoft/hl7-replicator/internal/db"
	"github.com/minasoft/hl7-replicator/internal/hl7"
	"github.com/nats-io/nats.go/jetstream"
)

type MessageForwarder struct {
	js     jetstream.JetStream
	config *config.Config
}

func NewMessageForwarder(js jetstream.JetStream, cfg *config.Config) *MessageForwarder {
	return &MessageForwarder{
		js:     js,
		config: cfg,
	}
}

func (f *MessageForwarder) Start(ctx context.Context) error {
	// Start order consumer (HIS -> ZenPACS)
	if err := f.startOrderConsumer(ctx); err != nil {
		return fmt.Errorf("order consumer başlatılamadı: %w", err)
	}
	
	// Start report consumer (ZenPACS -> HIS)
	if err := f.startReportConsumer(ctx); err != nil {
		return fmt.Errorf("report consumer başlatılamadı: %w", err)
	}
	
	return nil
}

func (f *MessageForwarder) startOrderConsumer(ctx context.Context) error {
	consumer, err := f.js.CreateOrUpdateConsumer(ctx, "HL7_ORDERS", jetstream.ConsumerConfig{
		Name:        "order-forwarder",
		Description: "HIS'ten ZenPACS'a order mesajlarını ileten consumer",
		MaxDeliver:  5,
		AckWait:     30 * time.Second,
		MaxAckPending: 100,
	})
	if err != nil {
		return err
	}
	
	// Create ZenPACS client
	zenpacsClient := hl7.NewMLLPClient(f.config.ZenPACSHost, f.config.ZenPACSPort)
	
	// Start consuming
	go func() {
		slog.Info("Order forwarder başlatıldı", 
			"stream", "HL7_ORDERS",
			"destination", fmt.Sprintf("%s:%d", f.config.ZenPACSHost, f.config.ZenPACSPort))
		
		cons, err := consumer.Consume(func(msg jetstream.Msg) {
			// Process message
			f.processOrderMessage(msg, zenpacsClient)
		})
		if err != nil {
			slog.Error("Consumer hatası", "error", err)
			return
		}
		
		// Wait for context cancellation
		<-ctx.Done()
		cons.Stop()
	}()
	
	return nil
}

func (f *MessageForwarder) startReportConsumer(ctx context.Context) error {
	consumer, err := f.js.CreateOrUpdateConsumer(ctx, "HL7_REPORTS", jetstream.ConsumerConfig{
		Name:        "report-forwarder",
		Description: "ZenPACS'tan HIS'e rapor mesajlarını ileten consumer",
		MaxDeliver:  5,
		AckWait:     30 * time.Second,
		MaxAckPending: 100,
	})
	if err != nil {
		return err
	}
	
	// Create HIS client
	hisClient := hl7.NewMLLPClient(f.config.HospitalHISHost, f.config.HospitalHISPort)
	
	// Start consuming
	go func() {
		slog.Info("Report forwarder başlatıldı",
			"stream", "HL7_REPORTS", 
			"destination", fmt.Sprintf("%s:%d", f.config.HospitalHISHost, f.config.HospitalHISPort))
		
		cons, err := consumer.Consume(func(msg jetstream.Msg) {
			// Process message
			f.processReportMessage(msg, hisClient)
		})
		if err != nil {
			slog.Error("Consumer hatası", "error", err)
			return
		}
		
		// Wait for context cancellation
		<-ctx.Done()
		cons.Stop()
	}()
	
	return nil
}

func (f *MessageForwarder) processOrderMessage(msg jetstream.Msg, client *hl7.MLLPClient) {
	// Parse message
	var hl7Msg db.HL7Message
	if err := json.Unmarshal(msg.Data(), &hl7Msg); err != nil {
		slog.Error("Mesaj parse hatası", "error", err)
		msg.Nak()
		return
	}
	
	slog.Info("Order mesajı işleniyor",
		"id", hl7Msg.ID,
		"messageType", hl7Msg.MessageType,
		"patientID", hl7Msg.PatientID)
	
	// Forward message
	err := client.SendMessage(hl7Msg.RawMessage)
	if err != nil {
		hl7Msg.Status = "failed"
		hl7Msg.LastError = err.Error()
		hl7Msg.RetryCount++
		
		slog.Error("Order mesaj gönderme hatası",
			"id", hl7Msg.ID,
			"error", err,
			"retryCount", hl7Msg.RetryCount)
		
		// Update message in stream
		updatedData, _ := json.Marshal(hl7Msg)
		f.js.Publish(context.Background(), msg.Subject(), updatedData)
		
		// NACK for retry
		msg.Nak()
		return
	}
	
	// Success
	hl7Msg.Status = "forwarded"
	now := time.Now()
	hl7Msg.ProcessedAt = &now
	
	slog.Info("Order mesajı başarıyla gönderildi",
		"id", hl7Msg.ID,
		"destination", fmt.Sprintf("%s:%d", f.config.ZenPACSHost, f.config.ZenPACSPort))
	
	// ACK message
	msg.Ack()
}

func (f *MessageForwarder) processReportMessage(msg jetstream.Msg, client *hl7.MLLPClient) {
	// Parse message
	var hl7Msg db.HL7Message
	if err := json.Unmarshal(msg.Data(), &hl7Msg); err != nil {
		slog.Error("Mesaj parse hatası", "error", err)
		msg.Nak()
		return
	}
	
	slog.Info("Report mesajı işleniyor",
		"id", hl7Msg.ID,
		"messageType", hl7Msg.MessageType,
		"patientID", hl7Msg.PatientID)
	
	// Forward message
	err := client.SendMessage(hl7Msg.RawMessage)
	if err != nil {
		hl7Msg.Status = "failed"
		hl7Msg.LastError = err.Error()
		hl7Msg.RetryCount++
		
		slog.Error("Report mesaj gönderme hatası",
			"id", hl7Msg.ID,
			"error", err,
			"retryCount", hl7Msg.RetryCount)
		
		// Update message in stream
		updatedData, _ := json.Marshal(hl7Msg)
		f.js.Publish(context.Background(), msg.Subject(), updatedData)
		
		// NACK for retry
		msg.Nak()
		return
	}
	
	// Success
	hl7Msg.Status = "forwarded"
	now := time.Now()
	hl7Msg.ProcessedAt = &now
	
	slog.Info("Report mesajı başarıyla gönderildi",
		"id", hl7Msg.ID,
		"destination", fmt.Sprintf("%s:%d", f.config.HospitalHISHost, f.config.HospitalHISPort))
	
	// ACK message
	msg.Ack()
}