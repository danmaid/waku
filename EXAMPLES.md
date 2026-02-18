# Dynamic Proxy 使用例集

## シナリオ1: 開発環境での複数サービス管理

フロントエンド、バックエンドAPI、データベース管理画面など複数のサービスを異なるホスト名でアクセスする。

```bash
# Dynamic Proxyを起動
./start.sh &

# フロントエンド (React/Vue/etc)
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "app.dev.local",
    "backend": "http://localhost:3000",
    "description": "Frontend Application"
  }'

# バックエンドAPI
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "api.dev.local",
    "backend": "http://localhost:8080",
    "description": "Backend API"
  }'

# phpMyAdmin
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "db.dev.local",
    "backend": "http://localhost:8081",
    "description": "Database Admin"
  }'

# /etc/hostsに追加
echo "127.0.0.1 app.dev.local api.dev.local db.dev.local" | sudo tee -a /etc/hosts

# アクセス（https:8443）
curl -k https://app.dev.local:8443/
curl -k https://api.dev.local:8443/api/users
curl -k https://db.dev.local:8443/
```

## シナリオ2: マイクロサービスのローカル開発

複数のマイクロサービスを開発中に、それぞれを独立したホスト名で管理。

```bash
# ユーザーサービス
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "users.service.local",
    "backend": "http://localhost:5001",
    "description": "User Service"
  }'

# 商品サービス
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "products.service.local",
    "backend": "http://localhost:5002",
    "description": "Product Service"
  }'

# 注文サービス
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "orders.service.local",
    "backend": "http://localhost:5003",
    "description": "Order Service"
  }'

# API Gatewayのテスト（https:8443）
curl -k -H "Host: users.service.local" https://localhost:8443/api/profile
curl -k -H "Host: products.service.local" https://localhost:8443/api/list
curl -k -H "Host: orders.service.local" https://localhost:8443/api/history
```

## シナリオ3: 異なる環境のシミュレーション

同じアプリケーションの開発・ステージング・本番環境を切り替える。

```bash
# 開発環境
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "dev.myapp.local",
    "backend": "http://localhost:3000",
    "description": "Development"
  }'

# ステージング環境
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "staging.myapp.local",
    "backend": "http://localhost:3001",
    "description": "Staging"
  }'

# 本番環境（リモート）
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "prod.myapp.local",
    "backend": "https://production-server.example.com",
    "description": "Production"
  }'

# 各環境へのアクセス（https:8443）
curl -k -H "Host: dev.myapp.local" https://localhost:8443/
curl -k -H "Host: staging.myapp.local" https://localhost:8443/
curl -k -H "Host: prod.myapp.local" https://localhost:8443/
```

## シナリオ4: DoHを使ったプライバシー保護

通常のDNSクエリをHTTPS経由で行い、ISPなどからの監視を避ける。

```bash
# Dynamic Proxyのみが起動していることを確認
./start.sh

# ブラウザのDoH設定（例：Firefox）
# about:config で以下を設定：
# network.trr.mode = 2
# network.trr.uri = http://localhost:6002/dns-query

# コマンドラインでDoHを使う（digの代わり）
# 簡単なPythonスクリプト例：

cat > doh_query.py << 'EOF'
#!/usr/bin/env python3
import requests
import base64
import struct
import sys

def create_dns_query(domain):
    """Create a simple DNS A record query"""
    # DNS Header
    query_id = 0x0000
    flags = 0x0100  # Standard query
    questions = 1
    
    header = struct.pack('!HHHHHH', query_id, flags, questions, 0, 0, 0)
    
    # Question section
    question = b''
    for label in domain.split('.'):
        question += struct.pack('B', len(label)) + label.encode()
    question += b'\x00'  # End of domain
    question += struct.pack('!HH', 1, 1)  # Type A, Class IN
    
    return header + question

def query_doh(domain, doh_url='http://localhost:6002/dns-query'):
    """Query DoH server"""
    dns_query = create_dns_query(domain)
    
    # Use POST method
    response = requests.post(
        doh_url,
        data=dns_query,
        headers={'Content-Type': 'application/dns-message'}
    )
    
    if response.status_code == 200:
        print(f"DNS Query for {domain} succeeded")
        print(f"Response length: {len(response.content)} bytes")
        return response.content
    else:
        print(f"Error: {response.status_code}")
        return None

if __name__ == '__main__':
    domain = sys.argv[1] if len(sys.argv) > 1 else 'example.com'
    query_doh(domain)
EOF

chmod +x doh_query.py
python3 doh_query.py google.com
```

## シナリオ5: 動的なプロキシ設定管理

Webインターフェイスやスクリプトから動的にプロキシ設定を変更。

```bash
# 管理スクリプトの作成
cat > manage_proxy.sh << 'EOF'
#!/bin/bash

API="http://localhost:6002/v1/proxy"

add() {
    curl -X POST "$API" \
        -H "Content-Type: application/json" \
        -d "{\"host\":\"$1\",\"backend\":\"$2\",\"description\":\"$3\"}"
}

remove() {
    curl -X DELETE "$API/$1"
}

list() {
    curl "$API" | jq
}

update() {
    curl -X PUT "$API/$1" \
        -H "Content-Type: application/json" \
        -d "{\"backend\":\"$2\",\"description\":\"$3\"}"
}

case "$1" in
    add)    add "$2" "$3" "$4" ;;
    remove) remove "$2" ;;
    list)   list ;;
    update) update "$2" "$3" "$4" ;;
    *)      echo "Usage: $0 {add|remove|list|update} ..." ;;
esac
EOF

chmod +x manage_proxy.sh

# 使用例
./manage_proxy.sh add "newapp.local" "http://localhost:4000" "New Application"
./manage_proxy.sh list
./manage_proxy.sh update "newapp.local" "http://localhost:5000" "Updated App"
./manage_proxy.sh remove "newapp.local"
```

## シナリオ6: Apache httpd + Dynamic Proxyの本番構成

Apache httpdをフロントに置いて、SSL終端やロードバランシングを行う。

```bash
# Apache httpd設定のインストール（HTTPS:8443）
# ssl.confの <VirtualHost _default_:8443> に ProxyPass を追加

# Dynamic Proxyをsystemdサービスとして登録
sudo cp dynamic-proxy.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable dynamic-proxy
sudo systemctl start dynamic-proxy

# Apache httpdの起動
sudo systemctl enable httpd
sudo systemctl start httpd

# これでHTTPS(8443)でApache経由でアクセス可能
curl -k https://localhost:8443/v1/proxy

# SSL証明書の追加（Let's Encryptなど）
sudo dnf install certbot python3-certbot-apache
sudo certbot --apache -d yourdomain.com

# 以降、https://yourdomain.com でアクセス可能
```

## シナリオ7: Docker コンテナへのプロキシ

Dockerコンテナで動作する複数のサービスにプロキシ。

```bash
# Dockerでサービスを起動
docker run -d -p 8081:80 --name nginx-demo nginx
docker run -d -p 8082:80 --name httpd-demo httpd
docker run -d -p 8083:8080 --name tomcat-demo tomcat

# プロキシ設定を追加
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{"host": "nginx.docker.local", "backend": "http://localhost:8081", "description": "Nginx Container"}'

curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{"host": "httpd.docker.local", "backend": "http://localhost:8082", "description": "Apache Container"}'

curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{"host": "tomcat.docker.local", "backend": "http://localhost:8083", "description": "Tomcat Container"}'

# アクセス（https:8443）
curl -k -H "Host: nginx.docker.local" https://localhost:8443/
curl -k -H "Host: httpd.docker.local" https://localhost:8443/
curl -k -H "Host: tomcat.docker.local" https://localhost:8443/
```

## シナリオ8: 設定のバックアップと復元

```bash
# 設定のバックアップ
sudo cp /etc/httpd/conf.d/dynamic-proxy.conf /path/to/backup/dynamic-proxy-$(date +%Y%m%d).conf

# 設定の復元
# dynamic-proxy.conf を差し替えて httpd をリロード

# または、APIを使って再構築
cat /path/to/backup/dynamic-proxy-20260216.conf | 
  grep -E "ServerName|ProxyPass / " | \
  awk 'BEGIN{host=""; backend=""} /ServerName/{host=$2} /ProxyPass \/ /{backend=$3} END{if(host!="" && backend!="") print host" "backend}' | \
  while read host backend; do \
    printf '{"host":"%s","backend":"%s","description":"Restored"}\n' "$host" "${backend%/}"; \
  done | jq -r '. |
  "curl -X POST http://localhost:6002/v1/proxy -H \"Content-Type: application/json\" -d '\''" + 
  (@json | tostring) + "'\''"' | bash
```

これらの使用例は、Dynamic Proxyの柔軟性と実用性を示しています。開発環境から本番環境まで、さまざまなシナリオで活用できます。
