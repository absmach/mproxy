// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ocsp"
)

var (
	certFile       = "ssl/certs/ca.crt"
	issuerCertFile = "ssl/certs/ca.crt"
	keyFile        = "ssl/certs/ca.key"
	crlFile        = "ssl/certs/revoked_certs.crl"
)

var (
	goodCertsPath    = []string{"ssl/certs/client.crt", "ssl/certs/ca.crt"}
	revokedCertsPath = []string{"ssl/certs/client_revoked.crt"}
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	goodCerts, err := serialNumbersFromCertsPath(goodCertsPath)
	if err != nil {
		panic(err)
	}

	revokedCerts, err := serialNumbersFromCertsPath(revokedCertsPath)
	if err != nil {
		panic(err)
	}

	// Load the certificate, issuer certificate, and private key
	cert, err := loadCertificate(certFile)
	if err != nil {
		fmt.Println("Error loading certificate:", err)
		return
	}

	issuerCert, err := loadCertificate(issuerCertFile)
	if err != nil {
		fmt.Println("Error loading issuer certificate:", err)
		return
	}

	privateKey, err := loadPrivateKey(keyFile)
	if err != nil {
		fmt.Println("Error loading private key:", err)
		return
	}

	// Register OCSP handler
	http.HandleFunc("/ocsp", func(w http.ResponseWriter, r *http.Request) {
		ocspHandler(w, r, cert, issuerCert, privateKey, goodCerts, revokedCerts, *logger)
	})

	http.HandleFunc("/crl.pem", func(w http.ResponseWriter, r *http.Request) {
		fileHandler(w, r, crlFile, "crl.pem", *logger)
	})

	http.HandleFunc("/ca.pem", func(w http.ResponseWriter, r *http.Request) {
		fileHandler(w, r, issuerCertFile, "ca.pem", *logger)
	})

	// Start the HTTP server
	fmt.Println("OCSP/CRL responder listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

func loadCertificate(file string) (*x509.Certificate, error) {
	pemData, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file %s, error : %w", file, err)
	}

	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block of cert file %s", file)
	}

	return x509.ParseCertificate(block.Bytes)
}

func loadPrivateKey(file string) (interface{}, error) {
	keyData, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	return x509.ParsePKCS8PrivateKey(block.Bytes)
}

func fileHandler(w http.ResponseWriter, r *http.Request, crlFile, fileName string, logger slog.Logger) {
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	files, err := os.ReadFile(crlFile)
	args := []any{
		slog.String("request", r.URL.String()),
	}
	if err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		args = append(args, slog.String("error", err.Error()))
		logger.Info("Request failed ", args...)
		return
	}
	if _, err := w.Write(files); err != nil {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		args = append(args, slog.String("error", err.Error()))
		logger.Info("Request failed ", args...)
		return
	}

	logger.Info("Request complete successfully ", args...)
}

func ocspHandler(w http.ResponseWriter, r *http.Request, cert, issuerCert *x509.Certificate, privateKey interface{}, goodCerts, revokedCerts []*big.Int, logger slog.Logger) {
	ocspStatus := ocsp.Unknown

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read OCSP request", http.StatusBadRequest)
	}
	req, err := ocsp.ParseRequest(body)
	if err != nil {
		http.Error(w, "Failed to parse OCSP request", http.StatusBadRequest)
		return
	}
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}

	for _, sn := range goodCerts {
		if req.SerialNumber.Cmp(sn) == 0 {
			ocspStatus = ocsp.Good
		}
	}

	for _, sn := range revokedCerts {
		if req.SerialNumber.Cmp(sn) == 0 {
			ocspStatus = ocsp.Revoked
		}
	}

	statusParam := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("force_status")))
	switch statusParam {
	case "REVOKE":
		ocspStatus = ocsp.Revoked
	case "GOOD":
		ocspStatus = ocsp.Good
	case "SERVERFAILED":
		ocspStatus = ocsp.ServerFailed
	case "RANDOM":
		r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
		ocspStatus = r.Intn(ocsp.ServerFailed)
	}

	template := ocsp.Response{
		Status:       ocspStatus,
		SerialNumber: req.SerialNumber,
		ThisUpdate:   time.Now(),
		NextUpdate:   time.Now(),
		Certificate:  cert,
		IssuerHash:   req.HashAlgorithm,
	}

	if ocspStatus == ocsp.Revoked {
		template.RevokedAt = time.Now()
	}

	response, err := ocsp.CreateResponse(issuerCert, cert, template, signer)
	if err != nil {
		http.Error(w, "Failed to create OCSP response", http.StatusInternalServerError)
		return
	}

	args := []any{
		slog.String("request", r.URL.String()),
		slog.String("ocsp_status", getOCSPStatus(ocspStatus)),
		slog.String("request_certificate_serial_number", fmt.Sprintf("%x", req.SerialNumber)),
	}

	w.Header().Set("Content-Type", "application/ocsp-response")
	if _, err := w.Write(response); err != nil {
		args = append(args, slog.String("error", err.Error()))
		logger.Info("Request complete with errors ", args...)
		return
	}
	logger.Info("Request complete successfully ", args...)
}

func serialNumbersFromCertsPath(certsPath []string) ([]*big.Int, error) {
	sns := make([]*big.Int, 0)
	for _, certPath := range certsPath {
		cert, err := loadCertificate(certPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate in path %s", certPath)
		}
		sns = append(sns, cert.SerialNumber)
	}
	return sns, nil
}

func getOCSPStatus(status int) string {
	switch status {
	case ocsp.Revoked:
		return "REVOKE"
	case ocsp.Good:
		return "GOOD"
	case ocsp.ServerFailed:
		return "SERVERFAILED"
	case ocsp.Unknown:
		fallthrough
	default:
		return "UNKNOWN"
	}
}
