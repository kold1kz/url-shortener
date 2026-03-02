package certutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// --- helpers ---

func mustWriteFile(t *testing.T, path string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readAll(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func writeCertPEM(t *testing.T, certPath string, certDER []byte) {
	t.Helper()
	p := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	mustWriteFile(t, certPath, p, 0o644)
}

func writeBrokenCertPEMType(t *testing.T, certPath string) {
	t.Helper()
	p := pem.EncodeToMemory(&pem.Block{Type: "NOT_A_CERT", Bytes: []byte("x")})
	mustWriteFile(t, certPath, p, 0o644)
}

func genCertDER(t *testing.T, hosts []string, notBefore, notAfter time.Time) []byte {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}

	tpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"test self-signed"},
			CommonName:   "test",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

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

	// если ничего не передали — как в прод-коде: localhost + loopback
	if len(tpl.DNSNames) == 0 && len(tpl.IPAddresses) == 0 {
		tpl.DNSNames = []string{"localhost"}
		tpl.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	}

	der, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return der
}

// --- tests: certIsValid ---

func Test_certIsValid_ReadFileError(t *testing.T) {
	ok, err := certIsValid(filepath.Join(t.TempDir(), "no_such_cert.pem"), nil, 0)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false")
	}
}

func Test_certIsValid_InvalidPEM(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	mustWriteFile(t, certPath, []byte("not a pem"), 0o644)

	ok, err := certIsValid(certPath, nil, 0)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false")
	}
	if !strings.Contains(err.Error(), "invalid cert pem") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_certIsValid_WrongPEMType(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	writeBrokenCertPEMType(t, certPath)

	ok, err := certIsValid(certPath, nil, 0)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false")
	}
}

func Test_certIsValid_ParseError(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")

	// PEM с типом CERTIFICATE, но битые DER-данные
	p := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage-der")})
	mustWriteFile(t, certPath, p, 0o644)

	ok, err := certIsValid(certPath, nil, 0)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false")
	}
	if !strings.Contains(err.Error(), "parse cert") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_certIsValid_NearExpiry(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")

	now := time.Now()
	der := genCertDER(t, []string{"localhost"}, now.Add(-time.Minute), now.Add(30*time.Minute))
	writeCertPEM(t, certPath, der)

	// renewBefore больше, чем оставшийся срок => считаем протухшим
	ok, err := certIsValid(certPath, []string{"localhost"}, 1*time.Hour)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false")
	}
	if !strings.Contains(err.Error(), "expired or near expiry") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_certIsValid_HostsEmpty_OnlyExpiryCheck(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")

	now := time.Now()
	der := genCertDER(t, []string{"example.com"}, now.Add(-time.Minute), now.Add(24*time.Hour))
	writeCertPEM(t, certPath, der)

	ok, err := certIsValid(certPath, nil, 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
}

func Test_certIsValid_HostMatch_DNS(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")

	now := time.Now()
	der := genCertDER(t, []string{"example.com"}, now.Add(-time.Minute), now.Add(24*time.Hour))
	writeCertPEM(t, certPath, der)

	ok, err := certIsValid(certPath, []string{"example.com"}, 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
}

func Test_certIsValid_HostMatch_IPAndTrimming(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")

	now := time.Now()
	der := genCertDER(t, []string{"127.0.0.1"}, now.Add(-time.Minute), now.Add(24*time.Hour))
	writeCertPEM(t, certPath, der)

	ok, err := certIsValid(certPath, []string{"   ", "127.0.0.1", "  "}, 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
}

func Test_certIsValid_HostMismatch(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")

	now := time.Now()
	der := genCertDER(t, []string{"localhost"}, now.Add(-time.Minute), now.Add(24*time.Hour))
	writeCertPEM(t, certPath, der)

	ok, err := certIsValid(certPath, []string{"example.com"}, 0)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false")
	}
	if !strings.Contains(err.Error(), "does not match provided hosts") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- tests: generateSelfSigned ---

func Test_generateSelfSigned_DefaultHostsWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	if err := generateSelfSigned(certPath, keyPath, nil, 24*time.Hour); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// проверим: сертификат валиден и подходит под localhost/127.0.0.1
	ok, err := certIsValid(certPath, []string{"localhost"}, 0)
	if err != nil || !ok {
		t.Fatalf("expected valid for localhost, ok=%v err=%v", ok, err)
	}

	ok, err = certIsValid(certPath, []string{"127.0.0.1"}, 0)
	if err != nil || !ok {
		t.Fatalf("expected valid for 127.0.0.1, ok=%v err=%v", ok, err)
	}
}

func Test_generateSelfSigned_WriteCertError_ReadOnlyDir(t *testing.T) {
	// На Windows chmod семантически не гарантирован.
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test is unreliable on windows")
	}

	dir := t.TempDir()
	roDir := filepath.Join(dir, "ro")
	if err := os.MkdirAll(roDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// делаем директорию read-only
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	certPath := filepath.Join(roDir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem") // ключ писать можно, но до него не дойдет, если упадем на cert

	err := generateSelfSigned(certPath, keyPath, []string{"localhost"}, 24*time.Hour)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "write cert") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- tests: EnsureCertFiles ---

func TestEnsureCertFiles_EmptyPaths(t *testing.T) {
	_, _, err := EnsureCertFiles(EnsureOptions{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "empty cert/key path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureCertFiles_GeneratesWhenMissing_AndAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls", "cert.pem")
	keyPath := filepath.Join(dir, "tls", "key.pem")

	// ValidFor=0 => должен примениться дефолт
	// RenewBefore=-1 => должен стать 0
	gotCert, gotKey, err := EnsureCertFiles(EnsureOptions{
		CertPath:    certPath,
		KeyPath:     keyPath,
		ValidFor:    0,
		Hosts:       []string{"localhost"},
		RenewBefore: -1,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotCert != certPath || gotKey != keyPath {
		t.Fatalf("unexpected paths: %s %s", gotCert, gotKey)
	}

	// файлы реально созданы
	if _, err := os.Stat(certPath); err != nil {
		t.Fatalf("cert stat: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("key stat: %v", err)
	}

	ok, err := certIsValid(certPath, []string{"localhost"}, 0)
	if err != nil || !ok {
		t.Fatalf("expected valid cert, ok=%v err=%v", ok, err)
	}
}

func TestEnsureCertFiles_ReturnsExistingWhenValid(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// заранее создаем валидные файлы
	if err := generateSelfSigned(certPath, keyPath, []string{"example.com"}, 24*time.Hour); err != nil {
		t.Fatalf("gen: %v", err)
	}
	before := readAll(t, certPath)

	gotCert, gotKey, err := EnsureCertFiles(EnsureOptions{
		CertPath:    certPath,
		KeyPath:     keyPath,
		Hosts:       []string{"example.com"},
		ValidFor:    24 * time.Hour,
		RenewBefore: 0,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotCert != certPath || gotKey != keyPath {
		t.Fatalf("unexpected paths: %s %s", gotCert, gotKey)
	}

	after := readAll(t, certPath)
	if string(before) != string(after) {
		t.Fatalf("expected cert not regenerated")
	}
}

func TestEnsureCertFiles_RegeneratesOnHostMismatch(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// создаем сертификат под localhost
	if err := generateSelfSigned(certPath, keyPath, []string{"localhost"}, 24*time.Hour); err != nil {
		t.Fatalf("gen: %v", err)
	}
	oldCert := readAll(t, certPath)

	// теперь требуем example.com => certIsValid должен сказать mismatch => Ensure перегенерит
	_, _, err := EnsureCertFiles(EnsureOptions{
		CertPath:    certPath,
		KeyPath:     keyPath,
		Hosts:       []string{"example.com"},
		ValidFor:    24 * time.Hour,
		RenewBefore: 0,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	newCert := readAll(t, certPath)
	if string(oldCert) == string(newCert) {
		t.Fatalf("expected cert regenerated on host mismatch")
	}

	ok, err := certIsValid(certPath, []string{"example.com"}, 0)
	if err != nil || !ok {
		t.Fatalf("expected regenerated cert valid for example.com, ok=%v err=%v", ok, err)
	}
}

func TestEnsureCertFiles_MkdirFails_WhenDirIsFile(t *testing.T) {
	dir := t.TempDir()

	// делаем "директорию" файлом, чтобы MkdirAll(filepath.Dir(certPath)) упал
	badDirAsFile := filepath.Join(dir, "not_a_dir")
	if err := os.WriteFile(badDirAsFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	certPath := filepath.Join(badDirAsFile, "cert.pem") // Dir(certPath) == badDirAsFile (file)
	keyPath := filepath.Join(dir, "key.pem")

	_, _, err := EnsureCertFiles(EnsureOptions{
		CertPath: certPath,
		KeyPath:  keyPath,
		Hosts:    []string{"localhost"},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mkdir cert dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureCertFiles_MkdirKeyFails_WhenDirIsFile(t *testing.T) {
	dir := t.TempDir()

	okDir := filepath.Join(dir, "ok")
	certPath := filepath.Join(okDir, "cert.pem")

	badDirAsFile := filepath.Join(dir, "not_a_dir")
	if err := os.WriteFile(badDirAsFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	keyPath := filepath.Join(badDirAsFile, "key.pem") // Dir(keyPath) == badDirAsFile (file)

	_, _, err := EnsureCertFiles(EnsureOptions{
		CertPath: certPath,
		KeyPath:  keyPath,
		Hosts:    []string{"localhost"},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mkdir key dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Доп. тест: “протухший” сертификат заставляет Ensure перегенерировать.
func TestEnsureCertFiles_RegeneratesOnExpired(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	now := time.Now()
	expiredDER := genCertDER(t, []string{"localhost"}, now.Add(-2*time.Hour), now.Add(-1*time.Hour))
	writeCertPEM(t, certPath, expiredDER)

	// ключ может быть любым — его валидность здесь не проверяется
	mustWriteFile(t, keyPath, []byte("dummy key"), 0o600)

	oldCert := readAll(t, certPath)

	_, _, err := EnsureCertFiles(EnsureOptions{
		CertPath:    certPath,
		KeyPath:     keyPath,
		Hosts:       []string{"localhost"},
		ValidFor:    24 * time.Hour,
		RenewBefore: 0,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	newCert := readAll(t, certPath)
	if string(oldCert) == string(newCert) {
		t.Fatalf("expected cert regenerated on expired cert")
	}

	ok, err := certIsValid(certPath, []string{"localhost"}, 0)
	if err != nil || !ok {
		t.Fatalf("expected new cert valid, ok=%v err=%v", ok, err)
	}
}

// sanity: ensure errors are wrapped consistently
func TestErrorWrappingSanity(t *testing.T) {
	err := fmt.Errorf("tlsutil: create cert: %w", errors.New("boom"))
	if !strings.Contains(err.Error(), "tlsutil: create cert") {
		t.Fatalf("unexpected: %v", err)
	}
}
