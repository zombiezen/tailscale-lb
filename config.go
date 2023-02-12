// Copyright 2022 Ross Light
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		 https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"zombiezen.com/go/ini"
	"zombiezen.com/go/log"
)

type configuration struct {
	hostname string
	authKey  string
	stateDir string
	ports    map[uint16]portConfig
}

type portConfig struct {
	tcp  *tcpConfig
	http *httpConfig
}

func (pc portConfig) isEmpty() bool {
	return pc.tcp == nil && pc.http == nil
}

type tcpConfig struct {
	backends []*backend
}

type httpConfig struct {
	backends []*backend
	whois    bool
	trustXFF bool
	tls      bool
}

func (cfg *configuration) fill(source configer) error {
	if cfg.hostname == "" {
		cfg.hostname = source.Get("", "hostname")
	}
	if cfg.authKey == "" {
		cfg.authKey = source.Get("", "auth-key")
	}
	if cfg.stateDir == "" {
		if v := source.Value("", "state-directory"); v != nil {
			if v.Filename == "" {
				return fmt.Errorf("configuration value for state-directory (line %d) has no file", v.Line)
			}
			if filepath.IsAbs(v.Value) {
				cfg.stateDir = v.Value
			} else {
				cfg.stateDir = filepath.Join(filepath.Dir(v.Filename), v.Value)
			}
		}
	}

	for sectionName := range source.Sections() {
		switch {
		case strings.HasPrefix(sectionName, "tcp "):
			n, err := strconv.ParseUint(sectionName[len("tcp "):], 10, 16)
			if err != nil {
				log.Warnf(context.TODO(), "Unknown config section %q", sectionName)
				continue
			}
			portNumber := uint16(n)
			if portNumber == 0 {
				return fmt.Errorf("read config: cannot configure port 0")
			}
			if cfg.ports == nil {
				cfg.ports = make(map[uint16]portConfig)
			} else if !cfg.ports[portNumber].isEmpty() {
				return fmt.Errorf("read config: conflicting definition of port %d", portNumber)
			}
			tc := new(tcpConfig)
			cfg.ports[portNumber] = portConfig{tcp: tc}

			for _, backendAddr := range source.Find(sectionName, "backend") {
				b, err := parseBackend(backendAddr, portNumber)
				if err != nil {
					return fmt.Errorf("read config: tcp %d: %v", portNumber, err)
				}
				tc.backends = append(tc.backends, b)
			}
		case strings.HasPrefix(sectionName, "http "):
			n, err := strconv.ParseUint(sectionName[len("http "):], 10, 16)
			if err != nil {
				log.Warnf(context.TODO(), "Unknown config section %q", sectionName)
				continue
			}
			portNumber := uint16(n)
			if portNumber == 0 {
				return fmt.Errorf("read config: cannot configure port 0")
			}
			if cfg.ports == nil {
				cfg.ports = make(map[uint16]portConfig)
			} else if !cfg.ports[portNumber].isEmpty() {
				return fmt.Errorf("read config: conflicting definition of port %d", portNumber)
			}
			hc := new(httpConfig)
			cfg.ports[portNumber] = portConfig{http: hc}

			if s := source.Get(sectionName, "tls"); s != "" {
				var err error
				hc.tls, err = strconv.ParseBool(s)
				if err != nil {
					return fmt.Errorf("read config: http %d: tls: %v", portNumber, err)
				}
			}
			if s := source.Get(sectionName, "whois"); s != "" {
				var err error
				hc.whois, err = strconv.ParseBool(s)
				if err != nil {
					return fmt.Errorf("read config: http %d: whois: %v", portNumber, err)
				}
			}
			if s := source.Get(sectionName, "trust-x-forwarded-for"); s != "" {
				var err error
				hc.trustXFF, err = strconv.ParseBool(s)
				if err != nil {
					return fmt.Errorf("read config: http %d: trust-x-forwarded-for: %v", portNumber, err)
				}
			}
			for _, backendAddr := range source.Find(sectionName, "backend") {
				b, err := parseBackend(backendAddr, portNumber)
				if err != nil {
					return fmt.Errorf("read config: http %d: %v", portNumber, err)
				}
				hc.backends = append(hc.backends, b)
			}
		default:
			if sectionName != "" {
				log.Warnf(context.TODO(), "Unknown config section %q", sectionName)
			}
			continue
		}
	}
	return nil
}

type backend struct {
	addr     netip.Addr
	hostname string
	port     uint16
	srv      bool
}

func parseBackend(s string, implicitPort uint16) (*backend, error) {
	const srvPrefix = "srv"
	if len(s) >= len(srvPrefix)+1 && s[:len(srvPrefix)] == srvPrefix {
		if c, size := utf8.DecodeRuneInString(s[len(srvPrefix):]); unicode.IsSpace(c) {
			return &backend{
				hostname: strings.TrimLeftFunc(s[len(srvPrefix)+size:], unicode.IsSpace),
				srv:      true,
			}, nil
		}
	}

	b := new(backend)
	host, portString, err := net.SplitHostPort(s)
	if err != nil {
		host = s
		b.port = implicitPort
	} else {
		port, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("parse backend %q: invalid port", s)
		}
		b.port = uint16(port)
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		b.addr = addr
	} else {
		b.hostname = host
	}
	return b, nil
}

func (b *backend) String() string {
	if b.srv {
		return "srv " + b.hostname
	}
	host := b.hostname
	if b.addr.IsValid() {
		host = b.addr.String()
	}
	return net.JoinHostPort(host, strconv.Itoa(int(b.port)))
}

type configer interface {
	Get(section, key string) string
	Value(section, key string) *ini.Value
	Find(section, key string) []string
	Sections() map[string]struct{}
}
