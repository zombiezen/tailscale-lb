package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"tailscale.com/client/tailscale"
	"tailscale.com/ipn"
	"tailscale.com/ipn/store/mem"
	"tailscale.com/tsnet"
	"zombiezen.com/go/ini"
	"zombiezen.com/go/log"
)

const programName = "tailscale-lb"

const tailscaleLogLevel = log.Debug - 1

func main() {
	flagSet := flag.NewFlagSet(programName, flag.ContinueOnError)
	flagSet.Usage = func() {
		fmt.Fprintf(flagSet.Output(), "usage: %s CONFIG [...]\n", programName)
		flagSet.PrintDefaults()
	}
	var cfg configuration
	flagSet.StringVar(&cfg.hostname, "hostname", "", "host`name` to send to Tailscale")
	debug := flagSet.Bool("debug", false, "show debugging output")
	debugTailscale := flagSet.Bool("debug-tailscale", false, "show all debugging output, including Tailscale")

	const exitUsage = 64
	if err := flagSet.Parse(os.Args[1:]); err == flag.ErrHelp {
		os.Exit(exitUsage)
	} else if err != nil {
		os.Exit(1)
	}

	const baseLogFlags = log.ShowDate | log.ShowTime
	if *debugTailscale {
		log.SetDefault(log.New(os.Stderr, "", baseLogFlags|log.ShowLevel, nil))
	} else if *debug {
		log.SetDefault(&log.LevelFilter{
			Min:    log.Debug,
			Output: log.New(os.Stderr, "", baseLogFlags|log.ShowLevel, nil),
		})
	} else {
		log.SetDefault(&log.LevelFilter{
			Min:    log.Info,
			Output: log.New(os.Stderr, "", baseLogFlags, nil),
		})
	}

	ctx, cancel := signal.NotifyContext(context.Background(), interruptSignals...)
	if flagSet.NArg() == 0 {
		log.Errorf(ctx, "No configuration files given")
		flagSet.PrintDefaults()
		os.Exit(exitUsage)
	}
	iniFiles, err := ini.ParseFiles(nil, flagSet.Args()...)
	if err != nil {
		log.Errorf(ctx, "%v", err)
		os.Exit(1)
	}
	if err := cfg.fill(iniFiles); err != nil {
		log.Errorf(ctx, "%v", err)
		os.Exit(1)
	}

	err = run(ctx, &cfg)
	cancel()
	if err != nil {
		log.Errorf(ctx, "%v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *configuration) error {
	if cfg.hostname == "" {
		return fmt.Errorf("hostname not set in configuration")
	}

	srv := tsnet.Server{
		// TODO(soon): Permit persisting state.
		Store:     new(mem.Store),
		Ephemeral: true,

		Hostname: cfg.hostname,
		AuthKey:  cfg.authKey,
		Logf: func(format string, args ...any) {
			ent := log.Entry{Time: time.Now(), Level: tailscaleLogLevel}
			if _, file, line, ok := runtime.Caller(2); ok {
				ent.File = file
				ent.Line = line
			}
			logger := log.Default()
			if !logger.LogEnabled(ent) {
				return
			}
			ent.Msg = fmt.Sprintf(format, args...)
			if n := len(ent.Msg); n > 0 && ent.Msg[n-1] == '\n' {
				ent.Msg = ent.Msg[:n-1]
			}
			logger.Log(ctx, ent)
		},
	}
	if err := srv.Start(); err != nil {
		return err
	}
	log.Infof(ctx, "Host %s connected to Tailscale", cfg.hostname)
	var wg sync.WaitGroup
	defer func() {
		log.Debugf(ctx, "Shutting down...")
		if err := srv.Close(); err != nil {
			log.Errorf(ctx, "While shutting down: %v", err)
		}
		log.Debugf(ctx, "Waiting for handlers to stop...")
		wg.Wait()
	}()

	wg.Add(1)
	client, err := srv.LocalClient()
	if err != nil {
		// LocalClient should not return an error if server successfully started.
		return err
	}
	go func() {
		defer wg.Done()
		logStartupInfo(ctx, client)
	}()

	systemResolver := new(net.Resolver)
	for port, pc := range cfg.tcpPorts {
		log.Infof(ctx, "Listening for TCP port %d", port)
		l, err := srv.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return err
		}
		lb := newLoadBalancer(systemResolver, pc.backends)
		wg.Add(1)
		go func() {
			defer wg.Done()
			listenTCPPort(ctx, l, lb)
		}()
	}
	<-ctx.Done()
	return nil
}

func logStartupInfo(ctx context.Context, client *tailscale.LocalClient) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	var prevAuthURL string
	for {
		if err := ctx.Err(); err != nil {
			log.Debugf(ctx, "Stopping startup info poll: %v", err)
			return
		}
		status, err := client.Status(ctx)
		if err != nil {
			log.Errorf(ctx, "Unable to query Tailscale status (will retry): %v", err)
			goto wait
		}
		if status.BackendState == ipn.NeedsLogin.String() {
			if status.AuthURL != prevAuthURL {
				log.Infof(ctx, "To start this load balancer, restart with TS_AUTHKEY set, or go to: %s", status.AuthURL)
				prevAuthURL = status.AuthURL
			}
		} else if len(status.TailscaleIPs) > 0 {
			sb := new(strings.Builder)
			for i, addr := range status.TailscaleIPs {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(addr.String())
			}
			log.Infof(ctx, "Listening on Tailscale addresses: %s", sb)
			return
		} else {
			log.Debugf(ctx, "Backend state = %q and has no addresses", status.BackendState)
		}

	wait:
		select {
		case <-tick.C:
		case <-ctx.Done():
		}
	}
}

func listenTCPPort(ctx context.Context, l net.Listener, lb *loadBalancer) {
	var closeOnce sync.Once
	closeListener := func() {
		closeOnce.Do(func() {
			if err := l.Close(); err != nil {
				log.Errorf(ctx, "Closing listener: %v", err)
			}
		})
	}

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		closeListener()
	}()
	defer func() {
		cancel()
		closeListener()
		wg.Wait()
	}()

	for {
		log.Debugf(ctx, "Waiting for connection on %v", l.Addr())
		conn, err := l.Accept()
		if err != nil {
			log.Debugf(ctx, "Accept on %v returned error (stopping listener): %v", l.Addr(), err)
			return
		}
		log.Debugf(ctx, "Accepted connection from %v on %v", conn.RemoteAddr(), conn.LocalAddr())
		wg.Add(1)
		go func() {
			defer wg.Done()
			handleTCPConn(ctx, conn, lb)
		}()
	}
}

func handleTCPConn(ctx context.Context, clientConn net.Conn, lb *loadBalancer) {
	defer func() {
		if err := clientConn.Close(); err != nil {
			log.Errorf(ctx, "%v", err)
		}
	}()

	pickCtx, cancelPick := context.WithTimeout(ctx, 30*time.Second)
	backendAddr, err := lb.pick(pickCtx)
	cancelPick()
	if err != nil {
		log.Warnf(ctx, "Unable to find suitable backend for %v on %v: %v", clientConn.RemoteAddr(), clientConn.LocalAddr(), err)
		return
	}
	log.Debugf(ctx, "Picked backend %v for %v on %v", backendAddr, clientConn.RemoteAddr(), clientConn.LocalAddr())
	backendConn, err := new(net.Dialer).DialContext(ctx, "tcp", backendAddr.String())
	if err != nil {
		log.Warnf(ctx, "Connect to backend for %v on %v: %v", clientConn.RemoteAddr(), clientConn.LocalAddr(), err)
		return
	}

	grp, ctx := errgroup.WithContext(ctx)
	grp.Go(func() error {
		<-ctx.Done()
		clientConn.SetDeadline(time.Now())
		backendConn.SetDeadline(time.Now())
		return nil
	})
	grp.Go(func() error {
		if _, err := io.Copy(backendConn, clientConn); err != nil {
			log.Warnf(ctx, "Connection for %v on %v (backend %v): %v", clientConn.RemoteAddr(), clientConn.LocalAddr(), backendAddr, err)
		}
		return errConnDone
	})
	grp.Go(func() error {
		if _, err := io.Copy(clientConn, backendConn); err != nil {
			log.Warnf(ctx, "Connection for %v on %v (backend %v): %v", clientConn.RemoteAddr(), clientConn.LocalAddr(), backendAddr, err)
		}
		return errConnDone
	})
	grp.Wait()
}

var errConnDone = errors.New("connection finished")
