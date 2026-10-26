package main

// Android-friendly DNS bootstrap.
//
// Go prefers the cgo resolver on Android because /etc/resolv.conf does not
// exist there (see net/conf.go, issue #10714). A CGO_ENABLED=0 build therefore
// ends up with an unusable resolver: every lookup fails with "no such host".
//
// Instead of requiring a cgo/NDK toolchain, this file installs a pure-Go
// resolver that talks to a DNS server chosen at start-up:
//
//  1. AMDL_DNS (comma/space separated addresses) when set - also honoured on
//     desktop builds, which makes the behaviour testable off-device.
//  2. On Android, the net.dns* system properties reported by getprop.
//  3. A built-in list of public resolvers.
//
// The first server that answers a probe query is used, the rest stay as
// fallbacks for later dials.

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var defaultPublicDNS = []string{
	"223.5.5.5:53",
	"119.29.29.29:53",
	"1.1.1.1:53",
	"8.8.8.8:53",
}

func init() {
	installDNSResolver()
}

func installDNSResolver() {
	servers := dnsServersFromEnv()
	if len(servers) == 0 && runtime.GOOS == "android" {
		servers = dnsServersFromGetprop()
	}
	if len(servers) == 0 {
		servers = append(servers, defaultPublicDNS...)
	}

	// Prefer servers that actually answer, keeping the rest as fallbacks.
	ordered := probeDNS(servers)
	if len(ordered) == 0 {
		ordered = servers
	}

	var (
		mu  sync.Mutex
		cur = 0
	)
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			mu.Lock()
			server := ordered[cur%len(ordered)]
			cur++
			mu.Unlock()
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", server)
		},
	}
}

func dnsServersFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("AMDL_DNS"))
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';' || r == '\n'
	}) {
		if s := normalizeDNSServer(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func normalizeDNSServer(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.Contains(v, ":") {
		// Already has a port (either IPv4:port or IPv6 literal).
		if ip, port, err := net.SplitHostPort(v); err == nil {
			if p := strings.TrimSpace(port); p != "" {
				if ip == "" {
					return ""
				}
				return net.JoinHostPort(ip, p)
			}
		}
		// Bare IPv6 without brackets.
		if strings.Count(v, ":") >= 2 {
			return net.JoinHostPort(v, "53")
		}
		return ""
	}
	return net.JoinHostPort(v, "53")
}

func dnsServersFromGetprop() []string {
	prop := func(name string) string {
		out, err := exec.Command("/system/bin/getprop", name).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	var out []string
	for _, key := range []string{"net.dns1", "net.dns2", "net.dns3", "net.dns4"} {
		if s := normalizeDNSServer(prop(key)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// probeDNS returns the responsive servers first, in probe order. Probes run
// concurrently so a fully unreachable network costs one timeout, not one per
// server.
func probeDNS(servers []string) []string {
	results := make([]bool, len(servers))
	var wg sync.WaitGroup
	for i, server := range servers {
		wg.Add(1)
		go func(i int, server string) {
			defer wg.Done()
			results[i] = probeOne(server)
		}(i, server)
	}
	wg.Wait()

	ok := make([]string, 0, len(servers))
	bad := make([]string, 0)
	for i, server := range servers {
		if results[i] {
			ok = append(ok, server)
		} else {
			bad = append(bad, server)
		}
	}
	return append(ok, bad...)
}

func probeOne(server string) bool {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, "udp", server)
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if _, err := r.LookupHost(ctx, "music.apple.com"); err != nil {
		fmt.Fprintf(os.Stderr, "dns: %s not usable (%v)\n", server, err)
		return false
	}
	return true
}
