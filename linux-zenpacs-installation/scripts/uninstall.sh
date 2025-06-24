#!/bin/bash
#
# HL7 Replicator Uninstall Script
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "========================================"
echo "HL7 Replicator Uninstall Script"
echo "========================================"
echo

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    echo -e "${RED}This script must be run as root (use sudo)${NC}"
    exit 1
fi

# Confirm uninstall
echo -e "${YELLOW}WARNING: This will completely remove HL7 Replicator${NC}"
echo "This action will:"
echo "  - Stop and remove all containers"
echo "  - Delete all data and configuration"
echo "  - Remove systemd service"
echo
read -p "Are you sure you want to continue? Type 'yes' to confirm: " confirm

if [ "$confirm" != "yes" ]; then
    echo "Uninstall cancelled"
    exit 0
fi

INSTALL_DIR="/opt/hl7-replicator"

# Stop systemd service
echo -e "\n${BLUE}Stopping systemd service...${NC}"
systemctl stop hl7-replicator 2>/dev/null || true
systemctl disable hl7-replicator 2>/dev/null || true

# Remove systemd service file
rm -f /etc/systemd/system/hl7-replicator.service
systemctl daemon-reload

# Stop and remove Docker containers
if [ -d "$INSTALL_DIR" ]; then
    echo -e "\n${BLUE}Stopping Docker containers...${NC}"
    cd "$INSTALL_DIR"
    
    if command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        COMPOSE_CMD="docker compose"
    fi
    
    $COMPOSE_CMD down -v 2>/dev/null || true
fi

# Remove Docker volumes
echo -e "\n${BLUE}Removing Docker volumes...${NC}"
docker volume rm hl7-replicator_hl7-data 2>/dev/null || true

# Backup before deletion
echo -e "\n${BLUE}Creating final backup...${NC}"
if [ -d "$INSTALL_DIR" ]; then
    BACKUP_FILE="/tmp/hl7-replicator-final-backup-$(date +%Y%m%d_%H%M%S).tar.gz"
    tar czf "$BACKUP_FILE" -C /opt hl7-replicator 2>/dev/null || true
    echo -e "${GREEN}Final backup saved to: $BACKUP_FILE${NC}"
fi

# Remove installation directory
echo -e "\n${BLUE}Removing installation directory...${NC}"
rm -rf "$INSTALL_DIR"

echo
echo -e "${GREEN}HL7 Replicator has been uninstalled successfully${NC}"
echo
echo "Note: Docker images are still cached. To remove them:"
echo "  docker rmi ghcr.io/minasoftweb/hl7-replicator:latest"
echo