package routes

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GenerateRSAKeyPair generates an RSA key pair and returns the public/private keys in PEM format.
// Public key is in PKCS#8 (SubjectPublicKeyInfo) format for Kong JWT plugin compatibility.
func GenerateRSAKeyPair(c *gin.Context) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		internalError(c)
		return
	}

	// Encode private key to PKCS#8 PEM
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key to PKCS#8 PEM (SubjectPublicKeyInfo) — Go 1.18 compatible
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		internalError(c)
		return
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	c.JSON(http.StatusOK, gin.H{
		"publicKey":  string(publicKeyPEM),
		"privateKey": string(privateKeyPEM),
	})
}