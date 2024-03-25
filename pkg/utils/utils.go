package utils

import (
	"crypto/x509"
	"fmt"

	"github.com/absmach/mproxy/pkg/tls"
)

type ValidationMethod interface {
	VerifyVerifiedPeerCertificates(verifiedChains [][]*x509.Certificate) error
	VerifyRawPeerCertificates(peerCertificates []*x509.Certificate) error
}

func SecurityStatus(c tls.Config) string {
	if c.CertFile == "" && c.KeyFile == "" {
		return "TLS"
	}
	if c.ClientCAFile != "" {
		if methods := c.Verifier.Methods(); methods != "" {
			return fmt.Sprintf("mTLS with client verification %s", methods)
		}
		return "mTLS"
	}
	return "no TLS"
}
