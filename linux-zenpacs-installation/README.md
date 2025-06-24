# HL7 Replicator - Linux Installation Bundle

This is a complete, self-contained installation bundle for HL7 Replicator, designed for easy deployment on any Linux system.

## Overview

HL7 Replicator forwards HL7 messages between hospital HIS systems and ZenPACS:
- Receives order messages from HIS on port 7001, forwards to ZenPACS
- Receives report messages from ZenPACS on port 7002, forwards to HIS
- Provides web dashboard for monitoring on port 5678

## Prerequisites

The installation script will check for these requirements:
- Linux system (Ubuntu, CentOS, RHEL, Debian, etc.)
- Docker installed
- Docker Compose installed
- Root or sudo access

## Quick Installation

1. **Copy this directory to the target Linux system**
   ```bash
   scp -r linux-zenpacs-installation/ user@server:/tmp/
   ```

2. **Run the installation script as root**
   ```bash
   cd /tmp/linux-zenpacs-installation
   sudo chmod +x install.sh
   sudo ./install.sh
   ```

3. **Follow the prompts to configure**
   - Enter your Hospital HIS server IP address
   - Enter your Hospital HIS server port

That's it! The installation script will:
- Create installation directory at `/opt/hl7-replicator`
- Configure the environment
- Create systemd service for automatic startup
- Start the HL7 Replicator service
- Display access information

## Directory Contents

```
linux-zenpacs-installation/
├── README.md              # This file
├── install.sh             # Main installation script
├── docker-compose.yml     # Docker Compose configuration
├── config/
│   └── .env.template      # Environment configuration template
└── scripts/
    ├── test-hl7.sh        # Send test HL7 messages
    ├── check-status.sh    # Check service status
    ├── view-logs.sh       # Interactive log viewer
    ├── backup.sh          # Create backups
    └── restore.sh         # Restore from backup
```

## Configuration

The only required configuration is the Hospital HIS endpoint:
- `HOSPITAL_HIS_HOST`: IP address of your HIS server
- `HOSPITAL_HIS_PORT`: Port number of your HIS server

All other settings are pre-configured:
- ZenPACS endpoint: 194.187.253.34:2575 (fixed)
- Order listener port: 7001
- Report listener port: 7002
- Web dashboard port: 5678

## Post-Installation

### Access the Web Dashboard
Open http://your-server-ip:5678 in a web browser to:
- View message statistics
- Monitor message flow
- Check system health
- View individual messages

### Test the Installation
```bash
# Run from the server
/opt/hl7-replicator/test-hl7.sh
```

### Check Service Status
```bash
# Using systemctl
sudo systemctl status hl7-replicator

# Using the status script
/opt/hl7-replicator/check-status.sh
```

### View Logs
```bash
# Interactive log viewer
/opt/hl7-replicator/view-logs.sh

# Direct Docker Compose logs
cd /opt/hl7-replicator
docker-compose logs -f
```

## Service Management

### Start/Stop/Restart
```bash
sudo systemctl start hl7-replicator
sudo systemctl stop hl7-replicator
sudo systemctl restart hl7-replicator
```

### Enable/Disable Automatic Startup
```bash
sudo systemctl enable hl7-replicator   # Enable auto-start
sudo systemctl disable hl7-replicator  # Disable auto-start
```

## Backup and Restore

### Create Backup
```bash
/opt/hl7-replicator/backup.sh
```
Backups are stored in `/opt/hl7-replicator/backups/`

### Restore from Backup
```bash
/opt/hl7-replicator/restore.sh /opt/hl7-replicator/backups/backup-file.tar.gz
```

## Troubleshooting

### Service Won't Start
1. Check Docker is running: `sudo systemctl status docker`
2. Check logs: `cd /opt/hl7-replicator && docker-compose logs`
3. Verify configuration: `cat /opt/hl7-replicator/.env`

### Connection Issues
1. Check firewall rules for ports 7001, 7002, 5678
2. Verify HIS configuration is correct
3. Test with: `/opt/hl7-replicator/test-hl7.sh`

### High Memory/Disk Usage
1. Check NATS data size: `du -sh /var/lib/docker/volumes/hl7-replicator_hl7-data`
2. Messages are retained for 30 days by default

## Security Considerations

1. **Firewall Configuration**
   - Open ports 7001, 7002 only to trusted HIS/ZenPACS systems
   - Port 5678 (web dashboard) should be restricted to admin access

2. **Network Isolation**
   - Consider using VPN or private network for HL7 traffic
   - The application uses a dedicated Docker network

3. **Data Persistence**
   - Messages are stored in Docker volume `hl7-replicator_hl7-data`
   - Regular backups are recommended

## Support

For issues or questions:
1. Check logs first: `/opt/hl7-replicator/view-logs.sh`
2. Verify configuration: `/opt/hl7-replicator/check-status.sh`
3. Contact ZenPACS support with log files

## Technical Details

- **Base Image**: Distroless for security
- **Message Queue**: Embedded NATS JetStream
- **Web Framework**: Echo (Go)
- **UI Framework**: Alpine.js
- **Message Retention**: 30 days
- **Automatic Retry**: Yes, with exponential backoff