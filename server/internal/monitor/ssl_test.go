package monitor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/serverwatch/serverwatch/server/internal/service"
)

func TestSSLCheckerValidCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	result := (&SSLChecker{RootCAs: roots}).CheckCertificate(context.Background(), service.Monitor{
		ID: 1, Target: server.Listener.Addr().String(), TimeoutSeconds: 1,
	})

	if result.Result.Status != "up" || !result.CertificateValid {
		t.Fatalf("result = %+v, want verified up result", result)
	}
	if result.Subject == "" || result.Issuer == "" || result.NotAfter.IsZero() {
		t.Fatalf("certificate fields not populated: %+v", result)
	}
	if result.DaysRemaining <= 0 {
		t.Fatalf("days remaining = %d, want positive", result.DaysRemaining)
	}
}

func TestSSLCheckerExpiredCertificate(t *testing.T) {
	server := newCertificateServer(t, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
	defer server.Close()

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	result := (&SSLChecker{RootCAs: roots}).CheckCertificate(context.Background(), service.Monitor{
		ID: 2, Target: server.Listener.Addr().String(), TimeoutSeconds: 1,
	})

	if result.Result.Status != "down" || result.Result.Message == "" {
		t.Fatalf("result = %+v, want expired certificate failure", result.Result)
	}
}

func TestSSLCheckerInvalidCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	result := NewSSLChecker().CheckCertificate(context.Background(), service.Monitor{
		ID: 3, Target: server.Listener.Addr().String(), TimeoutSeconds: 1,
	})
	if result.Result.Status != "down" || result.Result.Message == "" {
		t.Fatalf("result = %+v, want verification failure", result.Result)
	}
}

func TestParseSSLAddress(t *testing.T) {
	tests := []struct {
		name, target, want string
	}{
		{"default port", "example.com", "example.com:443"},
		{"explicit port", "example.com:8443", "example.com:8443"},
		{"IPv6", "[2001:db8::1]:443", "[2001:db8::1]:443"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, err := ParseSSLAddress(test.target)
			if err != nil || address != test.want {
				t.Fatalf("ParseSSLAddress(%q) = %q, %v; want %q", test.target, address, err, test.want)
			}
		})
	}

	for _, target := range []string{":", "example.com:", "bad:target:format"} {
		if _, err := ParseSSLAddress(target); err == nil {
			t.Fatalf("ParseSSLAddress(%q) succeeded, want error", target)
		}
	}
}

func newCertificateServer(t *testing.T, notBefore, notAfter time.Time) *httptest.Server {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: notBefore, NotAfter: notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	return server
}
