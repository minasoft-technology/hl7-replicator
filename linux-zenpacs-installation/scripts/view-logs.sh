#!/bin/bash
#
# HL7 Replicator Log Viewer
# Provides easy access to different log views
#

# Colors
BLUE='\033[0;34m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "========================================"
echo "HL7 Replicator Log Viewer"
echo "========================================"
echo

# Check Docker Compose command
if command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    COMPOSE_CMD="docker compose"
fi

cd /opt/hl7-replicator 2>/dev/null || {
    echo "Error: Installation directory not found"
    exit 1
}

# Show menu
echo "Select log view option:"
echo "  1) Follow all logs (real-time)"
echo "  2) Last 100 lines"
echo "  3) Last 500 lines"
echo "  4) Logs from last hour"
echo "  5) Error logs only"
echo "  6) Order messages only"
echo "  7) Report messages only"
echo "  8) Exit"
echo

read -p "Enter your choice (1-8): " choice

case $choice in
    1)
        echo -e "\n${BLUE}Following all logs (press Ctrl+C to stop)...${NC}\n"
        $COMPOSE_CMD logs -f
        ;;
    2)
        echo -e "\n${BLUE}Last 100 lines:${NC}\n"
        $COMPOSE_CMD logs --tail=100
        ;;
    3)
        echo -e "\n${BLUE}Last 500 lines:${NC}\n"
        $COMPOSE_CMD logs --tail=500
        ;;
    4)
        echo -e "\n${BLUE}Logs from last hour:${NC}\n"
        $COMPOSE_CMD logs --since=1h
        ;;
    5)
        echo -e "\n${BLUE}Error logs only:${NC}\n"
        $COMPOSE_CMD logs --tail=1000 | grep -E "(ERROR|error|Error|FATAL|fatal|Fatal)"
        ;;
    6)
        echo -e "\n${BLUE}Order messages only:${NC}\n"
        $COMPOSE_CMD logs --tail=1000 | grep -E "(ORM|order|Order|7001)"
        ;;
    7)
        echo -e "\n${BLUE}Report messages only:${NC}\n"
        $COMPOSE_CMD logs --tail=1000 | grep -E "(ORU|report|Report|7002)"
        ;;
    8)
        echo -e "${GREEN}Exiting...${NC}"
        exit 0
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac