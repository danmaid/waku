#!/bin/bash
# Quick start script for Dynamic Proxy service

# Go環境変数の設定
export PATH=$PATH:/usr/local/go/bin
export GOMODCACHE=/workspaces/waku/.go/pkg/mod
export GOPATH=/workspaces/waku/.go
export GOCACHE=/workspaces/waku/.go/cache

# ログディレクトリの作成
mkdir -p /var/log/api

# 既存プロセスを強制終了
pkill -9 dynamic-proxy 2>/dev/null || true

# apachectlラッパーの実行権限付与
chmod +x ./apachectl || true

# 実行（config/httpd/dynamic-proxy.confを明示指定）
cd /workspaces/waku
./bin/dynamic-proxy -port 6002 -dns-port 53 -dns 8.8.8.8:53 -logfile /var/log/api/dynamic-proxy.log \
	-front-ip 127.0.0.1 \
	> dynamic-proxy.stdout.log 2>&1 &

PID=$!
echo "Dynamic Proxy started (PID: $PID)"
echo "DNS server: port 53 (UDP/TCP)"
echo "API server: port 6002"
echo "Log file: /var/log/api/dynamic-proxy.log"
echo "To view logs: tail -f /var/log/api/dynamic-proxy.log"
