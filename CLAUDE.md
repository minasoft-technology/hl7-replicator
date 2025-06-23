# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HL7 Replicator is a Go-based medical message routing system that forwards HL7 messages between hospital systems and ZenPACS. It uses embedded NATS JetStream for reliable message delivery and provides a web dashboard for monitoring.

## Common Development Commands

### Building and Running
```bash
# Build the application
go build -o hl7-replicator ./cmd/server

# Run locally
./hl7-replicator

# Run with Docker
docker-compose up -d

# View logs
docker-compose logs -f

# Run tests (when implemented)
go test ./...
```

### Development Workflow
```bash
# Download dependencies
go mod download

# Update dependencies
go mod tidy

# Format code
go fmt ./...

# Run linter (if golangci-lint is installed)
golangci-lint run
```

## Architecture Overview

### Core Components

1. **Embedded NATS JetStream** (`internal/nats/`)
   - Provides message queuing and persistence
   - Two streams: HL7_ORDERS and HL7_REPORTS
   - Automatic retry on failure

2. **HL7 MLLP Servers** (`internal/hl7/`)
   - Two servers listening on different ports
   - Order server (7001): Receives from HIS, forwards to ZenPACS
   - Report server (7002): Receives from ZenPACS, forwards to HIS

3. **Message Consumers** (`internal/consumers/`)
   - Process messages from NATS streams
   - Handle forwarding with retry logic

4. **Web Dashboard** (`internal/web/` and `web/`)
   - Echo framework for HTTP API
   - Alpine.js for reactive UI (Turkish language)
   - Real-time monitoring via periodic polling

### Message Flow
```
HIS → [MLLP:7001] → NATS Stream → Consumer → ZenPACS (194.187.253.34:2575)
ZenPACS → [MLLP:7002] → NATS Stream → Consumer → HIS (configurable)
```

### Key Design Decisions

1. **Embedded NATS**: Single binary deployment, no external dependencies
2. **JetStream**: Reliable message delivery with persistence
3. **Turkish UI**: Target audience is Turkish hospital staff
4. **Echo + Alpine.js**: Lightweight web stack for monitoring

## Important Implementation Details

- All logging uses `slog` with structured logging
- Configuration via environment variables with `.env` support
- MLLP protocol implementation handles proper framing (0x0B start, 0x1C 0x0D end)
- Messages stored in NATS with 30-day retention
- Automatic ACK/NACK handling for HL7 messages
- Web assets embedded in binary using Go 1.16+ embed directive

## Common Tasks

### Adding New Message Types
1. Update parser in `internal/hl7/parser.go`
2. Add handling logic in consumers
3. Update web UI if needed

### Debugging Message Flow
1. Check NATS streams via web dashboard
2. Look for structured logs with message IDs
3. Use the message detail view in UI

### Modifying Forwarding Logic
1. Update consumer logic in `internal/consumers/forwarder.go`
2. Consider retry strategies and error handling
3. Update dashboard to reflect changes

## Testing HL7 Messages

Send test messages using netcat:
```bash
# Order message to HIS port
echo -e "\x0BMSH|^~\\&|HIS|HOSPITAL|ZENPACS|MINASOFT|$(date +%Y%m%d%H%M%S)||ORM^O01|TEST123|P|2.5\x1C\x0D" | nc localhost 7001

# Report message to ZenPACS port  
echo -e "\x0BMSH|^~\\&|ZENPACS|MINASOFT|HIS|HOSPITAL|$(date +%Y%m%d%H%M%S)||ORU^R01|TEST456|P|2.5\x1C\x0D" | nc localhost 7002
```

## Environment Configuration

Key environment variables:
- `HOSPITAL_HIS_HOST/PORT`: Must be configured per deployment
- `ZENPACS_HL7_HOST/PORT`: Fixed to 194.187.253.34:2575
- `DB_PATH`: NATS storage location (volume in Docker)
- `LOG_LEVEL`: debug, info, warn, error

## Deployment Considerations

1. Always mount `/data` volume for persistence
2. Expose ports 7001, 7002, and 8080
3. Configure `HOSPITAL_HIS_HOST/PORT` for each hospital
4. Monitor disk usage for NATS storage
5. Use Docker health checks for monitoring