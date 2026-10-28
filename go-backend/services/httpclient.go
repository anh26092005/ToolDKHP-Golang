// Package services cung cấp HTTP Client tùy biến với Connection Pooling và Proxy support
package services

import (
	"crypto/tls"
	"dkhp-backend/config"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// NewHTTPClient tạo http.Client mặc định với Connection Pooling tối ưu
func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		ForceAttemptHTTP2: true, // Kích hoạt HTTP/2 multiplexing song song siêu tốc

		// Connection Pooling tối ưu cho high-concurrency
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     200,
		IdleConnTimeout:     120 * time.Second,

		// Timeouts kết nối
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: config.RequestTimeout,
		ExpectContinueTimeout: 1 * time.Second,

		// TLS config
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
	}
}

// NewHTTPClientWithProxy tạo http.Client có cấu hình Proxy (HTTP hoặc SOCKS5)
//
// proxyURL có dạng:
//   - HTTP:   http://user:pass@host:port  hoặc  http://host:port
//   - SOCKS5: socks5://user:pass@host:port  hoặc  socks5://host:port
func NewHTTPClientWithProxy(proxyURL string) (*http.Client, error) {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     200,
		IdleConnTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		ResponseHeaderTimeout: config.RequestTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	switch parsed.Scheme {
	case "socks5", "socks5h":
		// SOCKS5 proxy
		var auth *proxy.Auth
		if parsed.User != nil {
			password, _ := parsed.User.Password()
			auth = &proxy.Auth{
				User:     parsed.User.Username(),
				Password: password,
			}
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}
		transport.DialContext = nil
		transport.Dial = dialer.Dial //nolint:staticcheck
	default:
		// HTTP/HTTPS proxy
		transport.Proxy = http.ProxyURL(parsed)
		transport.DialContext = (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
	}, nil
}

// DefaultClient là HTTP Client dùng chung trong toàn ứng dụng (singleton)
var DefaultClient = NewHTTPClient()
