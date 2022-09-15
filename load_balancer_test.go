package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"zombiezen.com/go/log/testlog"
)

func TestSingleAddress(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)
	lb := newLoadBalancer(fakeResolver{}, []*backend{
		{addr: netip.MustParseAddr("127.0.0.1"), port: 80},
	})
	got, err := lb.pick(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := netip.MustParseAddrPort("127.0.0.1:80"); got != want {
		t.Errorf("lb.pick(ctx) = %v; want %v", got, want)
	}
}

func TestMultipleAddresses(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)
	lb := newLoadBalancer(fakeResolver{}, []*backend{
		{addr: netip.MustParseAddr("127.0.0.1"), port: 80},
		{addr: netip.MustParseAddr("127.0.0.1"), port: 81},
		{addr: netip.MustParseAddr("127.0.0.1"), port: 82},
	})

	got := make(map[netip.AddrPort]struct{})
	for i := 0; i < 3; i++ {
		addrPort, err := lb.pick(ctx)
		if err != nil {
			t.Error(err)
			break
		}
		got[addrPort] = struct{}{}
	}
	want := map[netip.AddrPort]struct{}{
		netip.MustParseAddrPort("127.0.0.1:80"): {},
		netip.MustParseAddrPort("127.0.0.1:81"): {},
		netip.MustParseAddrPort("127.0.0.1:82"): {},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("picked (-want +got):\n%s", diff)
	}
}

func TestHostName(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)
	rslv := fakeResolver{a: map[string][]netip.Addr{
		"example.com": {
			netip.MustParseAddr("192.0.2.1"),
			netip.MustParseAddr("192.0.2.2"),
		},
	}}
	lb := newLoadBalancer(rslv, []*backend{
		{hostname: "example.com", port: 80},
	})

	got := make(map[netip.AddrPort]struct{})
	for i := 0; i < 2; i++ {
		addrPort, err := lb.pick(ctx)
		if err != nil {
			t.Error(err)
			break
		}
		got[addrPort] = struct{}{}
	}
	want := map[netip.AddrPort]struct{}{
		netip.MustParseAddrPort("192.0.2.1:80"): {},
		netip.MustParseAddrPort("192.0.2.2:80"): {},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("picked (-want +got):\n%s", diff)
	}
}

func TestSRV(t *testing.T) {
	ctx := testlog.WithTB(context.Background(), t)
	rslv := fakeResolver{
		a: map[string][]netip.Addr{
			"example.com.": {
				netip.MustParseAddr("192.0.2.1"),
				netip.MustParseAddr("192.0.2.2"),
			},
		},
		srv: map[string][]*net.SRV{
			"_http._tcp.example.com": {
				{
					Target:   "example.com.",
					Port:     80,
					Priority: 10,
					Weight:   0,
				},
				{
					Target:   "example.com.",
					Port:     8080,
					Priority: 20,
					Weight:   0,
				},
			},
		},
	}
	lb := newLoadBalancer(rslv, []*backend{
		{hostname: "_http._tcp.example.com", srv: true},
	})

	got := make(map[netip.AddrPort]struct{})
	for i := 0; i < 4; i++ {
		addrPort, err := lb.pick(ctx)
		if err != nil {
			t.Error(err)
			break
		}
		got[addrPort] = struct{}{}
	}
	want := map[netip.AddrPort]struct{}{
		netip.MustParseAddrPort("192.0.2.1:80"):   {},
		netip.MustParseAddrPort("192.0.2.2:80"):   {},
		netip.MustParseAddrPort("192.0.2.1:8080"): {},
		netip.MustParseAddrPort("192.0.2.2:8080"): {},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("picked (-want +got):\n%s", diff)
	}
}

type fakeResolver struct {
	a   map[string][]netip.Addr
	srv map[string][]*net.SRV
}

func (r fakeResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	if network != "ip" {
		return nil, fmt.Errorf("lookup ip: only \"ip\" network supported (got %q)", network)
	}
	return append([]netip.Addr(nil), r.a[host]...), nil
}

func (r fakeResolver) LookupSRV(ctx context.Context, service, proto, name string) (cname string, srv []*net.SRV, err error) {
	if service != "" || proto != "" {
		cname = fmt.Sprintf("_%s._%s.%s", service, proto, name)
	} else {
		cname = name
	}
	records := r.srv[cname]
	if len(records) == 0 {
		return cname, nil, nil
	}
	srv = make([]*net.SRV, 0, len(records))
	for _, r := range records {
		r2 := new(net.SRV)
		*r2 = *r
		srv = append(srv, r2)
	}
	return cname, srv, nil
}

func TestMain(m *testing.M) {
	testlog.Main(nil)
	os.Exit(m.Run())
}
