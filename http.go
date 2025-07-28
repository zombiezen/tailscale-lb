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
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"unicode/utf8"

	"tailscale.com/client/tailscale"
	"tailscale.com/client/tailscale/apitype"
	"zombiezen.com/go/log"
	"zombiezen.com/go/log/zstdlog"
)

type httpLoadBalancer struct {
	lb           *loadBalancer
	tailscale    *tailscale.LocalClient
	whoisHeaders bool
	trustXFF     bool
}

func (hlb *httpLoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	whoisChan := make(chan *apitype.WhoIsResponse, 1)
	if hlb.whoisHeaders {
		go func() {
			defer close(whoisChan)
			whois, err := hlb.tailscale.WhoIs(ctx, r.RemoteAddr)
			if err != nil {
				log.Errorf(ctx, "Tailscale whois: %v", err)
				return
			}
			whoisChan <- whois
		}()
	} else {
		close(whoisChan)
	}

	addr, err := hlb.lb.pick(ctx)
	if err != nil {
		log.Errorf(ctx, "Finding backend for %s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, "Could not find suitable backend for request.", http.StatusServiceUnavailable)
		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			// Strip any Tailscale headers out,
			// so proxied servers can know to trust the headers.
			for key := range r.Out.Header {
				if strings.HasPrefix(key, "Tailscale-") {
					delete(r.Out.Header, key)
				}
			}

			r.SetURL(&url.URL{
				Scheme: "http",
				Host:   addr.String(),
			})
			r.Out.Host = r.In.Host
			if hlb.trustXFF {
				r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
			}
			r.SetXForwarded()

			if hlb.whoisHeaders {
				whois := <-whoisChan
				if whois != nil {
					// Reference: https://tailscale.com/kb/1312/serve#identity-headers
					setHeader(r.Out.Header, "Tailscale-User-Login", whois.UserProfile.LoginName)
					setHeader(r.Out.Header, "Tailscale-User-Name", whois.UserProfile.DisplayName)
					setHeader(r.Out.Header, "Tailscale-User-Profile-Pic", whois.UserProfile.ProfilePicURL)
				}
			}
		},
		ErrorLog: zstdlog.New(log.Default(), &zstdlog.Options{
			Context: ctx,
			Level:   log.Warn,
		}),
	}
	proxy.ServeHTTP(w, r)
}

func setHeader(h http.Header, k, v string) {
	if v == "" || !utf8.ValidString(v) {
		return
	}
	h.Set(k, mime.QEncoding.Encode("utf-8", v))
}
