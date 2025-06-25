package web

import (
	"bytes"
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
	ctx := c.Request().Context()
	components := make(map[string]string)
	overallStatus := "healthy"

	// Check NATS JetStream
	if s.js != nil {
		_, err := s.js.AccountInfo(ctx)
		if err != nil {
			components["nats"] = "unhealthy: " + err.Error()
			overallStatus = "degraded"
		} else {
			components["nats"] = "healthy"
		}
	} else {
		components["nats"] = "unhealthy: not initialized"
		overallStatus = "unhealthy"
	}

	// Check MLLP servers by checking if streams exist
	orderStream, err := s.js.Stream(ctx, "HL7_ORDERS")
	if err != nil {
		components["order_server"] = "unhealthy: stream not found"
		overallStatus = "degraded"
	} else {
		info, _ := orderStream.Info(ctx)
		if info != nil {
			components["order_server"] = fmt.Sprintf("healthy (messages: %d)", info.State.Msgs)
		} else {
			components["order_server"] = "healthy"
		}
	}

	reportStream, err := s.js.Stream(ctx, "HL7_REPORTS")
	if err != nil {
		components["report_server"] = "unhealthy: stream not found"
		overallStatus = "degraded"
	} else {
		info, _ := reportStream.Info(ctx)
		if info != nil {
			components["report_server"] = fmt.Sprintf("healthy (messages: %d)", info.State.Msgs)
		} else {
			components["report_server"] = "healthy"
		}
	}

	// Check KV stores
	statsKV, err := s.js.KeyValue(ctx, "HL7_STATS")
	if err != nil {
		components["stats_store"] = "unhealthy"
		overallStatus = "degraded"
	} else {
		status, _ := statsKV.Status(ctx)
		if status != nil {
			components["stats_store"] = fmt.Sprintf("healthy (values: %d)", status.Values())
		} else {
			components["stats_store"] = "healthy"
		}
	}

	dlqKV, err := s.js.KeyValue(ctx, "HL7_DLQ")
	if err != nil {
		components["dlq_store"] = "unhealthy"
	} else {
		status, _ := dlqKV.Status(ctx)
		if status != nil {
			components["dlq_store"] = fmt.Sprintf("healthy (failed messages: %d)", status.Values())
		} else {
			components["dlq_store"] = "healthy"
		}
	}

	historyKV, err := s.js.KeyValue(ctx, "HL7_HISTORY")
	if err != nil {
		components["history_store"] = "unhealthy"
	} else {
		status, _ := historyKV.Status(ctx)
		if status != nil {
			components["history_store"] = fmt.Sprintf("healthy (messages: %d)", status.Values())
		} else {
			components["history_store"] = "healthy"
		}
	}

	health := map[string]interface{}{
		"status":     overallStatus,
		"timestamp":  time.Now(),
		"components": components,
		"version":    "1.0.0",
	}

	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	return c.JSON(statusCode, health)
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

	// Query parameters for filtering
	status := c.QueryParam("status")
	direction := c.QueryParam("direction")
	patientID := c.QueryParam("patientId")
	messageType := c.QueryParam("messageType")
	limit := 100 // Default limit

	messages := []db.HL7Message{}

	// Get messages from history (all messages)
	historyKV, err := s.js.KeyValue(ctx, "HL7_HISTORY")
	if err == nil {
		keys, err := historyKV.Keys(ctx)
		if err == nil {
			for _, key := range keys {
				entry, err := historyKV.Get(ctx, key)
				if err != nil {
					continue
				}

				var msg db.HL7Message
				if err := json.Unmarshal(entry.Value(), &msg); err == nil {
					// Apply filters
					if status != "" && msg.Status != status {
						continue
					}
					if direction != "" && msg.Direction != direction {
						continue
					}
					if patientID != "" && (msg.PatientID == "" || !contains(msg.PatientID, patientID)) {
						continue
					}
					if messageType != "" && (msg.MessageType == "" || !contains(msg.MessageType, messageType)) {
						continue
					}

					messages = append(messages, msg)
				}
			}
		}
	}

	// Also get failed messages from DLQ if showing all or failed status
	if status == "" || status == "failed" {
		dlqKV, err := s.js.KeyValue(ctx, "HL7_DLQ")
		if err == nil {
			keys, err := dlqKV.Keys(ctx)
			if err == nil {
				for _, key := range keys {
					entry, err := dlqKV.Get(ctx, key)
					if err != nil {
						continue
					}

					var msg db.HL7Message
					if err := json.Unmarshal(entry.Value(), &msg); err == nil {
						// Apply filters
						if direction != "" && msg.Direction != direction {
							continue
						}
						if patientID != "" && (msg.PatientID == "" || !contains(msg.PatientID, patientID)) {
							continue
						}
						if messageType != "" && (msg.MessageType == "" || !contains(msg.MessageType, messageType)) {
							continue
						}

						// Check if already in messages (from history)
						found := false
						for _, m := range messages {
							if m.ID == msg.ID && m.Timestamp.Equal(msg.Timestamp) {
								found = true
								break
							}
						}
						if !found {
							messages = append(messages, msg)
						}
					}
				}
			}
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})

	// Apply limit
	if len(messages) > limit {
		messages = messages[:limit]
	}

	return c.JSON(http.StatusOK, messages)
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		bytes.Contains(bytes.ToLower([]byte(s)), bytes.ToLower([]byte(substr)))
}

func (s *Server) handleRetryMessage(c echo.Context) error {
	ctx := c.Request().Context()
	messageID := c.Param("id")

	// Find the message in DLQ
	dlqKV, err := s.js.KeyValue(ctx, "HL7_DLQ")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "DLQ erişilemedi")
	}

	// Try to find the message
	var foundMsg *db.HL7Message
	var foundKey string

	keys, err := dlqKV.Keys(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Mesaj bulunamadı")
	}

	for _, key := range keys {
		entry, err := dlqKV.Get(ctx, key)
		if err != nil {
			continue
		}

		var msg db.HL7Message
		if err := json.Unmarshal(entry.Value(), &msg); err == nil {
			if msg.ID == messageID {
				foundMsg = &msg
				foundKey = key
				break
			}
		}
	}

	if foundMsg == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Mesaj bulunamadı")
	}

	// Determine which stream to republish to
	streamName := "HL7_ORDERS"
	if foundMsg.Direction == "report" {
		streamName = "HL7_REPORTS"
	}

	// Reset retry count and status
	foundMsg.RetryCount = 0
	foundMsg.Status = "pending"
	foundMsg.LastError = ""

	// Republish to the appropriate stream
	msgData, err := json.Marshal(foundMsg)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Mesaj serialize edilemedi")
	}

	if _, err := s.js.Publish(ctx, streamName, msgData); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Mesaj yeniden gönderilemedi: "+err.Error())
	}

	// Remove from DLQ
	if err := dlqKV.Delete(ctx, foundKey); err != nil {
		slog.Error("DLQ'dan mesaj silinemedi", "key", foundKey, "error", err)
	}

	slog.Info("Mesaj yeniden kuyruğa alındı",
		"messageID", messageID,
		"stream", streamName,
		"direction", foundMsg.Direction)

	return c.JSON(http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Mesaj yeniden kuyruğa alındı",
		"stream":  streamName,
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
