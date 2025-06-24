#!/bin/bash
#
# HL7 Replicator Status Check Script
#

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================"
echo "HL7 Replicator Status Check"
echo "========================================"
echo

# Check Docker service
echo -e "${BLUE}Docker Service:${NC}"
if systemctl is-active --quiet docker; then
    echo -e "  ${GREEN}✓ Running${NC}"
else
    echo -e "  ${RED}✗ Not running${NC}"
fi

# Check HL7 Replicator service
echo -e "\n${BLUE}HL7 Replicator Service:${NC}"
if systemctl is-active --quiet hl7-replicator; then
    echo -e "  ${GREEN}✓ Running${NC}"
else
    echo -e "  ${RED}✗ Not running${NC}"
fi

# Check Docker containers
echo -e "\n${BLUE}Docker Containers:${NC}"
cd /opt/hl7-replicator 2>/dev/null
if [ $? -eq 0 ]; then
    if command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        COMPOSE_CMD="docker compose"
    fi
    
    $COMPOSE_CMD ps
else
    echo -e "  ${YELLOW}Installation directory not found${NC}"
fi

# Check port availability
echo -e "\n${BLUE}Port Status:${NC}"
for port in 7001 7002 5678; do
    if netstat -tuln 2>/dev/null | grep -q ":$port "; then
        echo -e "  Port $port: ${GREEN}✓ Listening${NC}"
    else
        echo -e "  Port $port: ${RED}✗ Not listening${NC}"
    fi
done

# Check web dashboard
echo -e "\n${BLUE}Web Dashboard:${NC}"
if curl -s -o /dev/null -w "%{http_code}" http://localhost:5678/api/health | grep -q "200"; then
    echo -e "  ${GREEN}✓ Accessible at http://localhost:5678${NC}"
else
    echo -e "  ${RED}✗ Not accessible${NC}"
fi

# Show configuration
echo -e "\n${BLUE}Configuration:${NC}"
if [ -f /opt/hl7-replicator/.env ]; then
    echo "  Hospital HIS Configuration:"
    grep "HOSPITAL_HIS" /opt/hl7-replicator/.env | sed 's/^/    /'
else
    echo -e "  ${YELLOW}Configuration file not found${NC}"
fi

echo
echo "========================================"