#!/bin/bash
# Dynamic Proxy Setup Script

set -e

echo "=== Dynamic Proxy Setup ==="

# Go依存関係のインストール
echo "Installing Go dependencies..."
go mod download
go mod tidy

# ビルド
echo "Building Dynamic Proxy service..."
go build -o bin/dynamic-proxy main.go

# Apache httpd設定（HTTPS:8443のssl.confへ追加）
if [ -d "/etc/httpd/conf.d" ]; then
    echo "Note: HTTPS integration uses ssl.conf (<VirtualHost _default_:8443>)."
    echo "Add ProxyPass entries from config/httpd/dynamic-proxy-https.conf."
fi

# システムサービスファイルの作成（オプション）
echo "Creating systemd service file..."
cat > dynamic-proxy.service << EOF
[Unit]
Description=Dynamic Proxy with DoH (httpd-managed routing)
After=network.target

[Service]
Type=simple
User=vscode
WorkingDirectory=/workspaces/waku
ExecStart=/workspaces/waku/bin/dynamic-proxy -port 6002 -dns 8.8.8.8:53
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo ""
echo "=== Setup Complete ==="
echo ""
echo "To start the Dynamic Proxy service:"
echo "  ./bin/dynamic-proxy -port 6002 -dns 8.8.8.8:53"
echo ""
echo "Or install as systemd service:"
echo "  sudo cp dynamic-proxy.service /etc/systemd/system/"
echo "  sudo systemctl daemon-reload"
echo "  sudo systemctl enable --now dynamic-proxy"
echo ""
echo "REST API will be available at:"
echo "  http://localhost:6002/v1/proxy"
echo ""
echo "DoH endpoint:"
echo "  http://localhost:6002/dns-query"
echo ""
echo "Through Apache httpd (HTTPS 8443):"
echo "  https://localhost:8443/v1/proxy"
echo "  https://localhost:8443/dns-query"
echo ""
echo "Add the ProxyPass entries to /etc/httpd/conf.d/ssl.conf (see README)."
