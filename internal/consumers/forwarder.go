package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/minasoft/hl7-replicator/internal/config"
	"github.com/minasoft/hl7-replicator/internal/db"
	"github.com/minasoft/hl7-replicator/internal/hl7"
	"github.com/nats-io/nats.go/jetstream"
)

type MessageForwarder struct {
	js        jetstream.JetStream
	config    *config.Config
	statsKV   jetstream.KeyValue
	dlqKV     jetstream.KeyValue
	historyKV jetstream.KeyValue
	statsMu   sync.Mutex
}

func NewMessageForwarder(js jetstream.JetStream, cfg *config.Config) *MessageForwarder {
	ctx := context.Background()

	// Get stats KV store
	statsKV, err := js.KeyValue(ctx, "HL7_STATS")
	if err != nil {
		slog.Error("Stats KV store erişilemedi", "error", err)
	}

	// Get DLQ KV store
	dlqKV, err := js.KeyValue(ctx, "HL7_DLQ")
	if err != nil {
		slog.Error("DLQ KV store erişilemedi", "error", err)
	}

	// Get History KV store
	historyKV, err := js.KeyValue(ctx, "HL7_HISTORY")
	if err != nil {
		slog.Error("History KV store erişilemedi", "error", err)
	}

	return &MessageForwarder{
		js:        js,
		config:    cfg,
		statsKV:   statsKV,
		dlqKV:     dlqKV,
		historyKV: historyKV,
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
		Name:          "order-forwarder",
		Description:   "HIS'ten ZenPACS'a order mesajlarını ileten consumer",
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
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
		Name:          "report-forwarder",
		Description:   "ZenPACS'tan HIS'e rapor mesajlarını ileten consumer",
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
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

	// Get message metadata to check redelivery count
	meta, _ := msg.Metadata()

	slog.Info("Order mesajı işleniyor",
		"id", hl7Msg.ID,
		"messageType", hl7Msg.MessageType,
		"patientID", hl7Msg.PatientID,
		"deliveryAttempt", meta.NumDelivered)

	// Forward message
	err := client.SendMessage(hl7Msg.RawMessage)
	if err != nil {
		hl7Msg.Status = "failed"
		hl7Msg.LastError = err.Error()
		if meta != nil {
			hl7Msg.RetryCount = int(meta.NumDelivered)
		}

		slog.Error("Order mesaj gönderme hatası",
			"id", hl7Msg.ID,
			"error", err,
			"deliveryAttempt", meta.NumDelivered)

		// Update statistics
		if f.statsKV != nil {
			// Only count as a new message on first attempt
			if meta != nil && meta.NumDelivered == 1 {
				f.incrementKVCounter("total_orders")
				f.incrementKVCounter("failed_orders")
			}
		}

		// Save to DLQ after max retries
		if meta != nil && meta.NumDelivered >= 5 && f.dlqKV != nil {
			hl7Msg.Direction = "order"
			dlqKey := fmt.Sprintf("order_%s_%d", hl7Msg.ID, time.Now().Unix())
			dlqData, _ := json.Marshal(hl7Msg)
			f.dlqKV.Put(context.Background(), dlqKey, dlqData)
			slog.Warn("Mesaj DLQ'ya kaydedildi", "id", hl7Msg.ID, "key", dlqKey, "attempts", meta.NumDelivered)
			// Save to history
			f.saveToHistory(&hl7Msg)
			// ACK to remove from stream after saving to DLQ
			msg.Ack()
			return
		}

		// NACK for retry
		msg.Nak()
		return
	}

	// Success
	hl7Msg.Status = "forwarded"
	now := time.Now()
	hl7Msg.ProcessedAt = &now
	hl7Msg.Direction = "order"

	// Update KV statistics
	if f.statsKV != nil {
		f.incrementKVCounter("total_orders")
		f.incrementKVCounter("successful_orders")
		f.statsKV.Put(context.Background(), "last_order_time", []byte(now.Format(time.RFC3339)))
	}

	slog.Info("Order mesajı başarıyla gönderildi",
		"id", hl7Msg.ID,
		"destination", fmt.Sprintf("%s:%d", f.config.ZenPACSHost, f.config.ZenPACSPort))

	// Save to history
	f.saveToHistory(&hl7Msg)

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

	// Get message metadata to check redelivery count
	meta, _ := msg.Metadata()

	slog.Info("Report mesajı işleniyor",
		"id", hl7Msg.ID,
		"messageType", hl7Msg.MessageType,
		"patientID", hl7Msg.PatientID,
		"deliveryAttempt", meta.NumDelivered)

	// Forward message
	err := client.SendMessage(hl7Msg.RawMessage)
	if err != nil {
		hl7Msg.Status = "failed"
		hl7Msg.LastError = err.Error()
		if meta != nil {
			hl7Msg.RetryCount = int(meta.NumDelivered)
		}

		slog.Error("Report mesaj gönderme hatası",
			"id", hl7Msg.ID,
			"error", err,
			"deliveryAttempt", meta.NumDelivered)

		// Update statistics
		if f.statsKV != nil {
			// Only count as a new message on first attempt
			if meta != nil && meta.NumDelivered == 1 {
				f.incrementKVCounter("total_reports")
				f.incrementKVCounter("failed_reports")
			}
		}

		// Save to DLQ after max retries
		if meta != nil && meta.NumDelivered >= 5 && f.dlqKV != nil {
			hl7Msg.Direction = "report"
			dlqKey := fmt.Sprintf("report_%s_%d", hl7Msg.ID, time.Now().Unix())
			dlqData, _ := json.Marshal(hl7Msg)
			f.dlqKV.Put(context.Background(), dlqKey, dlqData)
			slog.Warn("Mesaj DLQ'ya kaydedildi", "id", hl7Msg.ID, "key", dlqKey, "attempts", meta.NumDelivered)
			// Save to history
			f.saveToHistory(&hl7Msg)
			// ACK to remove from stream after saving to DLQ
			msg.Ack()
			return
		}

		// NACK for retry
		msg.Nak()
		return
	}

	// Success
	hl7Msg.Status = "forwarded"
	now := time.Now()
	hl7Msg.ProcessedAt = &now
	hl7Msg.Direction = "report"

	// Update KV statistics
	if f.statsKV != nil {
		f.incrementKVCounter("total_reports")
		f.incrementKVCounter("successful_reports")
		f.statsKV.Put(context.Background(), "last_report_time", []byte(now.Format(time.RFC3339)))
	}

	slog.Info("Report mesajı başarıyla gönderildi",
		"id", hl7Msg.ID,
		"destination", fmt.Sprintf("%s:%d", f.config.HospitalHISHost, f.config.HospitalHISPort))

	// Save to history
	f.saveToHistory(&hl7Msg)

	// ACK message
	msg.Ack()
}

func (f *MessageForwarder) saveToHistory(msg *db.HL7Message) {
	if f.historyKV == nil {
		return
	}

	// Create unique key with timestamp
	key := fmt.Sprintf("%s_%s_%d", msg.Direction, msg.ID, time.Now().UnixNano())

	// Marshal message
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("Mesaj history'ye kaydedilemedi", "error", err, "id", msg.ID)
		return
	}

	// Save to history
	ctx := context.Background()
	if _, err := f.historyKV.Put(ctx, key, data); err != nil {
		slog.Error("History KV put hatası", "error", err, "key", key)
	}
}

func (f *MessageForwarder) incrementKVCounter(key string) {
	if f.statsKV == nil {
		return
	}

	// Lock for thread-safe counter increment
	f.statsMu.Lock()
	defer f.statsMu.Unlock()

	ctx := context.Background()
	entry, err := f.statsKV.Get(ctx, key)
	if err != nil {
		// If key doesn't exist, start from 0
		if err.Error() == "nats: key not found" {
			f.statsKV.Put(ctx, key, []byte("1"))
		}
		return
	}

	// Parse current value
	currentVal := 0
	fmt.Sscanf(string(entry.Value()), "%d", &currentVal)

	// Increment and store
	newVal := fmt.Sprintf("%d", currentVal+1)
	f.statsKV.Put(ctx, key, []byte(newVal))
}
