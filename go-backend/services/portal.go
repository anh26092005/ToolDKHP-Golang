package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dkhp-backend/config"
	"dkhp-backend/models"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrAuthenticationRequired = errors.New("cần đăng nhập lại: thiếu thông tin đăng nhập hoặc Portal từ chối tài khoản/mật khẩu")

// TokenSessionID uses the entire token; JWTs commonly share their first 16 characters.
func TokenSessionID(token string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
}

type PortalResponse struct {
	StatusCode int
	Data       map[string]interface{}
	RetryAfter time.Duration
}

func (r *PortalResponse) Success() bool {
	return r != nil && r.StatusCode >= 200 && r.StatusCode < 300 && r.Data["success"] == true
}

func (r *PortalResponse) Temporary() bool {
	return r.StatusCode == http.StatusRequestTimeout || r.StatusCode == http.StatusTooManyRequests ||
		r.StatusCode >= 500 || (r.StatusCode >= 300 && r.StatusCode < 400) ||
		(r.Data == nil && r.StatusCode >= 200 && r.StatusCode < 300)
}

func (r *PortalResponse) Unauthorized() bool {
	if r.StatusCode == http.StatusUnauthorized {
		return true
	}
	if r.StatusCode < 200 || r.StatusCode >= 300 || r.Data["success"] == true {
		return false
	}
	message := strings.ToLower(getStringField(r.Data, "message", ""))
	return strings.Contains(message, "unauthorized") || strings.Contains(message, "hết phiên") ||
		strings.Contains(message, "phiên làm việc hết hạn") ||
		(strings.Contains(message, "token") && (strings.Contains(message, "hết hạn") ||
			strings.Contains(message, "expired") || strings.Contains(message, "invalid")))
}

// RequestPortal bounds every attempt (including response-body reads), and allows Stop to cancel it.
// Retry policy belongs to the worker, rather than the HTTP transport or the request handler.
func RequestPortal(ctx context.Context, client *http.Client, method, endpoint string, data interface{}, token string) (*PortalResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, config.RequestTimeout)
	defer cancel()
	var body io.Reader
	if data != nil {
		encoded, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	} else if method == http.MethodPost {
		body = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, method, config.PortalBaseURL+endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", config.PortalOrigin)
	req.Header.Set("Referer", config.PortalOrigin+"/")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	encoded, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	result := &PortalResponse{StatusCode: response.StatusCode}
	// Invalid/empty/HTML responses must never be interpreted as registration success.
	if json.Unmarshal(encoded, &result.Data) != nil {
		result.Data = nil
	}
	if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 {
		result.RetryAfter = time.Duration(min(seconds, 300)) * time.Second
	} else if until, err := http.ParseTime(response.Header.Get("Retry-After")); err == nil {
		result.RetryAfter = min(max(time.Until(until), 0), 5*time.Minute)
	}
	return result, nil
}

func WaitContext(ctx context.Context, delay time.Duration) bool {
	if ctx.Err() != nil {
		return false
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.Err() == nil
	case <-ctx.Done():
		return false
	}
}

// One instance is owned by one worker: requests and login refreshes stay serialized.
type portalSession struct {
	client      *http.Client
	token       string
	credentials *models.Credentials
	onToken     func(string)
	log         func(string, string)
	wait        func(context.Context, time.Duration) bool
	needsLogin  bool
	failures    int
}

func newPortalSession(client *http.Client, token string, credentials *models.Credentials, onToken func(string), log func(string, string)) *portalSession {
	return &portalSession{client: client, token: token, credentials: credentials, onToken: onToken, log: log, wait: WaitContext}
}

func (p *portalSession) backoff(ctx context.Context, response *PortalResponse, stage string) bool {
	p.failures++
	// Jitter spreads out accounts recovering at the same time, without a retry storm.
	delay := min(time.Duration(2000+min(p.failures, 6)*500)*time.Millisecond, 5*time.Second)
	delay += time.Duration(rand.Intn(501)) * time.Millisecond
	if response != nil && response.RetryAfter > delay {
		delay = response.RetryAfter
	}
	p.log(fmt.Sprintf("⚠️ %s: Portal/kết nối chưa sẵn sàng. Thử lại sau %.1fs; giữ nguyên lượt thử.", stage, delay.Seconds()), "warning")
	return p.wait(ctx, delay)
}

func (p *portalSession) Do(ctx context.Context, method, endpoint string, data interface{}) (*PortalResponse, error) {
	refreshed := false
	for ctx.Err() == nil {
		if p.needsLogin {
			if p.credentials == nil || p.credentials.Username == "" || p.credentials.Password == "" {
				return nil, ErrAuthenticationRequired
			}
			p.log("🔄 Phiên hết hạn. Đang đăng nhập lại (mỗi yêu cầu tối đa 15 giây)...", "warning")
			response, err := RequestPortal(ctx, p.client, http.MethodPost, "/user/login", p.credentials, "")
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if err != nil || response.Temporary() {
				if !p.backoff(ctx, response, "Đăng nhập lại") {
					return nil, ctx.Err()
				}
				continue
			}
			token := getStringField(response.Data, "token", "")
			if !response.Success() {
				return nil, ErrAuthenticationRequired
			}
			if token == "" {
				if !p.backoff(ctx, response, "Portal chưa cấp token") {
					return nil, ctx.Err()
				}
				continue
			}
			p.token, p.needsLogin = token, false
			refreshed = true
			p.onToken(token)
			p.log("✅ Đã đăng nhập lại; tiếp tục yêu cầu đang chờ với token mới.", "success")
		}

		response, err := RequestPortal(ctx, p.client, method, endpoint, data, p.token)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err == nil && response.Unauthorized() {
			// A new token can also be rejected while Portal authentication is unstable.
			// Do not turn this into an unthrottled login -> registration -> 401 loop.
			if refreshed && !p.backoff(ctx, response, "Portal chưa chấp nhận phiên mới") {
				return nil, ctx.Err()
			}
			p.needsLogin = true
			continue
		}
		if err != nil || response.Temporary() {
			if !p.backoff(ctx, response, "Đăng ký") {
				return nil, ctx.Err()
			}
			continue
		}
		p.failures = 0
		return response, nil
	}
	return nil, ctx.Err()
}
