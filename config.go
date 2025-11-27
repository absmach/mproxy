// Copyright (c) Abstract Machines
// SPDX-License-Identifier: Apache-2.0

package mproxy

import (
	"crypto/tls"

	mptls "github.com/absmach/mproxy/pkg/tls"
	"github.com/caarlos0/env/v11"
	"github.com/pion/dtls/v3"
)

type Config struct {
	Host           string `env:"HOST"           envDefault:""`
	Port           string `env:"PORT"           envDefault:""`
	PathPrefix     string `env:"PATH_PREFIX"    envDefault:""`
	TargetHost     string `env:"TARGET_HOST"    envDefault:""`
	TargetPort     string `env:"TARGET_PORT"    envDefault:""`
	TargetProtocol string `env:"TARGET_PROTOCOL" envDefault:""`
	TargetPath     string `env:"TARGET_PATH"    envDefault:""`
	TLSConfig      *tls.Config
	DTLSConfig     *dtls.Config
}

func NewConfig(opts env.Options) (Config, error) {
	c := Config{}
	if err := env.ParseWithOptions(&c, opts); err != nil {
		return Config{}, err
	}

	cfg, err := mptls.NewConfig(opts)
	if err != nil {
		return Config{}, err
	}
	c.TLSConfig, err = mptls.LoadTLSConfig(&cfg, &tls.Config{})
	if err != nil {
		return Config{}, err
	}
	c.DTLSConfig, err = mptls.LoadTLSConfig(&cfg, &dtls.Config{})
	if err != nil {
		return Config{}, err
	}
	return c, nil
}
