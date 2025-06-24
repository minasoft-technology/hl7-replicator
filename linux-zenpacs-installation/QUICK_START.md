# HL7 Replicator - Quick Start Guide

## 🚀 Installation in 3 Steps

### Step 1: Copy files to server
```bash
scp -r linux-zenpacs-installation/ user@your-server:/tmp/
```

### Step 2: Run installer
```bash
ssh user@your-server
cd /tmp/linux-zenpacs-installation
sudo ./install.sh
```

### Step 3: Configure HIS connection
When prompted, enter:
- Your HIS server IP address
- Your HIS server port (default: 7200)

## ✅ Verify Installation

1. **Check web dashboard**: http://your-server:5678
2. **Send test message**: `/opt/hl7-replicator/test-hl7.sh`
3. **Check status**: `sudo systemctl status hl7-replicator`

## 📋 Important Information

### Network Ports
- **7001**: Receives orders from HIS → forwards to ZenPACS
- **7002**: Receives reports from ZenPACS → forwards to HIS  
- **5678**: Web dashboard

### ZenPACS Endpoint (Fixed)
- Host: 194.187.253.34
- Port: 2575

### Common Commands
```bash
# View logs
/opt/hl7-replicator/view-logs.sh

# Check status
/opt/hl7-replicator/check-status.sh

# Restart service
sudo systemctl restart hl7-replicator

# Create backup
/opt/hl7-replicator/backup.sh
```

## 🔧 Troubleshooting

### Service won't start?
```bash
cd /opt/hl7-replicator
docker-compose logs --tail=50
```

### Connection refused?
1. Check firewall: `sudo ufw status` or `sudo iptables -L`
2. Verify HIS config: `cat /opt/hl7-replicator/.env`

### Need help?
1. Check full README.md for detailed documentation
2. Contact ZenPACS support with logs