#!/bin/bash
#
# HL7 Test Message Sender
# Sends test HL7 messages to verify the installation
#

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo "========================================"
echo "HL7 Test Message Sender"
echo "========================================"
echo

# Function to send HL7 message
send_hl7_message() {
    local host=$1
    local port=$2
    local message=$3
    local description=$4
    
    echo -e "${BLUE}Sending $description to $host:$port...${NC}"
    
    # Send message with proper HL7 framing
    if echo -e "$message" | nc -w 3 $host $port; then
        echo -e "${GREEN}✓ Message sent successfully${NC}"
    else
        echo -e "${RED}✗ Failed to send message${NC}"
    fi
    echo
}

# Test Order Message (HIS → ZenPACS)
echo "1. Testing Order Message (HIS → ZenPACS via port 7001)"
ORDER_MSG="\x0BMSH|^~\\&|HIS|HOSPITAL|ZENPACS|MINASOFT|$(date +%Y%m%d%H%M%S)||ORM^O01|TEST$(date +%s)|P|2.5\rPID|1||123456||DOE^JOHN||19800101|M\rORC|NW|ORD$(date +%s)|||||^^^$(date +%Y%m%d%H%M%S)\rOBR|1|ORD$(date +%s)||RAD001^Chest X-Ray||||||||||||||||||||F\x1C\x0D"

send_hl7_message "localhost" "7001" "$ORDER_MSG" "Order Message"

# Test Report Message (ZenPACS → HIS)
echo "2. Testing Report Message (ZenPACS → HIS via port 7002)"
REPORT_MSG="\x0BMSH|^~\\&|ZENPACS|MINASOFT|HIS|HOSPITAL|$(date +%Y%m%d%H%M%S)||ORU^R01|TEST$(date +%s)|P|2.5\rPID|1||123456||DOE^JOHN||19800101|M\rOBR|1|ORD$(date +%s)||RAD001^Chest X-Ray||||||||||||DR001^Smith^John|||F||||||||||||||||||||\rOBX|1|TX|IMP^Impression||Normal chest radiograph. No acute findings.||||||F\x1C\x0D"

send_hl7_message "localhost" "7002" "$REPORT_MSG" "Report Message"

echo "========================================"
echo "Test complete!"
echo
echo "Check the web dashboard at http://localhost:5678 to see the messages."
echo "You should see:"
echo "  - 1 message in HL7_ORDERS stream (forwarded to ZenPACS)"
echo "  - 1 message in HL7_REPORTS stream (forwarded to HIS)"
echo