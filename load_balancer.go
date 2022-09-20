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
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"zombiezen.com/go/log"
	"zombiezen.com/go/tailscale-lb/deque"
)

type resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
	LookupSRV(ctx context.Context, service, proto, name string) (string, []*net.SRV, error)
}

type loadBalancer struct {
	resolver   resolver
	backends   []*backend
	refreshSem chan struct{}

	mu    sync.Mutex
	queue deque.Deque[netip.AddrPort]
}

func newLoadBalancer(r resolver, backends []*backend) *loadBalancer {
	return &loadBalancer{
		resolver:   r,
		backends:   backends,
		refreshSem: make(chan struct{}, 1),
	}
}

// pick chooses one of the available backends
// or returns an error if none are available.
func (lb *loadBalancer) pick(ctx context.Context) (netip.AddrPort, error) {
	refreshErr := lb.refresh(ctx)

	lb.mu.Lock()
	defer lb.mu.Unlock()
	addr, ok := lb.queue.Front()
	if !ok {
		if refreshErr != nil {
			return netip.AddrPort{}, fmt.Errorf("pick address: %w", refreshErr)
		}
		return netip.AddrPort{}, fmt.Errorf("pick address: no backend available")
	}
	lb.queue.Rotate(1)
	return addr, nil
}

// refresh updates the addresses in the queue.
// It only returns errors if the Context is canceled or exceeds its deadline
// before the DNS resolution is complete.
func (lb *loadBalancer) refresh(ctx context.Context) error {
	// Only allow one refresh call at a time.
	select {
	case lb.refreshSem <- struct{}{}:
		// Release the semaphore on return.
		defer func() { <-lb.refreshSem }()
	case <-ctx.Done():
		return fmt.Errorf("refresh backends: start: %w", ctx.Err())
	}

	ctx, cancel := context.WithCancel(ctx)
	grp, grpCtx := errgroup.WithContext(ctx)
	grp.SetLimit(10)
	addrChan := make(chan netip.AddrPort)
	addrSetChan := make(chan map[netip.AddrPort]struct{}, 1)
	defer func() {
		cancel()
		if err := grp.Wait(); err != nil {
			log.Debugf(ctx, "Load balance refresh abort: %v", err)
		}
		<-addrSetChan
	}()

	go func() {
		defer close(addrSetChan)
		addrs := make(map[netip.AddrPort]struct{})
		for {
			select {
			case a, ok := <-addrChan:
				if !ok {
					addrSetChan <- addrs
					return
				}
				addrs[a] = struct{}{}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start the name resolution.
	const maxConcurrency = 10
	goFunc := func(ctx context.Context, f func() error) error {
		grp.Go(f)
		return nil
	}
	for _, b := range lb.backends {
		if b.addr.IsValid() {
			addrChan <- netip.AddrPortFrom(b.addr, b.port)
			continue
		}

		b := b
		err := goFunc(ctx, func() error {
			return lookup(grpCtx, addrChan, lb.resolver, goFunc, b)
		})
		if err != nil {
			return fmt.Errorf("refresh backends: %w", err)
		}
	}

	// Wait until all workers have settled, then collect the set.
	if err := grp.Wait(); err != nil {
		return fmt.Errorf("refresh backends: %w", err)
	}
	close(addrChan)
	var addrSet map[netip.AddrPort]struct{}
	select {
	case addrSet = <-addrSetChan:
	case <-ctx.Done():
		return fmt.Errorf("refresh backends: %w", ctx.Err())
	}

	// Update the queue.
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.queue.Filter(func(a netip.AddrPort) bool { _, ok := addrSet[a]; return ok })
	for i, n := 0, lb.queue.Len(); i < n; i++ {
		delete(addrSet, lb.queue.At(i))
	}
	for newAddr := range addrSet {
		lb.queue.Append(newAddr)
	}
	return nil
}

func lookup(ctx context.Context, out chan<- netip.AddrPort, resolver resolver, goFunc func(context.Context, func() error) error, b *backend) error {
	if b.srv {
		_, records, err := resolver.LookupSRV(ctx, "", "", b.hostname)
		if err != nil {
			log.Warnf(ctx, "%v", err)
			return nil
		}
		if log.IsEnabled(log.Debug) {
			recordsString := new(strings.Builder)
			for i, r := range records {
				if i > 0 {
					recordsString.WriteString(" ")
				}
				fmt.Fprintf(recordsString, "%s:%d", r.Target, r.Port)
			}
			log.Debugf(ctx, "Resolved SRV %s -> %s", b.hostname, recordsString)
		}
		if len(records) == 0 {
			log.Warnf(ctx, "No SRV records found for %s", b.hostname)
			return nil
		}
		for _, r := range records[:len(records)-1] {
			r := r
			err := goFunc(ctx, func() error {
				return lookup(ctx, out, resolver, goFunc, &backend{
					hostname: r.Target,
					port:     r.Port,
				})
			})
			if err != nil {
				return err
			}
		}

		// Don't acquire the semaphore for the last record:
		// just reuse the one already grabbed for this goroutine.
		lastRecord := records[len(records)-1]
		b = &backend{
			hostname: lastRecord.Target,
			port:     lastRecord.Port,
		}
	}

	addrs, err := resolver.LookupNetIP(ctx, "ip", b.hostname)
	if err != nil {
		log.Warnf(ctx, "%v", err)
		return nil
	}
	// Workaround for upstream Go weirdness: https://go.dev/issue/53554
	// LookupNetIP can return IPv4-mapped IPv6 addresses,
	// which can't directly be dialed.
	for i, a := range addrs {
		if a.Is4In6() {
			addrs[i] = netip.AddrFrom4(a.As4())
		}
	}
	if log.IsEnabled(log.Debug) {
		addrsString := new(strings.Builder)
		for i, a := range addrs {
			if i > 0 {
				addrsString.WriteString(" ")
			}
			addrsString.WriteString(a.String())
		}
		log.Debugf(ctx, "Resolved A/AAAA %s -> %s", b.hostname, addrsString)
	}
	for _, a := range addrs {
		select {
		case out <- netip.AddrPortFrom(a, b.port):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
