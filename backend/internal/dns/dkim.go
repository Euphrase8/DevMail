package dns

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"github.com/Euphrase8/DevMail/backend/internal/secretbox"
)

// GenerateDKIM creates an RSA-2048 keypair for a domain, stores the private key
// encrypted at rest, publishes the public key as a TXT record value, and
// returns the DKIM record that should exist in DNS.
func (s *Service) GenerateDKIM(ctx context.Context, domainID, selector string) (string, Record, error) {
	facts, err := s.loadFacts(ctx, domainID)
	if err != nil {
		return "", Record{}, err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", Record{}, err
	}
	derPub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", Record{}, err
	}
	publicKey := base64.StdEncoding.EncodeToString(derPub)

	privDER := x509.MarshalPKCS1PrivateKey(key)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER})
	encPriv, err := secretbox.Seal(s.passphrase, string(privPEM))
	if err != nil {
		return "", Record{}, err
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE domains SET dkim_selector=$2, dkim_private_key=$3, dkim_public_key=$4, updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL`, domainID, selector, encPriv, publicKey); err != nil {
		return "", Record{}, err
	}

	record := Record{
		Type: "TXT", Name: selector + "._domainkey." + facts.name,
		Value: "v=DKIM1; k=rsa; p=" + publicKey, TTL: 3600, Purpose: "dkim", Status: "planned",
	}
	return publicKey, record, nil
}
