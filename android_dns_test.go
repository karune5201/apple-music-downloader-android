package main

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

// TestDefaultResolverUsesConfiguredServer documents and exercises the Android
// DNS bootstrap: net.DefaultResolver must resolve names through the dialer
// installed by installDNSResolver, even when the process cannot read
// /etc/resolv.conf (the Android case).
//
// Run it with an explicit server, e.g.
//
//	AMDL_DNS_TEST=1 AMDL_DNS=8.8.8.8 go test -run TestDefaultResolverUsesConfiguredServer -v .
func TestDefaultResolverUsesConfiguredServer(t *testing.T) {
	if os.Getenv("AMDL_DNS_TEST") != "1" {
		t.Skip("set AMDL_DNS_TEST=1 and AMDL_DNS=<server> to run this network test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	host := os.Getenv("AMDL_DNS_TEST_HOST")
	if host == "" {
		host = "music.apple.com"
	}
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		t.Fatalf("LookupHost(%q): %v", host, err)
	}
	if len(addrs) == 0 {
		t.Fatalf("LookupHost(%q): no addresses", host)
	}
	t.Logf("%s -> %v", host, addrs)
}
