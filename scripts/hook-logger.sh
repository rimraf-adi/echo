#!/bin/bash
# Hook logger for debugging Antigravity payloads

mkdir -p /tmp/echo-hooks
LOGFILE="/tmp/echo-hooks/antigravity-payloads.log"

# Read stdin JSON payload
PAYLOAD=$(cat)

echo "---" >> "$LOGFILE"
echo "Timestamp: $(date)" >> "$LOGFILE"
echo "Event: $1" >> "$LOGFILE"
echo "Payload: $PAYLOAD" >> "$LOGFILE"
