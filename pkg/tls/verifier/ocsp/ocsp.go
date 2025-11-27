// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package ocsp

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/absmach/mproxy/pkg/tls/verifier"
	"github.com/caarlos0/env/v11"
	"golang.org/x/crypto/ocsp"
)

var (
	errParseIssuerCrt       = errors.New("failed to parse issuer certificate")
	errCreateOCSPReq        = errors.New("failed to create OCSP Request")
	errCreateOCSPHTTPReq    = errors.New("failed to create OCSP HTTP Request")
	errParseOCSPUrl         = errors.New("failed to parse OCSP server URL")
	errOCSPReq              = errors.New("OCSP request failed")
	errOCSPReadResp         = errors.New("failed to read OCSP response")
	errParseOCSPRespForCert = errors.New("failed to parse OCSP Response for Certificate")
	errIssuerCert           = errors.New("neither the issuer certificate is present in the chain nor is the issuer certificate URL present in AIA")
	errNoOCSPURL            = errors.New("neither OCSP Server/Responder URL is not present AIA of certificate nor environmental variable OCSP_RESPONDER_URL have value")
	errOCSPServerFailed     = errors.New("OCSP Server Failed")
	errOCSPUnknown          = errors.New("OCSP status unknown")
	errCertRevoked          = errors.New("certificate revoked")
	errRetrieveIssuerCrt    = errors.New("failed to retrieve issuer certificate")
	errReadIssuerCrt        = errors.New("failed to read issuer certificate")
	errIssuerCrtPEM         = errors.New("failed to decode issuer certificate PEM")

	errParseCert = errors.New("failed to parse Certificate")
	errClientCrt = errors.New("client certificate not received")
)

type config struct {
	OCSPDepth        uint    `env:"OCSP_DEPTH"         envDefault:"0"`
	OCSPResponderURL url.URL `env:"OCSP_RESPONDER_URL" envDefault:""`
}

var _ verifier.Verifier = (*config)(nil)

func New(opts env.Options) (verifier.Verifier, error) {
	var c config
	if err := env.ParseWithOptions(&c, opts); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *config) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	switch {
	case len(verifiedChains) > 0:
		return c.VerifyVerifiedPeerCertificates(verifiedChains)
	case len(rawCerts) > 0:
		var peerCertificates []*x509.Certificate
		peerCertificates, err := parseCertificates(rawCerts)
		if err != nil {
			return err
		}
		return c.VerifyRawPeerCertificates(peerCertificates)
	default:
		return errClientCrt
	}
}

func (c *config) VerifyRawPeerCertificates(peerCertificates []*x509.Certificate) error {
	for i, peerCertificate := range peerCertificates {
		issuer := retrieveIssuerCert(peerCertificate.Issuer, peerCertificates)
		if err := c.ocspVerify(peerCertificate, issuer); err != nil {
			return err
		}
		if i+1 == int(c.OCSPDepth) {
			return nil
		}
	}
	return nil
}

func (c *config) VerifyVerifiedPeerCertificates(verifiedPeerCertificateChains [][]*x509.Certificate) error {
	for _, verifiedChain := range verifiedPeerCertificateChains {
		for i := range verifiedChain {
			cert := verifiedChain[i]
			issuer := cert
			if i+1 < len(verifiedChain) {
				issuer = verifiedChain[i+1]
			}
			if err := c.ocspVerify(cert, issuer); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *config) ocspVerify(peerCertificate, issuerCert *x509.Certificate) error {
	opts := &ocsp.RequestOptions{Hash: crypto.SHA256}
	var err error

	if !isRootCA(peerCertificate) {
		if issuerCert == nil {
			if len(peerCertificate.IssuingCertificateURL) < 1 {
				return fmt.Errorf("%w common name %s  and serial number %x", errIssuerCert, peerCertificate.Subject.CommonName, peerCertificate.SerialNumber)
			}
			issuerCert, err = retrieveIssuingCertificate(peerCertificate.IssuingCertificateURL[0])
			if err != nil {
				return err
			}
		}
	} else {
		issuerCert = peerCertificate
	}

	buffer, err := ocsp.CreateRequest(peerCertificate, issuerCert, opts)
	if err != nil {
		return errors.Join(errCreateOCSPReq, err)
	}

	ocspURL := ""
	ocspURLHost := ""
	if c.OCSPResponderURL.String() == "" {
		if len(peerCertificate.OCSPServer) < 1 {
			return fmt.Errorf("%w common name %s and serial number %x", errNoOCSPURL, peerCertificate.Subject.CommonName, peerCertificate.SerialNumber)
		}
		ocspURL = peerCertificate.OCSPServer[0]
		ocspParsedURL, err := url.Parse(peerCertificate.OCSPServer[0])
		if err != nil {
			return errors.Join(errParseOCSPUrl, err)
		}
		ocspURLHost = ocspParsedURL.Host
	} else {
		ocspURLHost = c.OCSPResponderURL.Host
		ocspURL = c.OCSPResponderURL.String()
	}

	httpRequest, err := http.NewRequest(http.MethodPost, ocspURL, bytes.NewBuffer(buffer))
	if err != nil {
		return errors.Join(errCreateOCSPHTTPReq, err)
	}
	httpRequest.Header.Add("Content-Type", "application/ocsp-request")
	httpRequest.Header.Add("Accept", "application/ocsp-response")
	httpRequest.Header.Add("host", ocspURLHost)

	httpClient := &http.Client{}
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return errors.Join(errOCSPReq, err)
	}
	defer httpResponse.Body.Close()
	output, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return errors.Join(errOCSPReadResp, err)
	}
	ocspResponse, err := ocsp.ParseResponseForCert(output, peerCertificate, issuerCert)
	if err != nil {
		return errors.Join(errParseOCSPRespForCert, err)
	}
	switch ocspResponse.Status {
	case ocsp.Good:
		return nil
	case ocsp.Revoked:
		return fmt.Errorf("%w command name %s and serial number %x revoked at %v", errCertRevoked, peerCertificate.Subject.CommonName, peerCertificate.SerialNumber, ocspResponse.RevokedAt)
	case ocsp.ServerFailed:
		return errOCSPServerFailed
	case ocsp.Unknown:
		fallthrough
	default:
		return errOCSPUnknown
	}
}

func retrieveIssuerCert(issuerSubject pkix.Name, certs []*x509.Certificate) *x509.Certificate {
	for _, cert := range certs {
		if cert.Subject.SerialNumber != "" && issuerSubject.SerialNumber != "" && cert.Subject.SerialNumber == issuerSubject.SerialNumber {
			return cert
		}
		if (cert.Subject.SerialNumber == "" || issuerSubject.SerialNumber == "") && cert.Subject.String() == issuerSubject.String() {
			return cert
		}
	}
	return nil
}

func retrieveIssuingCertificate(issuingCertificateURL string) (*x509.Certificate, error) {
	resp, err := http.Get(issuingCertificateURL)
	if err != nil {
		return nil, errors.Join(errRetrieveIssuerCrt, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Join(errReadIssuerCrt, err)
	}

	block, _ := pem.Decode(body)
	if block == nil {
		return nil, errIssuerCrtPEM
	}

	issCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.Join(errParseIssuerCrt, err)
	}
	return issCert, nil
}

func isRootCA(cert *x509.Certificate) bool {
	if cert.IsCA {
		// Check AuthorityKeyId and SubjectKeyId are same.
		if len(cert.AuthorityKeyId) > 0 && len(cert.SubjectKeyId) > 0 && bytes.Equal(cert.AuthorityKeyId, cert.SubjectKeyId) {
			return true
		}
		// Alternatively, check Issuer and Subject are same.
		if cert.Issuer.String() == cert.Subject.String() {
			return true
		}
	}
	return false
}

func parseCertificates(rawCerts [][]byte) ([]*x509.Certificate, error) {
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
