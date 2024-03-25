package types

import (
	"crypto/x509"
	"errors"
	"strings"
)

var (
	// Invalid client validation method
	ErrInvalidClientValidation = errors.New("invalid client validation method")
)

type Validation int

type ValidationMethod interface {
	VerifyVerifiedPeerCertificates(verifiedChains [][]*x509.Certificate) error
	VerifyRawPeerCertificates(peerCertificates []*x509.Certificate) error
	String() string
}

const (
	OCSP Validation = iota + 1
	CRL
)

func (v Validation) String() string {
	switch v {
	case OCSP:
		return "OCSP"
	case CRL:
		return "CRL"
	default:
		return ""
	}
}

func ParseValidation(v string) (Validation, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	switch v {
	case "OCSP":
		return OCSP, nil
	case "CRL":
		return CRL, nil
	default:
		return 0, ErrInvalidClientValidation
	}
}
