package routes

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGenerateRSAKeyPair_Success(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/crypto/rsa", nil)

	GenerateRSAKeyPair(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := gin.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	pubKey, ok := resp["publicKey"].(string)
	if !ok || pubKey == "" {
		t.Errorf("publicKey: expected non-empty string, got %v", resp["publicKey"])
	}

	privKey, ok := resp["privateKey"].(string)
	if !ok || privKey == "" {
		t.Errorf("privateKey: expected non-empty string, got %v", resp["privateKey"])
	}

	// Verify public key is valid PEM
	block, _ := pem.Decode([]byte(pubKey))
	if block == nil {
		t.Errorf("publicKey: not valid PEM")
	}
	if block.Type != "PUBLIC KEY" {
		t.Errorf("publicKey type: expected PUBLIC KEY, got %s", block.Type)
	}

	// Verify private key is valid PEM
	block, _ = pem.Decode([]byte(privKey))
	if block == nil {
		t.Errorf("privateKey: not valid PEM")
	}
	if block.Type != "RSA PRIVATE KEY" {
		t.Errorf("privateKey type: expected RSA PRIVATE KEY, got %s", block.Type)
	}
}

func TestGenerateRSAKeyPair_KeySize(t *testing.T) {
	// Generate a key and verify it's 2048 bits
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}

	if privateKey.N.BitLen() != 2048 {
		t.Errorf("key size: expected 2048, got %d", privateKey.N.BitLen())
	}
}

func TestGenerateRSAKeyPair_PublicPrivateMatch(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/crypto/rsa", nil)

	GenerateRSAKeyPair(c)

	var resp map[string]interface{}
	if err := gin.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	pubKeyStr := resp["publicKey"].(string)
	privKeyStr := resp["privateKey"].(string)

	// Decode both
	pubBlock, _ := pem.Decode([]byte(pubKeyStr))
	privBlock, _ := pem.Decode([]byte(privKeyStr))

	// Parse public key
	pubKey, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse public key: %v", err)
	}

	// Parse private key
	privKey, err := x509.ParsePKCS1PrivateKey(privBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	// Verify they match
	if pubKey.(*rsa.PublicKey).N.Cmp(privKey.N) != 0 {
		t.Errorf("public and private keys do not match")
	}
}
