package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/minasoft/hl7-replicator/internal/config"
	"github.com/minasoft/hl7-replicator/internal/consumers"
	"github.com/minasoft/hl7-replicator/internal/hl7"
	"github.com/minasoft/hl7-replicator/internal/nats"
	"github.com/minasoft/hl7-replicator/internal/web"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Yapılandırma yüklenemedi", "error", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start embedded NATS server
	natsServer, err := nats.NewEmbeddedServer(cfg.DBPath)
	if err != nil {
		slog.Error("NATS sunucu başlatılamadı", "error", err)
		os.Exit(1)
	}
	defer natsServer.Shutdown()

	// Get JetStream context
	js := natsServer.JetStream()

	// Create wait group for goroutines
	var wg sync.WaitGroup

	// Start HL7 MLLP servers
	orderServer := hl7.NewMLLPServer(cfg.OrderListenPort, "order", js)
	if err := orderServer.Start(ctx); err != nil {
		slog.Error("Order sunucu başlatılamadı", "error", err)
		os.Exit(1)
	}
	defer orderServer.Stop()

	reportServer := hl7.NewMLLPServer(cfg.ReportListenPort, "report", js)
	if err := reportServer.Start(ctx); err != nil {
		slog.Error("Report sunucu başlatılamadı", "error", err)
		os.Exit(1)
	}
	defer reportServer.Stop()

	// Start message forwarder consumers
	forwarder := consumers.NewMessageForwarder(js, cfg)
	if err := forwarder.Start(ctx); err != nil {
		slog.Error("Message forwarder başlatılamadı", "error", err)
		os.Exit(1)
	}

	// Start web server
	webServer := web.NewServer(js, cfg)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := webServer.Start(ctx); err != nil {
			slog.Error("Web sunucu hatası", "error", err)
		}
	}()

	slog.Info("HL7 Replicator başlatıldı",
		"orderPort", cfg.OrderListenPort,
		"reportPort", cfg.ReportListenPort,
		"webPort", cfg.WebPort,
		"zenpacsEndpoint", fmt.Sprintf("%s:%d", cfg.ZenPACSHost, cfg.ZenPACSPort),
		"hospitalEndpoint", fmt.Sprintf("%s:%d", cfg.HospitalHISHost, cfg.HospitalHISPort),
	)

	// Print startup information
	printStartupInfo(cfg)

	// Wait for shutdown signal
	<-sigChan
	slog.Info("Kapatma sinyali alındı, sunucu kapatılıyor...")

	// Cancel context to stop all services
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()

	slog.Info("HL7 Replicator kapatıldı")
}

func printStartupInfo(cfg *config.Config) {
	info := `
╔═══════════════════════════════════════════════════════════════╗
║                    HL7 Replicator Başlatıldı                  ║
╠═══════════════════════════════════════════════════════════════╣
║ Order Receiver Port  : %-39d ║
║ Report Receiver Port : %-39d ║
║ Web Dashboard        : http://localhost:%-22d ║
║                                                               ║
║ ZenPACS Endpoint     : %-39s ║
║ Hospital HIS Endpoint: %-39s ║
╚═══════════════════════════════════════════════════════════════╝
`
	zenpacsEndpoint := cfg.ZenPACSHost + ":" + fmt.Sprintf("%d", cfg.ZenPACSPort)
	hospitalEndpoint := cfg.HospitalHISHost + ":" + fmt.Sprintf("%d", cfg.HospitalHISPort)
	
	fmt.Printf(info, 
		cfg.OrderListenPort,
		cfg.ReportListenPort,
		cfg.WebPort,
		zenpacsEndpoint,
		hospitalEndpoint,
	)
}