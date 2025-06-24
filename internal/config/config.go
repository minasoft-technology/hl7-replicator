package config

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	OrderListenPort  int
	ReportListenPort int
	ZenPACSHost      string
	ZenPACSPort      int
	HospitalHISHost  string
	HospitalHISPort  int
	WebPort          int
	DBPath           string
	LogLevel         string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		OrderListenPort:  getEnvAsInt("ORDER_LISTEN_PORT", 7001),
		ReportListenPort: getEnvAsInt("REPORT_LISTEN_PORT", 7002),
		ZenPACSHost:      getEnv("ZENPACS_HL7_HOST", "194.187.253.34"),
		ZenPACSPort:      getEnvAsInt("ZENPACS_HL7_PORT", 2575),
		HospitalHISHost:  getEnv("HOSPITAL_HIS_HOST", "localhost"),
		HospitalHISPort:  getEnvAsInt("HOSPITAL_HIS_PORT", 9999), // Invalid port to test failures
		WebPort:          getEnvAsInt("WEB_PORT", 5678),
		DBPath:           getEnv("DB_PATH", "/data/messages.db"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
	}

	setupLogger(cfg.LogLevel)

	slog.Info("Yapılandırma yüklendi",
		"orderPort", cfg.OrderListenPort,
		"reportPort", cfg.ReportListenPort,
		"zenpacsEndpoint", cfg.ZenPACSHost+":"+strconv.Itoa(cfg.ZenPACSPort),
		"hospitalEndpoint", cfg.HospitalHISHost+":"+strconv.Itoa(cfg.HospitalHISPort),
	)

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func setupLogger(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}
