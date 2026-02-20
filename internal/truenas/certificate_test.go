package truenas

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func generateSelfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"test.example.com"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	return cert
}

func TestEncodeChainPEM(t *testing.T) {
	t.Parallel()
	cert := generateSelfSignedCert(t)
	pemStr := encodeChainPEM(cert)

	if !strings.HasPrefix(pemStr, "-----BEGIN CERTIFICATE-----") {
		t.Errorf("expected PEM certificate, got: %s", pemStr[:50])
	}

	block, rest := pem.Decode([]byte(pemStr))
	if block == nil {
		t.Fatal("pem.Decode returned nil block")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("expected CERTIFICATE block, got %s", block.Type)
	}
	if len(rest) != 0 {
		t.Errorf("expected no remaining bytes after decoding single cert, got %d bytes", len(rest))
	}
}

func TestEncodePrivateKeyPEM(t *testing.T) {
	t.Parallel()
	cert := generateSelfSignedCert(t)
	pemStr, err := encodePrivateKeyPEM(cert)
	if err != nil {
		t.Fatalf("encodePrivateKeyPEM: %v", err)
	}

	if !strings.HasPrefix(pemStr, "-----BEGIN PRIVATE KEY-----") {
		t.Errorf("expected PRIVATE KEY PEM, got: %s", pemStr[:50])
	}

	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		t.Fatal("pem.Decode returned nil block")
	}
	if block.Type != "PRIVATE KEY" {
		t.Errorf("expected PRIVATE KEY block, got %s", block.Type)
	}
}

func TestCertificateTLSCertificate(t *testing.T) {
	t.Parallel()
	original := generateSelfSignedCert(t)

	certPEM := encodeChainPEM(original)
	keyPEM, err := encodePrivateKeyPEM(original)
	if err != nil {
		t.Fatalf("encodePrivateKeyPEM: %v", err)
	}

	c := &Certificate{
		Certificate: certPEM,
		Privatekey:  keyPEM,
	}

	tlsCert, err := c.TLSCertificate()
	if err != nil {
		t.Fatalf("TLSCertificate: %v", err)
	}

	if tlsCert.Leaf == nil {
		t.Fatal("expected Leaf to be set")
	}

	if tlsCert.Leaf.Subject.CommonName != "test.example.com" {
		t.Errorf("expected CN=test.example.com, got %s", tlsCert.Leaf.Subject.CommonName)
	}

	if !tlsCert.Leaf.Equal(original.Leaf) {
		t.Error("parsed leaf does not equal original leaf")
	}
}

func TestCertificateTLSCertificate_InvalidPEM(t *testing.T) {
	t.Parallel()
	c := &Certificate{
		Certificate: "not valid pem",
		Privatekey:  "not valid pem",
	}

	_, err := c.TLSCertificate()
	if err == nil {
		t.Error("expected error for invalid PEM, got nil")
	}
}

func TestEncodeChainPEM_MultiCert(t *testing.T) {
	t.Parallel()
	// Build a cert chain with two certs by duplicating the certificate bytes.
	cert := generateSelfSignedCert(t)
	// Add a second entry to simulate a chain (intermediate).
	cert.Certificate = append(cert.Certificate, cert.Certificate[0])

	pemStr := encodeChainPEM(cert)

	// Should contain two PEM blocks.
	count := strings.Count(pemStr, "-----BEGIN CERTIFICATE-----")
	if count != 2 {
		t.Errorf("expected 2 certificate PEM blocks, got %d", count)
	}
}
