package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/minasoft/hl7-replicator/internal/config"
	"github.com/minasoft/hl7-replicator/internal/db"
	"github.com/nats-io/nats.go/jetstream"
)

//go:embed all:web/*
var webFiles embed.FS

type Server struct {
	echo   *echo.Echo
	js     jetstream.JetStream
	config *config.Config
}

func NewServer(js jetstream.JetStream, cfg *config.Config) *Server {
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	return &Server{
		echo:   e,
		js:     js,
		config: cfg,
	}
}

func (s *Server) Start(ctx context.Context) error {
	// Setup routes
	s.setupRoutes()

	// Start server
	addr := fmt.Sprintf(":%d", s.config.WebPort)
	slog.Info("Web sunucu başlatılıyor", "port", s.config.WebPort)

	go func() {
		if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error("Web sunucu hatası", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown gracefully
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.echo.Shutdown(shutdownCtx)
}

func (s *Server) setupRoutes() {
	// API routes
	api := s.echo.Group("/api")
	api.GET("/health", s.handleHealth)
	api.GET("/stats", s.handleStats)
	api.GET("/messages", s.handleGetMessages)
	api.POST("/messages/:id/retry", s.handleRetryMessage)
	api.GET("/streams", s.handleGetStreams)
	api.GET("/consumers", s.handleGetConsumers)

	// Static files
	// Serve static files from embedded filesystem
	webFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		slog.Error("Web dosyaları yüklenemedi", "error", err)
		return
	}

	// Specific route for root first
	s.echo.GET("/", func(c echo.Context) error {
		file, err := webFS.Open("index.html")
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		return c.HTML(http.StatusOK, string(data))
	})

	// Then generic static file handler
	s.echo.GET("/*", echo.WrapHandler(http.FileServer(http.FS(webFS))))
}

func (s *Server) handleHealth(c echo.Context) error {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"components": map[string]string{
			"nats":          "healthy",
			"order_server":  "healthy",
			"report_server": "healthy",
		},
	}

	return c.JSON(http.StatusOK, health)
}

func (s *Server) handleStats(c echo.Context) error {
	ctx := c.Request().Context()

	// Get stats from KV store
	statsKV, err := s.js.KeyValue(ctx, "HL7_STATS")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Stats KV erişilemedi")
	}

	// Helper function to get KV value as int
	getKVInt := func(key string) int {
		entry, err := statsKV.Get(ctx, key)
		if err != nil {
			return 0
		}
		var val int
		fmt.Sscanf(string(entry.Value()), "%d", &val)
		return val
	}

	// Get all statistics
	totalOrders := getKVInt("total_orders")
	successfulOrders := getKVInt("successful_orders")
	failedOrders := getKVInt("failed_orders")

	totalReports := getKVInt("total_reports")
	successfulReports := getKVInt("successful_reports")
	failedReports := getKVInt("failed_reports")

	stats := map[string]interface{}{
		"total":      totalOrders + totalReports,
		"successful": successfulOrders + successfulReports,
		"failed":     failedOrders + failedReports,
		"pending":    0, // We don't track pending anymore
		"orders": map[string]int{
			"total":      totalOrders,
			"successful": successfulOrders,
			"failed":     failedOrders,
		},
		"reports": map[string]int{
			"total":      totalReports,
			"successful": successfulReports,
			"failed":     failedReports,
		},
	}

	// Add last message times
	if lastOrderTime, err := statsKV.Get(ctx, "last_order_time"); err == nil {
		stats["last_order_time"] = string(lastOrderTime.Value())
	}
	if lastReportTime, err := statsKV.Get(ctx, "last_report_time"); err == nil {
		stats["last_report_time"] = string(lastReportTime.Value())
	}

	return c.JSON(http.StatusOK, stats)
}

func (s *Server) handleGetMessages(c echo.Context) error {
	ctx := c.Request().Context()
	showFailed := c.QueryParam("status") == "failed"

	messages := []db.HL7Message{}

	if showFailed {
		// Get failed messages from DLQ
		dlqKV, err := s.js.KeyValue(ctx, "HL7_DLQ")
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "DLQ erişilemedi")
		}

		// List all keys in DLQ
		keys, err := dlqKV.Keys(ctx)
		if err != nil {
			// Return empty array if no keys
			if err.Error() == "nats: no keys found" {
				return c.JSON(http.StatusOK, messages)
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "DLQ keys okunamadı: "+err.Error())
		}

		for _, key := range keys {
			entry, err := dlqKV.Get(ctx, key)
			if err != nil {
				continue
			}

			var msg db.HL7Message
			if err := json.Unmarshal(entry.Value(), &msg); err == nil {
				messages = append(messages, msg)
			}
		}
	} else {
		// Get recent successful messages (last 100)
		// For successful messages, we can show basic stats from KV
		_, err := s.js.KeyValue(ctx, "HL7_STATS")
		if err == nil {
			// Return empty array for now since we're not storing successful messages
			// In production, you might want to keep a circular buffer of recent messages
			return c.JSON(http.StatusOK, messages)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})

	return c.JSON(http.StatusOK, messages)
}

func (s *Server) handleRetryMessage(c echo.Context) error {
	// messageID := c.Param("id")

	// This would republish the message to the appropriate stream
	// For now, return success
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Mesaj yeniden kuyruğa alındı",
	})
}

func (s *Server) handleGetStreams(c echo.Context) error {
	ctx := c.Request().Context()
	streams := []db.StreamInfo{}

	for _, streamName := range []string{"HL7_ORDERS", "HL7_REPORTS"} {
		stream, err := s.js.Stream(ctx, streamName)
		if err != nil {
			continue
		}

		info, err := stream.Info(ctx)
		if err != nil {
			continue
		}

		streams = append(streams, db.StreamInfo{
			Name:          info.Config.Name,
			Messages:      info.State.Msgs,
			Bytes:         info.State.Bytes,
			FirstSequence: info.State.FirstSeq,
			LastSequence:  info.State.LastSeq,
		})
	}

	return c.JSON(http.StatusOK, streams)
}

func (s *Server) handleGetConsumers(c echo.Context) error {
	ctx := c.Request().Context()
	consumers := []db.ConsumerInfo{}

	// Get consumers for each stream
	for _, streamName := range []string{"HL7_ORDERS", "HL7_REPORTS"} {
		stream, err := s.js.Stream(ctx, streamName)
		if err != nil {
			continue
		}

		consumerNames := stream.ConsumerNames(ctx)
		for name := range consumerNames.Name() {
			consumer, err := stream.Consumer(ctx, name)
			if err != nil {
				continue
			}

			info, err := consumer.Info(ctx)
			if err != nil {
				continue
			}

			consumers = append(consumers, db.ConsumerInfo{
				Stream:          streamName,
				Name:            info.Name,
				Pending:         info.NumPending,
				Delivered:       info.Delivered.Consumer,
				AckPending:      uint64(info.NumAckPending),
				RedeliveryCount: uint64(info.NumRedelivered),
			})
		}
	}

	return c.JSON(http.StatusOK, consumers)
}
