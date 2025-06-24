#!/bin/bash
#
# HL7 Replicator Restore Script
# Restores from backup created by backup.sh
#

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
INSTALL_DIR="/opt/hl7-replicator"
BACKUP_FILE=$1

echo "========================================"
echo "HL7 Replicator Restore"
echo "========================================"
echo

# Check if backup file provided
if [ -z "$BACKUP_FILE" ]; then
    echo -e "${RED}Error: No backup file specified${NC}"
    echo "Usage: $0 <backup-file.tar.gz>"
    echo
    echo "Available backups:"
    ls -la /opt/hl7-replicator/backups/*.tar.gz 2>/dev/null || echo "  No backups found"
    exit 1
fi

# Check if backup file exists
if [ ! -f "$BACKUP_FILE" ]; then
    echo -e "${RED}Error: Backup file not found: $BACKUP_FILE${NC}"
    exit 1
fi

# Confirm restore
echo -e "${YELLOW}WARNING: This will restore HL7 Replicator from backup${NC}"
echo "Backup file: $BACKUP_FILE"
echo
read -p "Are you sure you want to continue? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Restore cancelled"
    exit 0
fi

# Check Docker Compose command
if command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    COMPOSE_CMD="docker compose"
fi

cd "$INSTALL_DIR" || exit 1

# Stop services
echo -e "${BLUE}Stopping services...${NC}"
$COMPOSE_CMD down

# Extract backup
TEMP_DIR=$(mktemp -d)
echo -e "${BLUE}Extracting backup...${NC}"
tar xzf "$BACKUP_FILE" -C "$TEMP_DIR"

# Find backup directory
BACKUP_DIR=$(find "$TEMP_DIR" -name "hl7-replicator-backup-*" -type d | head -1)
if [ -z "$BACKUP_DIR" ]; then
    echo -e "${RED}Error: Invalid backup file format${NC}"
    rm -rf "$TEMP_DIR"
    exit 1
fi

# Restore configuration
echo -e "${BLUE}Restoring configuration...${NC}"
if [ -f "$BACKUP_DIR/.env" ]; then
    cp "$BACKUP_DIR/.env" "$INSTALL_DIR/.env"
    echo -e "  ${GREEN}✓ Configuration restored${NC}"
fi

# Restore NATS data
if [ -f "$BACKUP_DIR/nats-data.tar.gz" ]; then
    echo -e "${BLUE}Restoring NATS data...${NC}"
    
    # Remove existing volume
    docker volume rm hl7-replicator_hl7-data 2>/dev/null || true
    
    # Create new volume
    docker volume create hl7-replicator_hl7-data
    
    # Restore data
    docker run --rm \
        -v hl7-replicator_hl7-data:/data \
        -v "$BACKUP_DIR":/backup \
        alpine tar xzf /backup/nats-data.tar.gz -C /
    
    echo -e "  ${GREEN}✓ NATS data restored${NC}"
fi

# Clean up
rm -rf "$TEMP_DIR"

# Start services
echo -e "${BLUE}Starting services...${NC}"
$COMPOSE_CMD up -d

# Wait for services
sleep 10

# Check status
if $COMPOSE_CMD ps | grep -q "Up"; then
    echo
    echo -e "${GREEN}✓ Restore completed successfully${NC}"
    echo "Services are running"
else
    echo
    echo -e "${RED}✗ Services failed to start${NC}"
    echo "Check logs with: cd $INSTALL_DIR && $COMPOSE_CMD logs"
fi