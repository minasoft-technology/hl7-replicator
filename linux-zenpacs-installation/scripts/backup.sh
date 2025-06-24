#!/bin/bash
#
# HL7 Replicator Backup Script
# Creates backups of NATS data and configuration
#

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
INSTALL_DIR="/opt/hl7-replicator"
BACKUP_DIR="/opt/hl7-replicator/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="hl7-replicator-backup-$TIMESTAMP"

echo "========================================"
echo "HL7 Replicator Backup"
echo "========================================"
echo

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Check Docker Compose command
if command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    COMPOSE_CMD="docker compose"
fi

cd "$INSTALL_DIR" || exit 1

# Stop services
echo -e "${BLUE}Stopping services...${NC}"
$COMPOSE_CMD stop

# Create backup
echo -e "${BLUE}Creating backup...${NC}"
mkdir -p "$BACKUP_DIR/$BACKUP_NAME"

# Backup configuration
cp "$INSTALL_DIR/.env" "$BACKUP_DIR/$BACKUP_NAME/" 2>/dev/null
cp "$INSTALL_DIR/docker-compose.yml" "$BACKUP_DIR/$BACKUP_NAME/" 2>/dev/null

# Backup NATS data volume
echo -e "${BLUE}Backing up NATS data...${NC}"
docker run --rm \
    -v hl7-replicator_hl7-data:/data \
    -v "$BACKUP_DIR/$BACKUP_NAME":/backup \
    alpine tar czf /backup/nats-data.tar.gz -C / data

# Start services again
echo -e "${BLUE}Starting services...${NC}"
$COMPOSE_CMD start

# Create compressed archive
echo -e "${BLUE}Compressing backup...${NC}"
cd "$BACKUP_DIR"
tar czf "$BACKUP_NAME.tar.gz" "$BACKUP_NAME"
rm -rf "$BACKUP_NAME"

# Clean old backups (keep last 7)
echo -e "${BLUE}Cleaning old backups...${NC}"
ls -t "$BACKUP_DIR"/*.tar.gz 2>/dev/null | tail -n +8 | xargs rm -f 2>/dev/null

echo
echo -e "${GREEN}✓ Backup completed successfully${NC}"
echo "  Backup file: $BACKUP_DIR/$BACKUP_NAME.tar.gz"
echo
echo "To restore from this backup, use:"
echo "  $INSTALL_DIR/restore.sh $BACKUP_DIR/$BACKUP_NAME.tar.gz"
echo