package nats

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type EmbeddedServer struct {
	server *server.Server
	nc     *nats.Conn
	js     jetstream.JetStream
}

func NewEmbeddedServer(dataDir string) (*EmbeddedServer, error) {
	// NATS sunucu ayarları
	opts := &server.Options{
		JetStream: true,
		StoreDir:  filepath.Join(dataDir, "nats-store"),
		Port:      -1, // Random port, sadece internal kullanım
		HTTPPort:  -1, // HTTP monitoring kapalı
	}

	// Store dizinini oluştur
	if err := os.MkdirAll(opts.StoreDir, 0755); err != nil {
		return nil, fmt.Errorf("store dizini oluşturulamadı: %w", err)
	}

	// NATS sunucusunu başlat
	ns, err := server.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("NATS sunucu oluşturulamadı: %w", err)
	}

	// Sunucuyu başlat
	ns.Start()
	
	// Hazır olmasını bekle
	if !ns.ReadyForConnections(10 * time.Second) {
		return nil, fmt.Errorf("NATS sunucu başlatılamadı")
	}

	slog.Info("Gömülü NATS sunucu başlatıldı", "clientURL", ns.ClientURL())

	// Client bağlantısı oluştur
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		ns.Shutdown()
		return nil, fmt.Errorf("NATS bağlantısı kurulamadı: %w", err)
	}

	// JetStream context oluştur
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		ns.Shutdown()
		return nil, fmt.Errorf("JetStream başlatılamadı: %w", err)
	}

	es := &EmbeddedServer{
		server: ns,
		nc:     nc,
		js:     js,
	}

	// Stream'leri oluştur
	if err := es.createStreams(); err != nil {
		es.Shutdown()
		return nil, err
	}

	return es, nil
}

func (es *EmbeddedServer) createStreams() error {
	// Order stream (HIS -> ZenPACS)
	orderStreamConfig := jetstream.StreamConfig{
		Name:        "HL7_ORDERS",
		Description: "Hastane HIS'ten gelen order mesajları",
		Subjects:    []string{"hl7.orders.>"},
		Retention:   jetstream.WorkQueuePolicy,
		MaxAge:      30 * 24 * time.Hour, // 30 gün
		Storage:     jetstream.FileStorage,
		Replicas:    1,
		MaxMsgs:     1000000,
		MaxBytes:    10 * 1024 * 1024 * 1024, // 10GB
	}

	_, err := es.js.CreateOrUpdateStream(context.Background(), orderStreamConfig)
	if err != nil {
		return fmt.Errorf("order stream oluşturulamadı: %w", err)
	}
	slog.Info("HL7_ORDERS stream oluşturuldu")

	// Report stream (ZenPACS -> HIS)
	reportStreamConfig := jetstream.StreamConfig{
		Name:        "HL7_REPORTS",
		Description: "ZenPACS'tan gelen rapor mesajları",
		Subjects:    []string{"hl7.reports.>"},
		Retention:   jetstream.WorkQueuePolicy,
		MaxAge:      30 * 24 * time.Hour, // 30 gün
		Storage:     jetstream.FileStorage,
		Replicas:    1,
		MaxMsgs:     1000000,
		MaxBytes:    10 * 1024 * 1024 * 1024, // 10GB
	}

	_, err = es.js.CreateOrUpdateStream(context.Background(), reportStreamConfig)
	if err != nil {
		return fmt.Errorf("report stream oluşturulamadı: %w", err)
	}
	slog.Info("HL7_REPORTS stream oluşturuldu")

	return nil
}

func (es *EmbeddedServer) JetStream() jetstream.JetStream {
	return es.js
}

func (es *EmbeddedServer) Connection() *nats.Conn {
	return es.nc
}

func (es *EmbeddedServer) Shutdown() {
	if es.nc != nil {
		es.nc.Close()
	}
	if es.server != nil {
		es.server.Shutdown()
		es.server.WaitForShutdown()
	}
	slog.Info("NATS sunucu kapatıldı")
}