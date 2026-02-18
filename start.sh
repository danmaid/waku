#!/bin/bash
# Quick start script for Dynamic Proxy service

# Go環境変数の設定
export PATH=$PATH:/usr/local/go/bin
export GOMODCACHE=/workspaces/waku/.go/pkg/mod
export GOPATH=/workspaces/waku/.go
export GOCACHE=/workspaces/waku/.go/cache

# ログディレクトリの作成
mkdir -p /var/log/api

# 実行（ログファイルはアプリケーション内で設定）
# ポート53で起動するためroot権限が必要
cd /workspaces/waku
./bin/dynamic-proxy -port 6002 -dns-port 53 -dns 8.8.8.8:53 -logfile /var/log/api/dynamic-proxy.log &

PID=$!
echo "Dynamic Proxy started (PID: $PID)"
echo "DNS server: port 53 (UDP/TCP)"
echo "API server: port 6002"
echo "Log file: /var/log/api/dynamic-proxy.log"
echo "To view logs: tail -f /var/log/api/dynamic-proxy.log"
