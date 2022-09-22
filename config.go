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
	tcpPorts map[uint16]*portConfig
}

type portConfig struct {
	backends []*backend
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
			cfg.stateDir = filepath.Join(filepath.Dir(v.Filename), v.Value)
		}
	}
	for name := range source.Sections() {
		const prefix = "tcp "
		if !strings.HasPrefix(name, prefix) {
			if name != "" {
				log.Warnf(context.TODO(), "Unknown config section %q", name)
			}
			continue
		}
		n, err := strconv.ParseUint(name[len(prefix):], 10, 16)
		if err != nil {
			log.Warnf(context.TODO(), "Unknown config section %q", name)
			continue
		}
		portNumber := uint16(n)

		if cfg.tcpPorts == nil {
			cfg.tcpPorts = make(map[uint16]*portConfig)
		} else if cfg.tcpPorts[portNumber] != nil {
			return fmt.Errorf("read config: conflicting definition of tcp %d", portNumber)
		}
		pc := new(portConfig)
		cfg.tcpPorts[portNumber] = pc

		for _, backendAddr := range source.Find(name, "backend") {
			b, err := parseBackend(backendAddr, portNumber)
			if err != nil {
				return fmt.Errorf("read config: tcp %d: %v", portNumber, err)
			}
			pc.backends = append(pc.backends, b)
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
