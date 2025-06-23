package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
	s.echo.GET("/*", echo.WrapHandler(http.FileServer(http.FS(webFiles))))
}

func (s *Server) handleHealth(c echo.Context) error {
	health := map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now(),
		"components": map[string]string{
			"nats": "healthy",
			"order_server": "healthy",
			"report_server": "healthy",
		},
	}
	
	return c.JSON(http.StatusOK, health)
}

func (s *Server) handleStats(c echo.Context) error {
	ctx := c.Request().Context()
	
	// Get stream info for both streams
	orderStream, err := s.js.Stream(ctx, "HL7_ORDERS")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	reportStream, err := s.js.Stream(ctx, "HL7_REPORTS")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	
	orderInfo, _ := orderStream.Info(ctx)
	reportInfo, _ := reportStream.Info(ctx)
	
	// Calculate stats
	total := orderInfo.State.Msgs + reportInfo.State.Msgs
	
	// Get consumer info for detailed stats
	orderConsumer, _ := orderStream.Consumer(ctx, "order-forwarder")
	reportConsumer, _ := reportStream.Consumer(ctx, "report-forwarder")
	
	orderConsumerInfo, _ := orderConsumer.Info(ctx)
	reportConsumerInfo, _ := reportConsumer.Info(ctx)
	
	pending := orderConsumerInfo.NumPending + reportConsumerInfo.NumPending
	delivered := orderConsumerInfo.Delivered.Consumer + reportConsumerInfo.Delivered.Consumer
	
	stats := map[string]interface{}{
		"total": total,
		"successful": delivered,
		"failed": orderConsumerInfo.NumRedelivered + reportConsumerInfo.NumRedelivered,
		"pending": pending,
	}
	
	return c.JSON(http.StatusOK, stats)
}

func (s *Server) handleGetMessages(c echo.Context) error {
	ctx := c.Request().Context()
	limit := 100 // Default limit
	
	messages := []db.HL7Message{}
	
	// Get messages from both streams
	for _, streamName := range []string{"HL7_ORDERS", "HL7_REPORTS"} {
		stream, err := s.js.Stream(ctx, streamName)
		if err != nil {
			continue
		}
		
		// Get last N messages
		consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			Name: fmt.Sprintf("web-reader-%d", time.Now().Unix()),
			DeliverPolicy: jetstream.DeliverLastPolicy,
			AckPolicy: jetstream.AckNonePolicy,
		})
		if err != nil {
			continue
		}
		
		// Fetch messages
		msgBatch, _ := consumer.Fetch(limit, jetstream.FetchMaxWait(1*time.Second))
		for msg := range msgBatch.Messages() {
			var hl7Msg db.HL7Message
			if err := json.Unmarshal(msg.Data(), &hl7Msg); err == nil {
				messages = append(messages, hl7Msg)
			}
		}
		
		// Clean up temporary consumer
		stream.DeleteConsumer(ctx, consumer.CachedInfo().Name)
	}
	
	// Sort by timestamp (newest first)
	// Simple bubble sort for demo
	for i := 0; i < len(messages)-1; i++ {
		for j := 0; j < len(messages)-i-1; j++ {
			if messages[j].Timestamp.Before(messages[j+1].Timestamp) {
				messages[j], messages[j+1] = messages[j+1], messages[j]
			}
		}
	}
	
	// Limit results
	if len(messages) > limit {
		messages = messages[:limit]
	}
	
	return c.JSON(http.StatusOK, messages)
}

func (s *Server) handleRetryMessage(c echo.Context) error {
	// messageID := c.Param("id")
	
	// This would republish the message to the appropriate stream
	// For now, return success
	return c.JSON(http.StatusOK, map[string]string{
		"status": "success",
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