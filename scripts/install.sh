#!/bin/bash

# Installation script for Number Dispenser

set -e

echo "=== Number Dispenser Installation Script ==="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Please run as root (use sudo)"
    exit 1
fi

# Variables
INSTALL_DIR="/opt/number-dispenser"
DATA_DIR="/var/lib/number-dispenser"
LOG_DIR="/var/log/number-dispenser"
SERVICE_USER="number-dispenser"
BINARY_PATH="./bin/number-dispenser"

# Check if binary exists
if [ ! -f "$BINARY_PATH" ]; then
    echo "Error: Binary not found at $BINARY_PATH"
    echo "Please run 'make build' first"
    exit 1
fi

echo "Step 1: Creating service user..."
if id "$SERVICE_USER" &>/dev/null; then
    echo "  User '$SERVICE_USER' already exists"
else
    useradd -r -s /bin/false "$SERVICE_USER"
    echo "  Created user '$SERVICE_USER'"
fi

echo ""
echo "Step 2: Creating directories..."
mkdir -p "$INSTALL_DIR/bin"
mkdir -p "$DATA_DIR/data"
mkdir -p "$LOG_DIR"
echo "  Created directories"

echo ""
echo "Step 3: Copying binary..."
cp "$BINARY_PATH" "$INSTALL_DIR/bin/"
chmod +x "$INSTALL_DIR/bin/number-dispenser"
echo "  Copied binary to $INSTALL_DIR/bin/"

echo ""
echo "Step 4: Setting permissions..."
chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
chown -R "$SERVICE_USER:$SERVICE_USER" "$LOG_DIR"
echo "  Set permissions"

echo ""
echo "Step 5: Installing systemd service..."
cp scripts/number-dispenser.service /etc/systemd/system/
systemctl daemon-reload
echo "  Installed systemd service"

echo ""
echo "=== Installation Complete ==="
echo ""
echo "To start the service:"
echo "  sudo systemctl enable number-dispenser"
echo "  sudo systemctl start number-dispenser"
echo ""
echo "To check status:"
echo "  sudo systemctl status number-dispenser"
echo ""
echo "To view logs:"
echo "  sudo journalctl -u number-dispenser -f"
echo ""

