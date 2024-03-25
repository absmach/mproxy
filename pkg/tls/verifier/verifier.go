// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"crypto/x509"
	"errors"
	"strings"

	"github.com/absmach/mproxy/pkg/tls/verifier/types"
	"github.com/absmach/mproxy/pkg/tls/verifier/validation"
	"github.com/caarlos0/env/v10"
)

var (
	errParseCert = errors.New("failed to parse Certificate")
	errClientCrt = errors.New("client certificate not received")
)

type Verifier interface {
	// VerifyPeerCertificate...
	VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
	IsThereVerifiers() bool
	Methods() string
}

type config struct {
	validationMethods []types.ValidationMethod
}

var _ Verifier = (*config)(nil)

func New(opts env.Options) (Verifier, error) {
	vms, err := validation.NewValidationMethods(opts)
	if err != nil {
		return nil, err
	}
	return &config{vms}, nil
}

func (c *config) Methods() string {
	methods := []string{}
	for _, vm := range c.validationMethods {
		methods = append(methods, vm.String())
	}
	return strings.Join(methods, ",")
}

// Client certificate verification fails when there is partial certificates of either verifiedChains or rawCerts.
func (c *config) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	switch {
	case len(verifiedChains) > 0:
		return c.verifyVerifiedPeerCertificates(verifiedChains)
	case len(rawCerts) > 0:
		return c.verifyRawPeerCertificates(rawCerts)
	default:
		return errClientCrt
	}
}

func (c *config) IsThereVerifiers() bool {
	return len(c.validationMethods) > 0
}

func (c *config) verifyVerifiedPeerCertificates(verifiedChains [][]*x509.Certificate) error {
	for _, vm := range c.validationMethods {
		if err := vm.VerifyVerifiedPeerCertificates(verifiedChains); err != nil {
			return err
		}
	}
	return nil
}

func (c *config) verifyRawPeerCertificates(rawCerts [][]byte) error {
	var peerCertificates []*x509.Certificate
	peerCertificates, err := c.parseCertificates(rawCerts)
	if err != nil {
		return err
	}
	for _, vm := range c.validationMethods {
		if err := vm.VerifyRawPeerCertificates(peerCertificates); err != nil {
			return err
		}
	}
	return nil
}

func (c *config) parseCertificates(rawCerts [][]byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for _, rawCert := range rawCerts {
		cert, err := x509.ParseCertificate(rawCert)
		if err != nil {
			return nil, errors.Join(errParseCert, err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}
