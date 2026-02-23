package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/danmaid/dynamic-proxy/internal/api"
	"github.com/danmaid/dynamic-proxy/internal/doh"
	"github.com/danmaid/dynamic-proxy/internal/httpd"
	"github.com/danmaid/dynamic-proxy/internal/tls"
	"github.com/gorilla/mux"
	"github.com/miekg/dns"
)

func main() {
	port := flag.Int("port", 6002, "Port to listen on for HTTP API")
	dnsPort := flag.Int("dns-port", 53, "Port to listen on for DNS")
	dnsServer := flag.String("dns", "8.8.8.8:53", "Upstream DNS server")
	frontIP := flag.String("front-ip", "127.0.0.1", "Front httpd IP for DNS answers")
	logFile := flag.String("logfile", "/var/log/api/dynamic-proxy.log", "Log file path")
	flag.Parse()

	// ログファイルを開く（追記モード）
	f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v\n", *logFile, err)
		os.Exit(1)
	}
	defer f.Close()

	// ログ出力先をファイルに設定
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags)

	// DNS resolverの初期化
	resolver := doh.NewResolver(*dnsServer)

	// TLSマネージャーの初期化（CA証明書の自動生成）
	tlsManager, err := tls.NewManager("config/tls")
	if err != nil {
		log.Fatalf("Failed to initialize TLS manager: %v", err)
	}

	// デフォルトVirtualHost用のlocalhost証明書を生成
	if err := tlsManager.GenerateServerCert("localhost"); err != nil {
		log.Printf("Warning: Failed to generate localhost certificate: %v", err)
	}

	// httpd設定マネージャーの初期化
	httpdManager := httpd.NewManager()
	httpdManager.SetTLSManager(tlsManager)

	// 管理対象ホストは設定されたIPで応答、管理外はアップストリームに転送
	parsedFrontIP := net.ParseIP(*frontIP)
	if parsedFrontIP == nil {
		log.Fatalf("Invalid front IP: %s", *frontIP)
	}

	// DNS応答アドレス取得関数を設定
	resolver.SetManagedHostResolver(func(host string) (net.IP, bool) {
		// ホストごとに設定されたDNSアドレスを取得
		if dnsAddr, ok := httpdManager.GetDNSAddress(host); ok {
			if ip := net.ParseIP(dnsAddr); ip != nil {
				return ip, true
			}
		}
		// 設定されていない場合はデフォルトのfrontIPを返す
		if httpdManager.HasHost(host) {
			return parsedFrontIP, true
		}
		return nil, false
	})

	// DNS サーバー起動（UDP）
	dnsAddr := fmt.Sprintf(":%d", *dnsPort)
	go func() {
		log.Printf("Starting DNS server (UDP) on %s", dnsAddr)
		if err := dns.ListenAndServe(dnsAddr, "udp", resolver); err != nil {
			log.Fatalf("Failed to start DNS server (UDP): %v", err)
		}
	}()

	// DNS サーバー起動（TCP）
	go func() {
		log.Printf("Starting DNS server (TCP) on %s", dnsAddr)
		if err := dns.ListenAndServe(dnsAddr, "tcp", resolver); err != nil {
			log.Fatalf("Failed to start DNS server (TCP): %v", err)
		}
	}()

	// REST APIハンドラーの初期化
	apiHandler := api.NewHandler(httpdManager)

	// ルーターの設定
	r := mux.NewRouter().StrictSlash(true)

	// REST API エンドポイント（Accept ヘッダーで応答を切り替え）
	// より具体的なルートを先に登録（重要：/v1/proxy/{host}/config は /v1/proxy/{host} より前）
	r.HandleFunc("/v1/proxy/{host}/config", apiHandler.GetVirtualHostConfig).Methods("GET")
	r.HandleFunc("/v1/proxy/{host}/config", apiHandler.UpdateVirtualHostConfig).Methods("PUT")
	r.HandleFunc("/v1/proxy/{host}/analyze", apiHandler.AnalyzeBackendCSP).Methods("GET")

	r.HandleFunc("/v1/proxy", apiHandler.ListProxies).Methods("GET")
	r.HandleFunc("/v1/proxy", apiHandler.CreateProxy).Methods("POST")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.GetProxy).Methods("GET")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.UpdateProxy).Methods("PUT")
	r.HandleFunc("/v1/proxy/{host}", apiHandler.DeleteProxy).Methods("DELETE")

	// DoH エンドポイント (RFC 8484) - ブラウザ単位での設定用
	r.HandleFunc("/dns-query", resolver.HandleDoH).Methods("GET", "POST")

	// CA証明書ダウンロードエンドポイント
	r.HandleFunc("/v1/ca/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Header().Set("Content-Disposition", "attachment; filename=dynamic-proxy-ca.crt")
		w.Write(tlsManager.GetCACertPEM())
	}).Methods("GET")

	// サーバー起動
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting Dynamic Proxy management server on %s", addr)
	log.Printf("DNS server: UDP/TCP on port %d", *dnsPort)
	log.Printf("DoH endpoint: http://localhost%s/dns-query", addr)
	log.Printf("Upstream DNS: %s", *dnsServer)
	log.Printf("Management UI: http://localhost%s/v1/proxy", addr)
	log.Printf("REST API: http://localhost%s/v1/proxy", addr)
	log.Printf("Note: Reverse proxy is handled by Apache httpd (port 8080)")
	log.Printf("Config file: /etc/httpd/conf.d/dynamic-proxy.conf")

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		log.Println("Shutting down server...")
		srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
