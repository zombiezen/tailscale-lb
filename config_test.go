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
	"net/netip"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseBackend(t *testing.T) {
	tests := []struct {
		s            string
		implicitPort uint16
		want         *backend
	}{
		{"127.0.0.1", 80, &backend{
			addr: netip.MustParseAddr("127.0.0.1"),
			port: 80,
		}},
		{"127.0.0.1:8080", 80, &backend{
			addr: netip.MustParseAddr("127.0.0.1"),
			port: 8080,
		}},
		{"example.com", 80, &backend{
			hostname: "example.com",
			port:     80,
		}},
		{"example.com:8080", 80, &backend{
			hostname: "example.com",
			port:     8080,
		}},
		{"srv example.com", 80, &backend{
			hostname: "example.com",
			srv:      true,
		}},
		{"srv  example.com", 80, &backend{
			hostname: "example.com",
			srv:      true,
		}},
		{"srv.example.com", 80, &backend{
			hostname: "srv.example.com",
			port:     80,
		}},
	}
	for _, test := range tests {
		got, err := parseBackend(test.s, test.implicitPort)
		if err != nil {
			t.Errorf("parseBackend(%q, %d) = _, %v; want %#v", test.s, test.implicitPort, got, test.want)
			continue
		}
		diff := cmp.Diff(
			test.want, got,
			cmp.AllowUnexported(backend{}),
			cmp.Comparer(func(a1, a2 netip.Addr) bool { return a1 == a2 }),
		)
		if diff != "" {
			t.Errorf("parseBackend(%q, %d) (-want +got):\n%s", test.s, test.implicitPort, diff)
		}
	}
}
