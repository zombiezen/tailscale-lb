// Copyright 2023 Ross Light
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
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"testing"

	"tailscale.com/client/tailscale"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/tailcfg"
)

func TestHTTPLoadBalancer(t *testing.T) {
	const (
		wantPath              = "/foo"
		wantHost              = "ts-service.example.com"
		wantUser              = "foo@example.com"
		wantDisplayName       = "Foo Bar"
		wantProfilePictureURL = "https://www.example.com/user/foo/profile.png"

		wantContentType = "text/plain; charset=utf-8"
		wantResponse    = "Hello, World!\n"

		userProvidedHeader = "Tailscale-Evil-Header"
	)

	backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Host, wantHost; got != want {
			t.Errorf("Host = %q; want %q", got, want)
		}
		if got, want := r.URL.Path, wantPath; got != want {
			t.Errorf(":path = %q; want %q", got, want)
		}
		if got, want := r.Header.Get("Tailscale-User-Login"), wantUser; got != want {
			t.Errorf("Tailscale-User-Login = %q; want %q", got, want)
		}
		if got, want := r.Header.Get("Tailscale-User-Name"), wantDisplayName; got != want {
			t.Errorf("Tailscale-User-Name = %q; want %q", got, want)
		}
		if got, want := r.Header.Get("Tailscale-User-Profile-Pic"), wantProfilePictureURL; got != want {
			t.Errorf("Tailscale-User-Profile-Pic = %q; want %q", got, want)
		}
		if got := r.Header.Values(userProvidedHeader); len(got) > 0 {
			t.Errorf("%s = %q; want []", userProvidedHeader, got)
		}
		if got, want1, want2 := r.Header.Get("X-Forwarded-For"), "127.0.0.1", "::1"; got != want1 && got != want2 {
			t.Errorf("X-Forwarded-For = %q; want %q or %q", got, want1, want2)
		}
		w.Header().Set("Content-Type", wantContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(wantResponse)))
		io.WriteString(w, wantResponse)
	}))
	defer backendSrv.Close()
	backendURL, err := url.Parse(backendSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	backendAddr, err := netip.ParseAddrPort(backendURL.Host)
	if err != nil {
		t.Fatal(err)
	}

	tailscaleLocalAPISrv := httptest.NewServer(fakeWhoIsHandler(
		func(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error) {
			return &apitype.WhoIsResponse{
				UserProfile: &tailcfg.UserProfile{
					LoginName:     wantUser,
					DisplayName:   wantDisplayName,
					ProfilePicURL: wantProfilePictureURL,
				},
			}, nil
		},
	))
	defer tailscaleLocalAPISrv.Close()
	tailscaleLocalAPIURL, err := url.Parse(tailscaleLocalAPISrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	tailscaleLocalAPIAddr, err := netip.ParseAddrPort(tailscaleLocalAPIURL.Host)
	if err != nil {
		t.Fatal(err)
	}

	proxySrv := httptest.NewServer(&httpLoadBalancer{
		lb: newLoadBalancer(nil, []*backend{{
			addr: backendAddr.Addr(),
			port: backendAddr.Port(),
		}}),
		tailscale: &tailscale.LocalClient{
			Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("tcp", tailscaleLocalAPIAddr.String())
			},
		},
		whoisHeaders: true,
	})
	defer proxySrv.Close()

	req, err := http.NewRequest(http.MethodGet, proxySrv.URL+wantPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = wantHost
	req.Header.Set(userProvidedHeader, "xyzzy")
	resp, err := proxySrv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if string(got) != wantResponse || err != nil {
		t.Errorf("io.ReadAll(Body) = %q, %v; want %q, <nil>", got, err, wantResponse)
	}
}

// fakeWhoIsHandler returns a fake of the Tailscale Local API
// that implements the "WhoIs" endpoint.
func fakeWhoIsHandler(f func(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/localapi/v0/whois" {
			http.NotFound(w, r)
			return
		}
		remoteAddr := r.URL.Query().Get("addr")
		resp, err := f(r.Context(), remoteAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respJSON, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(respJSON)))
		w.Write(respJSON)
	})
}
