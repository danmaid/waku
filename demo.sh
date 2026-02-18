#!/bin/bash
# Dynamic Proxy Demo Script

echo "=== Dynamic Proxy Demo ==="
echo ""

# カラー定義
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

API_BASE="http://localhost:6002/v1/proxy"

echo -e "${BLUE}1. プロキシ設定一覧の確認${NC}"
curl -s $API_BASE | jq '.'
echo ""

echo -e "${BLUE}2. 新しいプロキシ設定を追加（app1.localへ）${NC}"
curl -s -X POST $API_BASE \
  -H "Content-Type: application/json" \
  -d '{
    "host": "app1.local",
    "backend": "http://localhost:3000",
    "description": "Demo Application 1"
  }' | jq '.'
echo ""

echo -e "${BLUE}3. もう一つプロキシ設定を追加（app2.localへ）${NC}"
curl -s -X POST $API_BASE \
  -H "Content-Type: application/json" \
  -d '{
    "host": "app2.local",
    "backend": "http://localhost:4000",
    "description": "Demo Application 2"
  }' | jq '.'
echo ""

echo -e "${BLUE}4. 設定一覧を再確認${NC}"
curl -s $API_BASE | jq '.'
echo ""

echo -e "${BLUE}5. app1.localの設定を取得${NC}"
curl -s $API_BASE/app1.local | jq '.'
echo ""

echo -e "${BLUE}6. app1.localの設定を更新${NC}"
curl -s -X PUT $API_BASE/app1.local \
  -H "Content-Type: application/json" \
  -d '{
    "backend": "http://localhost:3001",
    "description": "Updated Demo Application 1"
  }' | jq '.'
echo ""

echo -e "${BLUE}7. 更新後の設定を確認${NC}"
curl -s $API_BASE/app1.local | jq '.'
echo ""

echo -e "${BLUE}8. app2.localの設定を削除${NC}"
curl -s -X DELETE $API_BASE/app2.local | jq '.'
echo ""

echo -e "${BLUE}9. 削除後の設定一覧${NC}"
curl -s $API_BASE | jq '.'
echo ""

echo -e "${GREEN}=== Demo Complete ===${NC}"
echo ""
echo "To test DoH:"
echo "  curl -X POST http://localhost:6002/dns-query \\"
echo "    -H 'Content-Type: application/dns-message' \\"
echo "    --data-binary @<dns-query-file>"
echo ""
echo "To test reverse proxy via https (after adding /etc/hosts entry):"
echo "  curl -k -H 'Host: app1.local' https://localhost:8443/"
echo ""
echo "Management UI:"
echo "  Open https://localhost:8443/v1/proxy (or http://localhost:6002/v1/proxy)"
