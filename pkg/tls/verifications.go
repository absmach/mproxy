// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package tls

import (
	"errors"
	"reflect"
	"strings"

	"github.com/absmach/mproxy/pkg/tls/verifier"
	"github.com/absmach/mproxy/pkg/tls/verifier/crl"
	"github.com/absmach/mproxy/pkg/tls/verifier/ocsp"
	"github.com/caarlos0/env/v11"
)

// ErrInvalidCertVerification represents an error during the cert verification
// method loading. Supported are OCSP and CRL verification methods.
var ErrInvalidCertVerification = errors.New("invalid certificate verification method")

type verification int

const (
	OCSP verification = iota + 1
	CRL
)

func newVerifiers(opts env.Options) ([]verifier.Verifier, error) {
	if opts.FuncMap == nil {
		opts.FuncMap = make(map[reflect.Type]env.ParserFunc)
	}
	opts.FuncMap[reflect.TypeOf(make([]verification, 0))] = envParseSliceValidate
	opts.FuncMap[reflect.TypeOf(new(verification))] = envParseValidation

	var c struct {
		Verifications []verification `env:"CERT_VERIFICATION_METHODS"             envDefault:""`
	}
	if err := env.ParseWithOptions(&c, opts); err != nil {
		return nil, err
	}
	if len(c.Verifications) == 0 {
		return nil, nil
	}

	var vms []verifier.Verifier
	for _, v := range c.Verifications {
		switch v {
		case OCSP:
			vm, err := ocsp.New(opts)
			if err != nil {
				return nil, err
			}
			vms = append(vms, vm)
		case CRL:
			vm, err := crl.New(opts)
			if err != nil {
				return nil, err
			}
			vms = append(vms, vm)
		default:
			return nil, ErrInvalidCertVerification
		}
	}

	return vms, nil
}

func parseValidation(v string) (verification, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	switch v {
	case "OCSP":
		return OCSP, nil
	case "CRL":
		return CRL, nil
	default:
		return 0, ErrInvalidCertVerification
	}
}

func envParseSliceValidate(v string) (interface{}, error) {
	var vms []verification
	v = strings.TrimSpace(v)
	vmss := strings.Split(v, ",")
	for _, vm := range vmss {
		v, err := parseValidation(vm)
		if err != nil {
			return nil, err
		}
		vms = append(vms, v)
	}
	return vms, nil
}

func envParseValidation(v string) (interface{}, error) {
	return parseValidation(v)
}
