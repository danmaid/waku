package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultTLSDir はTLS証明書の保存先ディレクトリ
	DefaultTLSDir = "/etc/httpd/tls"
	// CACommonName はCA証明書のCommon Name
	CACommonName = "Dynamic Proxy CA"
	// CertValidityYears は証明書の有効期限（年）
	CertValidityYears = 10
)

// Manager はTLS証明書を管理する
type Manager struct {
	tlsDir      string
	caCert      *x509.Certificate
	caKey       *rsa.PrivateKey
	caCertPEM   []byte
	caCertPath  string
	caKeyPath   string
}

// NewManager はTLS証明書マネージャーを作成
func NewManager(tlsDir string) (*Manager, error) {
	if tlsDir == "" {
		tlsDir = DefaultTLSDir
	}

	m := &Manager{
		tlsDir:     tlsDir,
		caCertPath: filepath.Join(tlsDir, "ca.crt"),
		caKeyPath:  filepath.Join(tlsDir, "ca.key"),
	}

	// TLSディレクトリを作成
	if err := os.MkdirAll(tlsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create TLS directory: %w", err)
	}

	// CA証明書を読み込みまたは生成
	if err := m.loadOrGenerateCA(); err != nil {
		return nil, err
	}

	return m, nil
}

// loadOrGenerateCA はCA証明書を読み込みまたは生成
func (m *Manager) loadOrGenerateCA() error {
	// 既存のCA証明書と秘密鍵をチェック
	if _, err := os.Stat(m.caCertPath); err == nil {
		if _, err := os.Stat(m.caKeyPath); err == nil {
			// 既存のCA証明書を読み込み
			return m.loadCA()
		}
	}

	// CA証明書が存在しない場合は生成
	log.Println("[TLS] Generating new CA certificate...")
	return m.generateCA()
}

// loadCA は既存のCA証明書と秘密鍵を読み込み
func (m *Manager) loadCA() error {
	// CA証明書を読み込み
	certPEM, err := os.ReadFile(m.caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}

	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// CA秘密鍵を読み込み
	keyPEM, err := os.ReadFile(m.caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read CA key: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode CA key PEM")
	}

	caKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA key: %w", err)
	}

	m.caCert = caCert
	m.caKey = caKey
	m.caCertPEM = certPEM

	log.Printf("[TLS] Loaded existing CA certificate from %s", m.caCertPath)
	return nil
}

// generateCA は新しいCA証明書と秘密鍵を生成
func (m *Manager) generateCA() error {
	// RSA秘密鍵を生成
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	// CA証明書のテンプレート
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	caTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   CACommonName,
			Organization: []string{"Dynamic Proxy"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(CertValidityYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// 自己署名CA証明書を作成
	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertBytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	// CA証明書をPEM形式で保存
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertBytes,
	})
	if err := os.WriteFile(m.caCertPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	// CA秘密鍵をPEM形式で保存
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caKey),
	})
	if err := os.WriteFile(m.caKeyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write CA key: %w", err)
	}

	m.caCert = caCert
	m.caKey = caKey
	m.caCertPEM = certPEM

	log.Printf("[TLS] Generated new CA certificate: %s", m.caCertPath)
	log.Printf("[TLS] CA certificate valid until: %s", caCert.NotAfter.Format(time.RFC3339))
	return nil
}

// GetCACertPEM はCA証明書のPEMデータを取得
func (m *Manager) GetCACertPEM() []byte {
	return m.caCertPEM
}

// GenerateServerCert はホスト名用のサーバ証明書を生成
func (m *Manager) GenerateServerCert(hostname string) error {
	certPath := m.GetCertPath(hostname)
	keyPath := m.GetKeyPath(hostname)

	// 既存の証明書をチェック（既存の場合はスキップ）
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			log.Printf("[TLS] Server certificate already exists for %s", hostname)
			return nil
		}
	}

	log.Printf("[TLS] Generating server certificate for %s...", hostname)

	// RSA秘密鍵を生成
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate server key: %w", err)
	}

	// サーバ証明書のテンプレート
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	serverTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: hostname,
		},
		DNSNames:    []string{hostname},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(CertValidityYears, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// CAで署名されたサーバ証明書を作成
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, m.caCert, &serverKey.PublicKey, m.caKey)
	if err != nil {
		return fmt.Errorf("failed to create server certificate: %w", err)
	}

	// サーバ証明書をPEM形式で保存
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertBytes,
	})
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write server certificate: %w", err)
	}

	// サーバ秘密鍵をPEM形式で保存
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverKey),
	})
	// パーミッション0640: 所有者読取・書入、グループ読取（httpd/apacheが読める）
	if err := os.WriteFile(keyPath, keyPEM, 0640); err != nil {
		return fmt.Errorf("failed to write server key: %w", err)
	}

	log.Printf("[TLS] Generated server certificate: %s", certPath)
	return nil
}

// DeleteServerCert はホスト名用のサーバ証明書を削除
func (m *Manager) DeleteServerCert(hostname string) error {
	certPath := m.GetCertPath(hostname)
	keyPath := m.GetKeyPath(hostname)

	// 証明書と秘密鍵を削除
	if err := os.Remove(certPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete certificate: %w", err)
	}

	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	log.Printf("[TLS] Deleted server certificate for %s", hostname)
	return nil
}

// GetCertPath はホスト名用の証明書パスを取得
func (m *Manager) GetCertPath(hostname string) string {
	return filepath.Join(m.tlsDir, hostname+".crt")
}

// GetKeyPath はホスト名用の秘密鍵パスを取得
func (m *Manager) GetKeyPath(hostname string) string {
	return filepath.Join(m.tlsDir, hostname+".key")
}

// GetCACertPath はCA証明書のパスを取得
func (m *Manager) GetCACertPath() string {
	return m.caCertPath
}
