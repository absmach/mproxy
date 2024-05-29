// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"

	"github.com/pion/dtls/v2"
)

var (
	errTLSdetails     = errors.New("failed to get TLS details of connection")
	errLoadCerts      = errors.New("failed to load certificates")
	errLoadServerCA   = errors.New("failed to load Server CA")
	errLoadClientCA   = errors.New("failed to load Client CA")
	errAppendCA       = errors.New("failed to append root ca tls.Config")
	errUnsupportedTLS = errors.New("unsupported tls configuration")
)

type TLSConfig interface {
	*tls.Config | *dtls.Config
}

// LoadTLSConfig returns a TLS or DTLS configuration that can be used for TLS or DTLS servers.
func LoadTLSConfig[sc TLSConfig](c *Config, s sc) (sc, error) {
	if c.CertFile == "" || c.KeyFile == "" {
		return nil, nil
	}

	certificate, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, errors.Join(errLoadCerts, err)
	}

	// Loading Server CA file
	rootCA, err := loadCertFile(c.ServerCAFile)
	if err != nil {
		return nil, errors.Join(errLoadServerCA, err)
	}

	// Loading Client CA File
	clientCA, err := loadCertFile(c.ClientCAFile)
	if err != nil {
		return nil, errors.Join(errLoadClientCA, err)
	}

	switch config := any(s).(type) {
	case *tls.Config:
		config.Certificates = []tls.Certificate{certificate}

		if len(rootCA) > 0 {
			if config.RootCAs == nil {
				config.RootCAs = x509.NewCertPool()
			}
			if !config.RootCAs.AppendCertsFromPEM(rootCA) {
				return nil, errAppendCA
			}
		}

		if len(clientCA) > 0 {
			if config.ClientCAs == nil {
				config.ClientCAs = x509.NewCertPool()
			}
			if !config.ClientCAs.AppendCertsFromPEM(clientCA) {
				return nil, errAppendCA
			}
			config.ClientAuth = tls.RequireAndVerifyClientCert
			if c.Validator != nil {
				config.VerifyPeerCertificate = c.Validator
			}
		}
		return s, nil
	case *dtls.Config:
		config.Certificates = []tls.Certificate{certificate}

		if len(rootCA) > 0 {
			if config.RootCAs == nil {
				config.RootCAs = x509.NewCertPool()
			}
			if !config.RootCAs.AppendCertsFromPEM(rootCA) {
				return nil, errAppendCA
			}
		}

		if len(clientCA) > 0 {
			if config.ClientCAs == nil {
				config.ClientCAs = x509.NewCertPool()
			}
			if !config.ClientCAs.AppendCertsFromPEM(clientCA) {
				return nil, errAppendCA
			}
			config.ClientAuth = dtls.RequireAndVerifyClientCert
			if c.Validator != nil {
				config.VerifyPeerCertificate = c.Validator
			}
		}
		return s, nil
	default:
		return nil, errUnsupportedTLS
	}
}

// ClientCert returns client certificate.
func ClientCert(conn net.Conn) (x509.Certificate, error) {
	switch connVal := conn.(type) {
	case *tls.Conn:
		if err := connVal.Handshake(); err != nil {
			return x509.Certificate{}, err
		}
		state := connVal.ConnectionState()
		if state.Version == 0 {
			return x509.Certificate{}, errTLSdetails
		}
		if len(state.PeerCertificates) == 0 {
			return x509.Certificate{}, nil
		}
		cert := *state.PeerCertificates[0]
		return cert, nil
	default:
		return x509.Certificate{}, nil
	}
}

// SecurityStatus returns log message from TLS config.
func SecurityStatus(c *tls.Config) string {
	if c == nil {
		return "no TLS"
	}
	ret := "TLS"
	// It is possible to establish TLS with client certificates only.
	if c.Certificates == nil || len(c.Certificates) == 0 {
		ret = "no server certificates"
	}
	if c.ClientCAs != nil {
		ret += " and " + c.ClientAuth.String()
	}
	return ret
}

func loadCertFile(certFile string) ([]byte, error) {
	if certFile != "" {
		return os.ReadFile(certFile)
	}
	return []byte{}, nil
}
