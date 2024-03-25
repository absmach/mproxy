package websocket

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	errLoadCerts    = errors.New("failed to load certificates")
	errLoadServerCA = errors.New("failed to load Server CA")
	errLoadClientCA = errors.New("failed to load Client CA")
	errAppendCA     = errors.New("failed to append root ca tls.Config")
)

func Connect(brokerAddress string, tlsCfg *tls.Config) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker(brokerAddress)

	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return client, token.Error()
	}
	return client, nil
}

// Load return a TLS configuration that can be used in TLS servers
func LoadTLS(certFile, keyFile, serverCAFile, clientCAFile string) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	// Load Certs and Key if available
	if certFile != "" || keyFile != "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, errors.Join(errLoadCerts, err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{certificate},
		}
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	// Load Server CA if available
	rootCA, err := loadCertFile(serverCAFile)
	if err != nil {
		return nil, errors.Join(errLoadServerCA, err)
	}
	if len(rootCA) > 0 {
		if tlsConfig.RootCAs == nil {
			tlsConfig.RootCAs = x509.NewCertPool()
		}
		if !tlsConfig.RootCAs.AppendCertsFromPEM(rootCA) {
			return nil, errAppendCA
		}
	}

	// Load Client CA if available
	clientCA, err := loadCertFile(clientCAFile)
	if err != nil {
		return nil, errors.Join(errLoadClientCA, err)
	}
	if len(clientCA) > 0 {
		if tlsConfig.ClientCAs == nil {
			tlsConfig.ClientCAs = x509.NewCertPool()
		}
		if !tlsConfig.ClientCAs.AppendCertsFromPEM(clientCA) {
			return nil, errAppendCA
		}
	}
	return tlsConfig, nil
}

func loadCertFile(certFile string) ([]byte, error) {
	if certFile != "" {
		return os.ReadFile(certFile)
	}
	return []byte{}, nil
}
