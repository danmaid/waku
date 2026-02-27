# Dynamic Proxy クイックスタートガイド

## すぐに始める

### 1. サービスを起動

```bash
./start.sh
```

これでDynamic Proxyサービスがポート6002で起動します。

### 2. 管理画面にアクセス（推奨）

ブラウザで以下にアクセス：

```
https://localhost:8443/v1/proxy
```

直感的なWeb UIで簡単にプロキシの追加・編集・削除ができます。

**または、コマンドラインでプロキシを追加：**

### 3. プロキシを追加（CLI）

例えば、`myapp.local` というホスト名で `localhost:3000` で動いているアプリにプロキシする場合：

```bash
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{
    "host": "myapp.local",
    "backend": "http://localhost:3000",
    "description": "My Application"
  }'
```

### 4. 設定を確認

```bash
curl -H "Accept: application/json" http://localhost:6002/v1/proxy | jq
```

出力例：
```json
{
  "count": 1,
  "proxies": [
    {
      "host": "myapp.local",
      "backend": "http://localhost:3000",
      "description": "My Application"
    }
  ]
}
```

### 5. アクセスしてみる

```bash
curl -k -H "Host: myapp.local" https://localhost:8443/
```

## よくある使い方

### 複数のサービスをホストする

```bash
# サービス1
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{"host": "api.local", "backend": "http://localhost:8000", "description": "API Server"}'

# サービス2
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{"host": "web.local", "backend": "http://localhost:3000", "description": "Web Frontend"}'

# サービス3
curl -X POST http://localhost:6002/v1/proxy \
  -H "Content-Type: application/json" \
  -d '{"host": "admin.local", "backend": "http://localhost:5000", "description": "Admin Panel"}'
```

### プロキシ設定を変更する

```bash
curl -X PUT http://localhost:6002/v1/proxy/myapp.local \
  -H "Content-Type: application/json" \
  -d '{
    "backend": "http://localhost:4000",
    "description": "My Application (新ポート)"
  }'
```

### プロキシを削除する

```bash
curl -X DELETE http://localhost:6002/v1/proxy/myapp.local
```

## DoH (DNS over HTTPS) を使う

Dynamic Proxyは内蔵のDoHサーバーを持っています。

### エンドポイント

```
http://localhost:6002/dns-query

HTTPS 経由で利用する場合：

https://localhost:8443/dns-query
```

### curlで使う例

```bash
# GETメソッド（Base64エンコードされたDNSクエリ）
curl "http://localhost:6002/dns-query?dns=AAABAAABAAAAAAAAA3d3dwdleGFtcGxlA2NvbQAAAQAB"

# POSTメソッド（バイナリDNSクエリ）
echo -ne '\x00\x00\x01\x00\x00\x01\x00\x00\x00\x00\x00\x00\x07example\x03com\x00\x00\x01\x00\x01' | \
  curl -X POST http://localhost:6002/dns-query \
    -H "Content-Type: application/dns-message" \
    --data-binary @-
```

### ブラウザでDoHを設定（Firefox）

1. `about:config` を開く
2. `network.trr.mode` を `2` に設定（DoHとシステムDNSの両方を使用）
3. `network.trr.uri` を `http://localhost:6002/dns-query` に設定

## Tips

### /etc/hostsを使わずにテスト

```bash
# Hostヘッダーを指定してアクセス
curl -H "Host: myapp.local" http://localhost:6002/
```

### 設定ファイルの保存場所

プロキシ設定は `/etc/httpd/conf.d/dynamic-proxy.conf` が単一の真実の源です。

```bash
# 設定ファイルを直接確認
sudo cat /etc/httpd/conf.d/dynamic-proxy.conf
```

### ログの確認

Dynamic Proxyサービスは標準出力にログを出力します：

```bash
# バックグラウンドで起動してログをファイルに保存
./start.sh > dynamic-proxy.log 2>&1 &

# ログを監視
tail -f dynamic-proxy.log
```

## トラブルシューティング

### ポートが既に使用されている

```bash
# ポートを確認
ss -tlnp | grep 6002

# 別のポートで起動
./bin/dynamic-proxy -port 6003 -dns 8.8.8.8:53
```

### バックエンドに接続できない

1. バックエンドサービスが実際に起動しているか確認
2. ファイアウォールやネットワーク設定を確認
3. バックエンドURLが正しいか確認

```bash
# 直接アクセスできるか確認
curl http://localhost:3000/
```

### プロキシが動作しない

```bash
# 設定が正しく登録されているか確認
curl http://localhost:6002/v1/proxy/myapp.local

# Dynamic Proxyのログを確認
# "Proxying myapp.local -> http://localhost:3000" のようなログが出るはず
```

## 次のステップ

- [README.md](README.md) - 完全なドキュメント
- `demo.sh` を実行してデモを見る
- Apache httpdをフロントに配置する（本番環境向け）

## コンテナ環境での httpd リロード

- プロジェクト直下の `apachectl` スクリプトが `docker compose restart httpd` を実行します。
- dynamic-proxy の httpd リロード要求が自動的にコンテナ再起動に変換されます。
- 必要に応じて `chmod +x ./apachectl` と `export PATH="$(pwd):$PATH"` を実行してください。
