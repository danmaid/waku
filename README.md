# Dynamic Proxy - リバースプロキシ with DoH

Apache httpdのNamed Virtual Hostベースのリバースプロキシと DNS over HTTPS (DoH) を統合したシステムです。

## 機能

### 1. DoH (DNS over HTTPS)
RFC 8484準拠のDNS over HTTPS実装
- GET/POSTメソッド対応
- DNSクエリキャッシュ
- アップストリームDNSサーバー設定可能

### 2. リバースプロキシ
Named Virtual Hostベースのリバースプロキシ
- ホスト名によるルーティング
- DoHを使用した名前解決
- 動的な設定管理

### 3. REST API & Web UI
プロキシ設定をHTTP経由で管理
- プロキシの追加/更新/削除
- 設定の永続化
- JSON API & ブラウザ向けHTML管理画面

## アーキテクチャ

```
[Client] 
    ↓
[Apache httpd :8443] (フロント)
  ├── /dns-query → Dynamic Proxy Go Service :6002 (DoH)
  ├── /v1/proxy → Dynamic Proxy Go Service :6002 (REST API & UI)
  └── Named Virtual Hosts → Reverse Proxy (httpd)
     ↓
  [Backend Services]
```

## セットアップ

### 1. 環境構築

```bash
# セットアップスクリプトの実行
chmod +x setup.sh
./setup.sh
```

### 2. サービス起動

```bash
# 簡単起動スクリプト
./start.sh

# または直接起動
./bin/dynamic-proxy -port 6002 -dns 8.8.8.8:53

# または systemd経由（本番環境向け）
sudo cp dynamic-proxy.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now dynamic-proxy
```

### 3. Apache httpd設定

Apache httpdがフロントとして動作します。設定ファイルは自動的にコピーされます。

```bash
#### HTTPS (8443) に統合する場合

`ssl.conf` の `<VirtualHost _default_:8443>` 内に以下を追加してください：

```apache
ProxyPreserveHost On
ProxyPass /dns-query http://localhost:6002/dns-query
ProxyPassReverse /dns-query http://localhost:6002/dns-query
ProxyPass /v1/proxy http://localhost:6002/v1/proxy
ProxyPassReverse /v1/proxy http://localhost:6002/v1/proxy
```
```

## 管理画面

ブラウザで以下にアクセス：

```
https://localhost:8443/v1/proxy
```

（Goアプリ直接アクセスの場合は `http://localhost:6002/v1/proxy`）

直感的なテーブル形式のUIで、プロキシの追加・編集・削除が可能です。

## API使用例

### プロキシの追加

```bash
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{
    "host": "app1.local",
    "backend": "http://localhost:3000",
    "description": "Application 1"
  }'
```

### プロキシ一覧の取得

```bash
curl -H "Accept: application/json" http://localhost:6002/v1/proxy
```

### 特定のプロキシ設定を取得

```bash
curl http://localhost:6002/v1/proxy/app1.local
```

### プロキシの更新

```bash
curl -X PUT http://localhost:6002/v1/proxy/app1.local \
  -H "Content-Type: application/json" \
  -d '{
    "backend": "http://localhost:4000",
    "description": "Updated Application 1"
  }'
```

### プロキシの削除

```bash
curl -X DELETE http://localhost:6002/v1/proxy/app1.local
```

## DoH使用例

### GET メソッド

```bash
# example.comのAレコードを照会
curl "http://localhost:6002/dns-query?dns=AAABAAABAAAAAAAAA3d3dwdleGFtcGxlA2NvbQAAAQAB"
```

HTTPS経由で利用する場合：

```bash
curl -k "https://localhost:8443/dns-query?dns=AAABAAABAAAAAAAAA3d3dwdleGFtcGxlA2NvbQAAAQAB"
```

### POST メソッド

```bash
# DNSクエリをバイナリで送信
curl -X POST http://localhost:6002/dns-query \
  -H "Content-Type: application/dns-message" \
  --data-binary @query.bin
```

## テストシナリオ

### 1. バックエンドサービスの起動

```bash
# テスト用のバックエンドサービスを起動
python3 -m http.server 3000 &
python3 -m http.server 4000 &
```

### 2. プロキシを設定

```bash
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{
    "host": "service1.local",
    "backend": "http://localhost:3000",
    "description": "Service 1"
  }'

curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{
    "host": "service2.local",
    "backend": "http://localhost:4000",
    "description": "Service 2"
  }'
```

### 3. /etc/hostsを設定

```bash
echo "127.0.0.1 service1.local service2.local" | sudo tee -a /etc/hosts
```

### 4. アクセステスト

```bash
curl -k -H "Host: service1.local" https://localhost:8443/
curl -k -H "Host: service2.local" https://localhost:8443/
```

## 設定ファイル

プロキシ設定は `/etc/httpd/conf.d/dynamic-proxy.conf` が単一の真実の源です。
管理画面からの変更もこのファイルに反映され、管理者が直接編集しても動作します。

## ポート

- `8443`: Apache httpd (HTTPS)
- `6002`: Dynamic Proxy Go service (管理API/DoH)

## 依存関係

- Go 1.21+
- Apache httpd 2.4+ (オプション)
- github.com/gorilla/mux (HTTPルーター)
- github.com/miekg/dns (DNS処理)

## ファイル構成

```
/workspaces/waku/
├── main.go                      # エントリーポイント
├── go.mod                       # Go依存関係定義
├── internal/
│   ├── doh/
│   │   └── resolver.go          # DoH実装
│   ├── httpd/
│   │   └── manager.go           # httpd設定管理
│   └── api/
│       └── handler.go           # REST APIハンドラー
├── web/
│   └── index.html               # HTML管理画面
├── config/
│   └── httpd/
│       ├── dynamic-proxy-https.conf    # ssl.conf用の参考スニペット
├── setup.sh                     # セットアップスクリプト
├── start.sh                     # クイックスタートスクリプト
└── README.md                    # このファイル
```

## トラブルシューティング

### Apache httpdが起動しない

```bash
sudo systemctl status httpd
sudo journalctl -xeu httpd
```

### Goサービスが起動しない

```bash
./bin/dynamic-proxy -port 6002 -dns 8.8.8.8:53
# エラーメッセージを確認
```

### プロキシが動作しない

1. プロキシ設定を確認: `curl -H "Accept: application/json" http://localhost:6002/v1/proxy`
2. バックエンドサービスが起動しているか確認
3. ログを確認: サービスの標準出力（ログファイルも確認）

## ライセンス

MIT

## コンテナ環境での httpd リロードについて

このプロジェクトでは、`dynamic-proxy` が httpd 設定変更後に `apachectl` コマンドを呼び出して httpd のリロードを試みます。

### コンテナ環境でのリロード方法

- 通常の Linux サーバでは `apachectl graceful` などでリロードできますが、Docker Compose 環境では直接コマンドが使えません。
- そのため、プロジェクト直下に `apachectl` というラッパースクリプトを用意しています。
- このスクリプトは `docker compose restart httpd` を実行し、httpd コンテナを再起動します。
- dynamic-proxy のコードを変更せず、コンテナ環境でも自動的にリロードが反映されます。

### 使い方

1. `dynamic-proxy` を devcontainer などで起動する前に、プロジェクト直下の `apachectl` に実行権限があることを確認してください。

    ```sh
    chmod +x ./apachectl
    export PATH="$(pwd):$PATH"
    ```

2. これで dynamic-proxy からの httpd リロード要求が自動的にコンテナ再起動に変換されます。
