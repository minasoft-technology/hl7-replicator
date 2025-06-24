#!/bin/bash
#
# HL7 Replicator Installation Script
# For ZenPACS Integration
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    # Check for Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        print_info "Please install Docker first: https://docs.docker.com/engine/install/"
        exit 1
    fi
    
    # Check for Docker Compose
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_error "Docker Compose is not installed"
        print_info "Please install Docker Compose first: https://docs.docker.com/compose/install/"
        exit 1
    fi
    
    # Check if Docker service is running
    if ! systemctl is-active --quiet docker; then
        print_warning "Docker service is not running. Starting Docker..."
        systemctl start docker
        systemctl enable docker
    fi
    
    print_success "All prerequisites are met"
}

# Create installation directory
create_installation_dir() {
    INSTALL_DIR="/opt/hl7-replicator"
    
    print_info "Creating installation directory at $INSTALL_DIR..."
    
    if [ -d "$INSTALL_DIR" ]; then
        print_warning "Installation directory already exists"
        read -p "Do you want to continue and overwrite? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Installation cancelled"
            exit 0
        fi
    fi
    
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR"
    
    print_success "Installation directory created"
}

# Copy installation files
copy_files() {
    print_info "Copying installation files..."
    
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    
    # Copy docker-compose.yml
    cp "$SCRIPT_DIR/docker-compose.yml" "$INSTALL_DIR/"
    
    # Copy environment template
    cp "$SCRIPT_DIR/config/.env.template" "$INSTALL_DIR/.env"
    
    # Copy scripts
    cp "$SCRIPT_DIR/scripts/"*.sh "$INSTALL_DIR/" 2>/dev/null || true
    chmod +x "$INSTALL_DIR/"*.sh 2>/dev/null || true
    
    print_success "Files copied successfully"
}

# Configure environment
configure_environment() {
    print_info "Configuring environment..."
    
    ENV_FILE="$INSTALL_DIR/.env"
    
    # Check if .env already has configured values
    if grep -q "YOUR_HIS_SERVER_IP" "$ENV_FILE"; then
        print_warning "Environment configuration required"
        echo
        echo "Please provide the following information:"
        
        # Get HIS host
        while true; do
            read -p "Hospital HIS Server IP Address: " HIS_HOST
            if [[ -z "$HIS_HOST" ]]; then
                print_error "HIS host cannot be empty"
            else
                break
            fi
        done
        
        # Get HIS port
        while true; do
            read -p "Hospital HIS Server Port [7200]: " HIS_PORT
            HIS_PORT=${HIS_PORT:-7200}
            if ! [[ "$HIS_PORT" =~ ^[0-9]+$ ]]; then
                print_error "Port must be a number"
            else
                break
            fi
        done
        
        # Update .env file
        sed -i "s/YOUR_HIS_SERVER_IP/$HIS_HOST/g" "$ENV_FILE"
        sed -i "s/YOUR_HIS_PORT/$HIS_PORT/g" "$ENV_FILE"
        
        print_success "Environment configured"
    else
        print_info "Environment already configured, skipping..."
    fi
}

# Create systemd service
create_systemd_service() {
    print_info "Creating systemd service..."
    
    SERVICE_FILE="/etc/systemd/system/hl7-replicator.service"
    
    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=HL7 Replicator - ZenPACS Integration
Requires=docker.service
After=docker.service

[Service]
Type=simple
Restart=always
RestartSec=10
WorkingDirectory=/opt/hl7-replicator
ExecStart=/usr/bin/docker-compose up
ExecStop=/usr/bin/docker-compose down
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable hl7-replicator.service
    
    print_success "Systemd service created and enabled"
}

# Start services
start_services() {
    print_info "Starting HL7 Replicator..."
    
    cd "$INSTALL_DIR"
    
    # Check which docker-compose command to use
    if command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        COMPOSE_CMD="docker compose"
    fi
    
    # Pull latest image
    print_info "Pulling latest Docker image..."
    $COMPOSE_CMD pull
    
    # Start services
    $COMPOSE_CMD up -d
    
    # Wait for services to be ready
    print_info "Waiting for services to start..."
    sleep 10
    
    # Check if services are running
    if $COMPOSE_CMD ps | grep -q "Up"; then
        print_success "HL7 Replicator is running"
    else
        print_error "Failed to start services"
        print_info "Check logs with: cd $INSTALL_DIR && $COMPOSE_CMD logs"
        exit 1
    fi
}

# Display summary
display_summary() {
    echo
    echo "========================================"
    echo -e "${GREEN}HL7 Replicator Installation Complete!${NC}"
    echo "========================================"
    echo
    echo "Installation directory: $INSTALL_DIR"
    echo "Configuration file: $INSTALL_DIR/.env"
    echo
    echo "Service endpoints:"
    echo "  - Order receiver (HIS → ZenPACS): Port 7001"
    echo "  - Report receiver (ZenPACS → HIS): Port 7002"
    echo "  - Web Dashboard: http://localhost:5678"
    echo
    echo "Useful commands:"
    echo "  - View logs: cd $INSTALL_DIR && docker-compose logs -f"
    echo "  - Stop service: systemctl stop hl7-replicator"
    echo "  - Start service: systemctl start hl7-replicator"
    echo "  - Restart service: systemctl restart hl7-replicator"
    echo "  - Check status: systemctl status hl7-replicator"
    echo
    echo "To test the installation:"
    echo "  1. Open web dashboard: http://localhost:5678"
    echo "  2. Send test HL7 message using: $INSTALL_DIR/test-hl7.sh"
    echo
}

# Main installation flow
main() {
    echo "========================================"
    echo "HL7 Replicator Installation Script"
    echo "Version: 1.0"
    echo "========================================"
    echo
    
    check_root
    check_prerequisites
    create_installation_dir
    copy_files
    configure_environment
    create_systemd_service
    start_services
    display_summary
}

# Run main function
main "$@"