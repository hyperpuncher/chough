package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type ipResolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type urlPolicy struct {
	allowPrivate bool
	resolver     ipResolver
}

func newURLPolicy(allowPrivate bool, resolver ipResolver) *urlPolicy {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &urlPolicy{allowPrivate: allowPrivate, resolver: resolver}
}

func (p *urlPolicy) validate(ctx context.Context, rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL scheme must be http or https")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("URL credentials are not allowed")
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("URL host is required")
	}
	if _, err := p.resolve(ctx, parsed.Hostname()); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (p *urlPolicy) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	var addresses []netip.Addr
	if address, err := netip.ParseAddr(host); err == nil {
		addresses = []netip.Addr{address}
	} else {
		resolved, err := p.resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve URL host: %w", err)
		}
		addresses = resolved
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("URL host resolved to no addresses")
	}
	if p.allowPrivate {
		return addresses, nil
	}
	for _, address := range addresses {
		if isBlockedAddress(address) {
			return nil, fmt.Errorf("URL resolves to blocked address %s", address)
		}
	}
	return addresses, nil
}

func isBlockedAddress(address netip.Addr) bool {
	address = address.Unmap()
	return address.IsPrivate() ||
		address.IsLoopback() ||
		address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() ||
		address.IsInterfaceLocalMulticast() ||
		address.IsMulticast() ||
		address.IsUnspecified()
}

func newDownloadClient(policy *urlPolicy) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid download address: %w", err)
			}
			addresses, err := policy.resolve(ctx, host)
			if err != nil {
				return nil, err
			}
			var dialErrors []error
			for _, resolved := range addresses {
				connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
				if err == nil {
					return connection, nil
				}
				dialErrors = append(dialErrors, err)
			}
			return nil, errors.Join(dialErrors...)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if request.URL.User != nil {
				return fmt.Errorf("redirect credentials are not allowed")
			}
			if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
				return fmt.Errorf("redirect scheme must be http or https")
			}
			if strings.TrimSpace(request.URL.Hostname()) == "" {
				return fmt.Errorf("redirect host is required")
			}
			_, err := policy.resolve(request.Context(), request.URL.Hostname())
			return err
		},
	}
}
