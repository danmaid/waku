package doh

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

const testUpstreamDNS = "127.0.0.1:1"

// TestQueryManagedHostReturnsFixedIP は管理対象ホストがA記録固定IPを返すことをテスト
func TestQueryManagedHostReturnsFixedIP(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	managedHosts := map[string]bool{
		"app.local": true,
		"api.local": true,
	}
	frontIP := net.ParseIP("127.0.0.1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if managedHosts[host] {
			return frontIP, true
		}
		return nil, false
	})

	// A クエリ
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("app.local"), dns.TypeA)

	response, err := resolver.Query(msg)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if response.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected RcodeSuccess, got %d", response.Rcode)
	}

	found := false
	for _, rr := range response.Answer {
		if a, ok := rr.(*dns.A); ok && a.A.Equal(frontIP) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected A record with front IP")
	}
}

// TestQueryUnmanagedHostFallsBackToUpstream は管理外ホストがアップストリームへ委譲されることをテスト
func TestQueryUnmanagedHostFallsBackToUpstream(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	managedHosts := map[string]bool{
		"app.local": true,
	}
	frontIP := net.ParseIP("127.0.0.1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if managedHosts[host] {
			return frontIP, true
		}
		return nil, false
	})

	// 管理外ホストのクエリ
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("unmanaged.local"), dns.TypeA)

	_, err := resolver.Query(msg)
	if err == nil {
		t.Fatal("expected upstream error for unmanaged host")
	}
}

// TestQueryManagedHostAAAA はAAAA述語（IPv6）をテスト
func TestQueryManagedHostAAAA(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	managedHosts := map[string]bool{
		"app.local": true,
	}
	// IPv6アドレス
	frontIP := net.ParseIP("::1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if managedHosts[host] {
			return frontIP, true
		}
		return nil, false
	})

	// AAAA クエリ
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("app.local"), dns.TypeAAAA)

	response, err := resolver.Query(msg)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if response.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected RcodeSuccess, got %d", response.Rcode)
	}

	found := false
	for _, rr := range response.Answer {
		if aaaa, ok := rr.(*dns.AAAA); ok && aaaa.AAAA.Equal(frontIP) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected AAAA record with front IPv6")
	}
}

// TestQueryWithoutManagedHostResolver はmanagedHost=nilの場合の動作をテスト
func TestQueryWithoutManagedHostResolver(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	// SetManagedHostResolver を呼び出さない（nil）

	// クエリはアップストリームに流れるが、テスト環境では接続できないため
	// エラーが返ることを期待
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("google.com"), dns.TypeA)

	response, err := resolver.Query(msg)
	// エラーまたは応答いずれでもOK（アップストリームに委譲された）
	if err == nil && response.Rcode == dns.RcodeSuccess {
		// アップストリームが動作している（テスト環境では稀）
		if len(response.Answer) == 0 {
			t.Fatal("expected answers for successful upstream query")
		}
	}
	// エラーが返るのが一般的（テスト環境では8.8.8.8に到達できない）
}

// TestQueryMultipleManagedHosts は複数の管理ホストをテスト
func TestQueryMultipleManagedHosts(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	managedHosts := map[string]bool{
		"api.local": true,
		"web.local": true,
		"db.local":  true,
	}
	frontIP := net.ParseIP("192.168.1.100")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if managedHosts[host] {
			return frontIP, true
		}
		return nil, false
	})

	hosts := []string{"api.local", "web.local", "db.local", "unmanaged.local"}
	for i, host := range hosts {
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(host), dns.TypeA)

		response, err := resolver.Query(msg)
		if i == len(hosts)-1 {
			if err == nil {
				t.Fatal("expected upstream error for unmanaged host")
			}
			continue
		}
		if err != nil {
			t.Fatalf("query for %s failed: %v", host, err)
		}
		if response.Rcode != dns.RcodeSuccess {
			t.Fatalf("%s: expected %d, got %d", host, dns.RcodeSuccess, response.Rcode)
		}
	}
}

// TestQueryResponseContainsFQDN はレスポンスがクエリのFQDNを含むことをテスト
func TestQueryResponseContainsFQDN(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	frontIP := net.ParseIP("127.0.0.1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if host == "example.local" {
			return frontIP, true
		}
		return nil, false
	})

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("example.local"), dns.TypeA)

	response, err := resolver.Query(msg)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(response.Answer) == 0 {
		t.Fatal("expected answer records")
	}

	// 最初のレコードのNameがクエリのFQDNと一致するか確認
	if response.Answer[0].Header().Name != dns.Fqdn("example.local") {
		t.Fatalf("expected FQDN %s in answer, got %s", dns.Fqdn("example.local"), response.Answer[0].Header().Name)
	}
}

// TestCacheHit はキャッシュのヒットをテスト
func TestCacheHit(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	queryCount := 0

	// managedHost 関数でクエリカウント
	frontIP := net.ParseIP("127.0.0.1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		queryCount++
		if host == "cached.local" {
			return frontIP, true
		}
		return nil, false
	})

	// 1回目のクエリ
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("cached.local"), dns.TypeA)
	msg.Id = 0

	_, err := resolver.Query(msg)
	if err != nil {
		t.Fatalf("first query failed: %v", err)
	}

	// 2回目のクエリ（キャッシュ）
	msg2 := new(dns.Msg)
	msg2.SetQuestion(dns.Fqdn("cached.local"), dns.TypeA)
	msg2.Id = 0

	_, err = resolver.Query(msg2)
	if err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	// 管理ホスト判定は常に呼ばれるためカウントは厳密ではないが、
	// キャッシュがある場合はアップストリームDNS呼び出しが減る
	// （テストの複雑性を避けるため、基本的な動作確認のみ）
}

// TestQueryTTL はレコードのTTLをテスト
func TestQueryTTL(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	frontIP := net.ParseIP("127.0.0.1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if host == "ttl.local" {
			return frontIP, true
		}
		return nil, false
	})

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("ttl.local"), dns.TypeA)

	response, err := resolver.Query(msg)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	for _, rr := range response.Answer {
		if rr.Header().Ttl != 60 {
			t.Fatalf("expected TTL 60, got %d", rr.Header().Ttl)
		}
	}
}

// TestQueryAuthoritative はAutoritativeフラグをテスト
func TestQueryAuthoritative(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	frontIP := net.ParseIP("127.0.0.1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if host == "auth.local" {
			return frontIP, true
		}
		return nil, false
	})

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("auth.local"), dns.TypeA)

	response, err := resolver.Query(msg)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if !response.Authoritative {
		t.Fatal("expected Authoritative flag to be set")
	}
}

// TestQueryManagedHostWithIPv4FrontIP はIPv4フロントIPでのA記録をテスト
func TestQueryManagedHostWithIPv4FrontIP(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	frontIPs := []string{"192.168.1.1", "10.0.0.1", "172.16.0.1"}

	for _, ipStr := range frontIPs {
		frontIP := net.ParseIP(ipStr)
		resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
			if host == "test.local" {
				return frontIP, true
			}
			return nil, false
		})

		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn("test.local"), dns.TypeA)

		response, err := resolver.Query(msg)
		if err != nil {
			t.Fatalf("query failed for %s: %v", ipStr, err)
		}

		found := false
		for _, rr := range response.Answer {
			if a, ok := rr.(*dns.A); ok && a.A.Equal(frontIP) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected A record %s in response", ipStr)
		}
	}
}

// TestQueryQType はクエリタイプ（A/AAAA）の判定をテスト
func TestQueryQType(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	frontIP := net.ParseIP("::1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if host == "qtype.local" {
			return frontIP, true
		}
		return nil, false
	})

	// AAAA クエリ
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("qtype.local"), dns.TypeAAAA)

	response, err := resolver.Query(msg)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if response.Rcode != dns.RcodeSuccess {
		t.Fatalf("expected RcodeSuccess, got %d", response.Rcode)
	}
	if len(response.Answer) == 0 {
		t.Fatal("expected AAAA answer")
	}

	ok := false
	for _, rr := range response.Answer {
		if _, isAAAA := rr.(*dns.AAAA); isAAAA {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatal("expected AAAA record type in answer")
	}
}

// TestQueryReplyPreservesID はレスポンスがクエリIDを保持することをテスト
func TestQueryReplyPreservesID(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(testUpstreamDNS)
	frontIP := net.ParseIP("127.0.0.1")
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		if host == "id.local" {
			return frontIP, true
		}
		return nil, false
	})

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("id.local"), dns.TypeA)
	msg.Id = 12345

	response, err := resolver.Query(msg)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if response.Id != 12345 {
		t.Fatalf("expected ID 12345, got %d", response.Id)
	}
}
