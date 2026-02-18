package doh

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Resolver はDNSリゾルバー（ポート53およびDoH対応）
type Resolver struct {
	upstream      string
	cache         map[string]*cacheEntry
	mu            sync.RWMutex
	getDNSAddress func(string) (net.IP, bool) // ホストごとのDNSアドレス取得関数
}

type cacheEntry struct {
	response *dns.Msg
	expires  time.Time
}

// NewResolver は新しいDNSリゾルバーを作成
func NewResolver(upstreamDNS string) *Resolver {
	return &Resolver{
		upstream: upstreamDNS,
		cache:    make(map[string]*cacheEntry),
	}
}

// SetManagedHostResolver は管理対象ホストのDNSアドレス取得関数を設定
func (r *Resolver) SetManagedHostResolver(getDNSAddress func(string) (net.IP, bool)) {
	r.getDNSAddress = getDNSAddress
}

// ServeDNS は標準DNSクエリを処理（ポート53用）
func (r *Resolver) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	startTime := time.Now()
	
	// リクエストログ
	if len(req.Question) > 0 {
		q := req.Question[0]
		qtype := dns.TypeToString[q.Qtype]
		log.Printf("[DNS] Request: From=%s, Name=%s, Type=%s, ID=%d", 
			w.RemoteAddr(), q.Name, qtype, req.Id)
	}

	// クエリを処理
	response, err := r.Query(req)
	if err != nil {
		log.Printf("[DNS] Error: Query failed: %v", err)
		// エラーレスポンスを返す
		response = new(dns.Msg)
		response.SetReply(req)
		response.Rcode = dns.RcodeServerFailure
	}

	// レスポンスログ
	rcodeStr := dns.RcodeToString[response.Rcode]
	elapsed := time.Since(startTime)
	log.Printf("[DNS] Response: Rcode=%s, Answers=%d, elapsed=%v", 
		rcodeStr, len(response.Answer), elapsed)

	// レスポンスを送信
	if err := w.WriteMsg(response); err != nil {
		log.Printf("[DNS] Error: Failed to write response: %v", err)
	}
}

// HandleDoH はDoHリクエストを処理 (RFC 8484)
func (r *Resolver) HandleDoH(w http.ResponseWriter, req *http.Request) {
	var dnsMsg *dns.Msg
	var err error
	startTime := time.Now()

	log.Printf("[DoH] Request: Method=%s, RemoteAddr=%s, Path=%s", req.Method, req.RemoteAddr, req.URL.Path)

	// GETまたはPOSTメソッドに対応
	if req.Method == http.MethodGet {
		// GET: dns パラメータから取得
		dnsParam := req.URL.Query().Get("dns")
		if dnsParam == "" {
			log.Printf("[DoH] Error: Missing dns parameter")
			http.Error(w, "Missing dns parameter", http.StatusBadRequest)
			return
		}

		log.Printf("[DoH] GET request with dns parameter (length=%d)", len(dnsParam))

		// Base64 URL デコード
		wireFormat, err := base64.RawURLEncoding.DecodeString(dnsParam)
		if err != nil {
			log.Printf("[DoH] Error: Invalid dns parameter encoding: %v", err)
			http.Error(w, "Invalid dns parameter encoding", http.StatusBadRequest)
			return
		}

		dnsMsg = new(dns.Msg)
		if err := dnsMsg.Unpack(wireFormat); err != nil {
			log.Printf("[DoH] Error: Invalid DNS message: %v", err)
			http.Error(w, "Invalid DNS message", http.StatusBadRequest)
			return
		}
	} else if req.Method == http.MethodPost {
		// POST: ボディから取得
		wireFormat, err := io.ReadAll(req.Body)
		if err != nil {
			log.Printf("[DoH] Error: Failed to read request body: %v", err)
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		log.Printf("[DoH] POST request with body (length=%d bytes)", len(wireFormat))

		dnsMsg = new(dns.Msg)
		if err := dnsMsg.Unpack(wireFormat); err != nil {
			log.Printf("[DoH] Error: Invalid DNS message: %v", err)
			http.Error(w, "Invalid DNS message", http.StatusBadRequest)
			return
		}
	} else {
		log.Printf("[DoH] Error: Method not allowed: %s", req.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// リクエストの詳細をログ
	if len(dnsMsg.Question) > 0 {
		for i, q := range dnsMsg.Question {
			qtype := dns.TypeToString[q.Qtype]
			log.Printf("[DoH] Query[%d]: Name=%s, Type=%s (ID=%d)", i, q.Name, qtype, dnsMsg.Id)
		}
	}

	// DNS クエリを処理
	response, err := r.Query(dnsMsg)
	if err != nil {
		log.Printf("[DoH] Error: DNS query failed: %v", err)
		http.Error(w, "DNS query failed", http.StatusInternalServerError)
		return
	}

	// レスポンスの詳細をログ
	rcodeStr := dns.RcodeToString[response.Rcode]
	log.Printf("[DoH] Response: Rcode=%s, Questions=%d, Answers=%d, Authorities=%d",
		rcodeStr, len(response.Question), len(response.Answer), len(response.Ns))

	if len(response.Answer) > 0 {
		for i, rr := range response.Answer {
			header := rr.Header()
			typeStr := dns.TypeToString[header.Rrtype]
			log.Printf("[DoH] Answer[%d]: Name=%s, Type=%s, TTL=%d, Value=%s",
				i, header.Name, typeStr, header.Ttl, rr.String())
		}
	}

	// レスポンスをシリアライズ
	packed, err := response.Pack()
	if err != nil {
		log.Printf("[DoH] Error: Failed to pack DNS response: %v", err)
		http.Error(w, "Failed to pack DNS response", http.StatusInternalServerError)
		return
	}

	elapsed := time.Since(startTime)
	log.Printf("[DoH] Success: Response size=%d bytes, elapsed=%v", len(packed), elapsed)

	// RFC 8484: application/dns-message
	w.Header().Set("Content-Type", "application/dns-message")
	w.WriteHeader(http.StatusOK)
	w.Write(packed)
}

// Query はDNSクエリを実行（キャッシュ付き）
func (r *Resolver) Query(msg *dns.Msg) (*dns.Msg, error) {
	if len(msg.Question) == 0 {
		return nil, fmt.Errorf("no questions in DNS message")
	}

	question := msg.Question[0]
	qname := strings.TrimSuffix(question.Name, ".")
	qtype := dns.TypeToString[question.Qtype]

	log.Printf("[Query] Processing: Name=%s, Type=%s, ManagedHost=%v", qname, qtype, r.getDNSAddress != nil)

	// 管理対象ホストの場合は固定応答
	if r.getDNSAddress != nil {
		if dnsIP, ok := r.getDNSAddress(qname); ok {
			log.Printf("[Query] Managed host detected: %s, returning fixed IP: %s", qname, dnsIP.String())
			response := new(dns.Msg)
			response.SetReply(msg)
			response.Authoritative = true

			if question.Qtype == dns.TypeA {
				if ip4 := dnsIP.To4(); ip4 != nil {
					rr := &dns.A{
						Hdr: dns.RR_Header{
							Name:   question.Name,
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    60,
						},
						A: ip4,
					}
					response.Answer = append(response.Answer, rr)
					log.Printf("[Query] Added A record: %s -> %s", question.Name, ip4.String())
				}
			}
			if question.Qtype == dns.TypeAAAA {
				if ip4 := dnsIP.To4(); ip4 == nil {
					rr := &dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   question.Name,
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    60,
						},
						AAAA: dnsIP,
					}
					response.Answer = append(response.Answer, rr)
					log.Printf("[Query] Added AAAA record: %s -> %s", question.Name, dnsIP.String())
				}
			}

			return response, nil
		}

		// 管理外ホスト
		log.Printf("[Query] Unmanaged host detected: %s, forwarding to upstream DNS: %s", qname, r.upstream)
	}

	cacheKey := fmt.Sprintf("%s:%d", question.Name, question.Qtype)

	// キャッシュチェック
	r.mu.RLock()
	if entry, ok := r.cache[cacheKey]; ok {
		if time.Now().Before(entry.expires) {
			r.mu.RUnlock()
			log.Printf("[Query] Cache hit: %s (expires in %v)", cacheKey, time.Until(entry.expires))
			response := entry.response.Copy()
			response.SetReply(msg)
			return response, nil
		}
		log.Printf("[Query] Cache expired: %s", cacheKey)
	}
	r.mu.RUnlock()

	log.Printf("[Query] Cache miss: %s, querying upstream DNS: %s", cacheKey, r.upstream)

	// アップストリームDNSへクエリ
	c := new(dns.Client)
	c.Timeout = 5 * time.Second

	startTime := time.Now()
	response, _, err := c.Exchange(msg, r.upstream)
	elapsed := time.Since(startTime)

	if err != nil {
		log.Printf("[Query] Upstream DNS query failed for %s: %v (elapsed=%v)", cacheKey, err, elapsed)
		return nil, fmt.Errorf("upstream DNS query failed: %w", err)
	}

	rcodeStr := dns.RcodeToString[response.Rcode]
	log.Printf("[Query] Upstream response: Rcode=%s, Answers=%d, elapsed=%v", rcodeStr, len(response.Answer), elapsed)

	if len(response.Answer) > 0 {
		for i, rr := range response.Answer {
			log.Printf("[Query] Answer[%d]: %s", i, rr.String())
		}
	}

	// キャッシュに保存（TTLを考慮）
	if response.Rcode == dns.RcodeSuccess && len(response.Answer) > 0 {
		ttl := uint32(300) // デフォルト5分
		for _, rr := range response.Answer {
			if rr.Header().Ttl < ttl {
				ttl = rr.Header().Ttl
			}
		}

		r.mu.Lock()
		r.cache[cacheKey] = &cacheEntry{
			response: response.Copy(),
			expires:  time.Now().Add(time.Duration(ttl) * time.Second),
		}
		r.mu.Unlock()

		log.Printf("[Query] Cached response for %s with TTL=%d seconds", cacheKey, ttl)
	}

	return response, nil
}

// Resolve はホスト名をIPアドレスに解決（リバースプロキシから使用）
func (r *Resolver) Resolve(hostname string) ([]net.IP, error) {
	log.Printf("[Resolve] Starting resolution for hostname: %s", hostname)

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(hostname), dns.TypeA)

	response, err := r.Query(msg)
	if err != nil {
		log.Printf("[Resolve] Error: Failed to resolve %s: %v", hostname, err)
		return nil, err
	}

	var ips []net.IP
	for _, rr := range response.Answer {
		if a, ok := rr.(*dns.A); ok {
			ips = append(ips, a.A)
			log.Printf("[Resolve] Found A record: %s -> %s", hostname, a.A.String())
		}
	}

	if len(ips) == 0 {
		log.Printf("[Resolve] Error: No A records found for %s", hostname)
		return nil, fmt.Errorf("no A records found for %s", hostname)
	}

	log.Printf("[Resolve] Success: Resolved %s to %v", hostname, ips)
	return ips, nil
}

// Dial はDoHを使用してホスト名を解決し、接続を確立
func (r *Resolver) Dial(network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		log.Printf("[Dial] Error: Failed to split host:port %s: %v", address, err)
		return nil, err
	}

	log.Printf("[Dial] Attempting to dial: host=%s, port=%s, network=%s", host, port, network)

	// DoHで名前解決
	ips, err := r.Resolve(host)
	if err != nil {
		// フォールバック: デフォルトの解決を使用
		log.Printf("[Dial] Warning: DoH resolution failed for %s, using default resolver: %v", host, err)
		return net.Dial(network, address)
	}

	// 最初のIPで接続を試行
	if len(ips) > 0 {
		resolvedAddr := net.JoinHostPort(ips[0].String(), port)
		log.Printf("[Dial] Resolved address: %s -> %s", address, resolvedAddr)
		return net.Dial(network, resolvedAddr)
	}

	log.Printf("[Dial] Error: No valid IP address for %s", host)
	return nil, fmt.Errorf("no valid IP address for %s", host)
}
