# Dynamic Proxy - Apache httpd Configuration Guide

## ファイル一覧

### 1. `dynamic-proxy-https.conf`
https (ポート 8443) 用の VirtualHost 設定

**用途:**
- DoH (DNS over HTTPS) エンドポイント (`/dns-query`) の reverse proxy
- REST API & 管理画面 (`/v1/proxy`) の reverse proxy
- SSL/TLS 設定

**設定方法:**
このファイル全体を Apache httpd の設定に含める:

```bash
# /etc/httpd/conf.d/ にシンボリックリンクを張る
sudo ln -s /workspaces/waku/config/httpd/dynamic-proxy-https.conf /etc/httpd/conf.d/

# または、手動で /etc/httpd/conf.d/ssl.conf にコンテンツをコピー
```

### 2. `dynamic-proxy.conf`
httpd によって管理される Named Virtual Host 設定

**用途:**
- dynamic-proxy Go アプリケーションが**自動生成**する
- ユーザーが REST API で新しいプロキシを追加するたびに更新される
- 個別のメインホストごとの reverse proxy 設定を含む

**重要:** このファイルは手動で編集しないでください。アプリケーションが上書きします。

**設定方法:**
```bash
# /etc/httpd/conf.d/ にシンボリックリンクを張る
sudo ln -s /workspaces/waku/config/httpd/dynamic-proxy.conf /etc/httpd/conf.d/
```

## セットアップ手順

### 1. 本番環境での設定

```bash
# httpd の設定ディレクトリにシンボリックリンクを作成
sudo ln -s /workspaces/waku/config/httpd/dynamic-proxy-https.conf /etc/httpd/conf.d/
sudo ln -s /workspaces/waku/config/httpd/dynamic-proxy.conf /etc/httpd/conf.d/

# httpd の設定をテスト
sudo httpd -t

# httpd を再起動
sudo systemctl restart httpd
```

### 2. SSL 証明書の生成

証明書がない場合は、自己署名証明書を生成:

```bash
sudo mkdir -p /etc/pki/tls/certs /etc/pki/tls/private

# OpenSSL で自己署名証明書を生成
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/pki/tls/private/localhost.key \
  -out /etc/pki/tls/certs/localhost.crt \
  -subj "/CN=localhost"

# パーミッション設定
sudo chmod 600 /etc/pki/tls/private/localhost.key
sudo chmod 644 /etc/pki/tls/certs/localhost.crt
```

### 3. ログディレクトリの作成

```bash
# API ログディレクトリ
sudo mkdir -p /var/log/api
sudo chown root:root /var/log/api
sudo chmod 755 /var/log/api

# logrotate の設定
sudo cp /workspaces/waku/config/logrotate/dynamic-proxy /etc/logrotate.d/
sudo chmod 644 /etc/logrotate.d/dynamic-proxy
```

## ポート設定

- **ポート 53** (UDP/TCP): DNS サーバー (dynamic-proxy Go アプリケーション)
- **ポート 6002** (HTTP): Dynamic Proxy API サーバー
- **ポート 8443** (HTTPS): Apache httpd フロント (公開インターフェース)

## REST API でプロキシを追加

```bash
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{
    "host": "app.example.com",
    "backend": "http://localhost:3000",
    "description": "My Application"
  }'
```

`dynamic-proxy.conf` が自動的に更新され、対応する Virtual Host が追加されます。

## 管理画面

```
https://localhost:8443/v1/proxy
```

## トラブルシューティング

### httpd が起動しない

```bash
sudo httpd -t
```

で設定を検証してください。証明書ファイルのパスが正しいか確認してください。

### dynamic-proxy.conf が作成されない

dynamic-proxy アプリケーションが実行中か確認:

```bash
ps aux | grep dynamic-proxy
```

アプリケーションのログを確認:

```bash
tail -f /var/log/api/dynamic-proxy.log
```
