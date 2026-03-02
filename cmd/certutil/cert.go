package certutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EnsureOptions struct {
	// куда сохранять
	CertPath string
	KeyPath  string

	// на сколько выпускать сертификат
	ValidFor time.Duration

	// SAN: например "localhost", "127.0.0.1", "::1"
	Hosts []string

	// за сколько до истечения считать "протухшим"
	RenewBefore time.Duration
}

// EnsureCertFiles гарантирует наличие валидных cert/key файлов.
// Если файлов нет / сертификат протух или не парсится - создает новые.
func EnsureCertFiles(opts EnsureOptions) (certPath, keyPath string, err error) {
	if opts.CertPath == "" || opts.KeyPath == "" {
		return "", "", fmt.Errorf("tlsutil: empty cert/key path")
	}
	if opts.ValidFor <= 0 {
		opts.ValidFor = 365 * 24 * time.Hour
	}
	if opts.RenewBefore < 0 {
		opts.RenewBefore = 0
	}

	// есть ли файлы и валиден ли сертификат
	ok, err := certIsValid(opts.CertPath, opts.Hosts, opts.RenewBefore)
	if err == nil && ok {
		return opts.CertPath, opts.KeyPath, nil
	}

	// если сертификат плохой - пробуем перегенерировать
	if err := os.MkdirAll(filepath.Dir(opts.CertPath), 0o755); err != nil {
		return "", "", fmt.Errorf("tlsutil: mkdir cert dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(opts.KeyPath), 0o755); err != nil {
		return "", "", fmt.Errorf("tlsutil: mkdir key dir: %w", err)
	}

	if err := generateSelfSigned(opts.CertPath, opts.KeyPath, opts.Hosts, opts.ValidFor); err != nil {
		return "", "", err
	}
	return opts.CertPath, opts.KeyPath, nil
}

func certIsValid(certPath string, hosts []string, renewBefore time.Duration) (bool, error) {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return false, err
	}
	block, _ := pem.Decode(b)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, fmt.Errorf("tlsutil: invalid cert pem")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("tlsutil: parse cert: %w", err)
	}

	now := time.Now()
	// протух или скоро протухнет
	if now.After(cert.NotAfter.Add(-renewBefore)) {
		return false, fmt.Errorf("tlsutil: cert expired or near expiry (notAfter=%s)", cert.NotAfter.Format(time.RFC3339))
	}

	// если hosts не заданы - только срок проверяем
	if len(hosts) == 0 {
		return true, nil
	}

	// проверяем, что сертификат подходит хотя бы под один host
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			if err := cert.VerifyHostname(ip.String()); err == nil {
				return true, nil
			}
			continue
		}
		if err := cert.VerifyHostname(h); err == nil {
			return true, nil
		}
	}

	return false, fmt.Errorf("tlsutil: cert does not match provided hosts")
}

func generateSelfSigned(certPath, keyPath string, hosts []string, validFor time.Duration) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("tlsutil: generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("tlsutil: serial: %w", err)
	}

	notBefore := time.Now().Add(-1 * time.Minute)
	notAfter := notBefore.Add(validFor)

	tpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"url-shortener dev self-signed"},
			CommonName:   "url-shortener",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// SAN
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, ip)
		} else {
			tpl.DNSNames = append(tpl.DNSNames, h)
		}
	}
	// по дефолту добавим localhost, если ничего не передали
	if len(tpl.DNSNames) == 0 && len(tpl.IPAddresses) == 0 {
		tpl.DNSNames = []string{"localhost"}
		tpl.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	}

	der, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("tlsutil: create cert: %w", err)
	}

	// write cert
	if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	}), 0o644); err != nil {
		return fmt.Errorf("tlsutil: write cert: %w", err)
	}

	// write key (EC PRIVATE KEY)
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("tlsutil: marshal key: %w", err)
	}
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	}), 0o600); err != nil {
		return fmt.Errorf("tlsutil: write key: %w", err)
	}

	return nil
}
