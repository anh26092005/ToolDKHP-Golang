package services

import (
	"context"
	"dkhp-backend/models"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func reply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func noWait(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }

func TestEventRecoversFromTimeoutsRateLimitAndFailedLoginWithoutSpendingRetry(t *testing.T) {
	var calls []string
	logins, registrations, waits := 0, 0, 0
	session := &models.AutoEventSession{Active: true, Status: "running", Token: "old"}
	ctx := session.RunContext()
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/user/login") {
			logins++
			switch logins {
			case 1:
				return nil, errors.New("dial tcp: i/o timeout")
			case 2:
				return reply(503, `{"success":false}`), nil
			case 3:
				return reply(200, "<html>Portal unavailable</html>"), nil
			default:
				return reply(200, `{"success":true,"token":"fresh"}`), nil
			}
		}
		registrations++
		var body map[string]int
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if r.URL.Query().Get("eventId") != "114" || !reflect.DeepEqual(body, map[string]int{"dateId": 184, "objectId": 12}) {
			t.Fatalf("payload changed during recovery: query=%s, body=%v", r.URL.RawQuery, body)
		}
		switch registrations {
		case 1:
			return nil, errors.New("http2: timeout awaiting response headers")
		case 2:
			return reply(401, `{}`), nil
		case 3:
			if r.Header.Get("Authorization") != "Bearer fresh" {
				t.Fatal("registration did not use refreshed token")
			}
			response := reply(429, `{}`)
			response.Header.Set("Retry-After", "7")
			return response, nil
		case 4:
			return reply(500, `{"success":true}`), nil
		default:
			return reply(200, `{"success":true,"message":"Đã đăng ký"}`), nil
		}
	})}
	credentials := &models.Credentials{Username: "student", Password: "test-only"}
	portal := newPortalSession(client, "old", credentials, func(token string) { session.Token = token }, session.AddLog)
	portal.wait = func(ctx context.Context, delay time.Duration) bool {
		waits++
		if registrations == 3 && delay < 7*time.Second {
			t.Fatalf("Retry-After was ignored: %v", delay)
		}
		return ctx.Err() == nil
	}
	date, object := int64(184), int64(12)
	runEventWithPortal(ctx, portal, models.AutoEventRequest{EventID: 114, DateID: &date, ObjectID: &object, Retries: 1}, session)
	if session.Status != "success" || session.Active || session.Token != "fresh" || session.OwnerToken != "old" {
		t.Fatalf("did not recover: status=%s active=%v", session.Status, session.Active)
	}
	if logins != 4 || registrations != 5 || waits != 6 {
		t.Fatalf("unexpected request counts: login=%d register=%d wait=%d calls=%v", logins, registrations, waits, calls)
	}
	// Failed refreshes must retry login, not send the known-expired token repeatedly.
	for _, path := range calls[2:6] {
		if !strings.HasSuffix(path, "/user/login") {
			t.Fatalf("expired registration was repeated during login recovery: %v", calls)
		}
	}
}

func TestStopCancelsInFlightHTTP2Request(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if r.ProtoMajor != 2 {
			t.Errorf("wanted a real HTTP/2 request, got %s", r.Proto)
		}
		close(started)
		<-r.Context().Done()
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	client := server.Client()
	target, _ := url.Parse(server.URL)
	transport := client.Transport
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return transport.RoundTrip(r)
	})
	session := &models.AutoEventSession{Active: true, Status: "running", Token: "test"}
	ctx := session.RunContext()
	done := make(chan struct{})
	go func() {
		defer close(done)
		runAutoEventWorker(ctx, client, "test", models.AutoEventRequest{EventID: 114, Retries: 1}, session)
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP/2 request did not start")
	}
	session.Finish("stopped", nil)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel the in-flight HTTP/2 request")
	}
	if session.Status != "stopped" || session.Active {
		t.Fatal("late worker completion overwrote Stop")
	}
}

func TestClientHeaderTimeoutIsRetriedAndThenRecovers(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		mu.Lock()
		calls++
		attempt := calls
		mu.Unlock()
		if attempt == 1 {
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, `{"success":true}`)
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	client := &http.Client{Timeout: 100 * time.Millisecond, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return http.DefaultTransport.RoundTrip(r)
	})}
	portal := newPortalSession(client, "test", nil, func(string) {}, func(string, string) {})
	waits := 0
	portal.wait = func(ctx context.Context, _ time.Duration) bool { waits++; return ctx.Err() == nil }
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response, err := portal.Do(ctx, "POST", "/eventWeb/register?eventId=1", nil)
	if err != nil || !response.Success() || waits != 1 {
		t.Fatalf("timeout did not recover: response=%v err=%v waits=%d", response, err, waits)
	}
}

func TestStopCancelsBackoffAndLogin(t *testing.T) {
	for _, stage := range []string{"backoff", "login"} {
		t.Run(stage, func(t *testing.T) {
			session := &models.AutoEventSession{Active: true, Status: "running", Token: "test"}
			ctx := session.RunContext()
			started, done := make(chan struct{}), make(chan struct{})
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if stage == "login" {
					close(started)
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				return reply(503, `{}`), nil
			})}
			portal := newPortalSession(client, "test", &models.Credentials{Username: "test", Password: "test"}, func(string) {}, session.AddLog)
			portal.needsLogin = stage == "login"
			if stage == "backoff" {
				portal.wait = func(ctx context.Context, delay time.Duration) bool { close(started); return WaitContext(ctx, delay) }
			}
			go func() {
				defer close(done)
				runEventWithPortal(ctx, portal, models.AutoEventRequest{EventID: 1, Retries: 1}, session)
			}()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("worker did not reach recovery stage")
			}
			session.Finish("stopped", nil)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("worker is stuck after Stop")
			}
			if calls != 1 || session.Status != "stopped" {
				t.Fatalf("stopped run continued: calls=%d status=%s", calls, session.Status)
			}
		})
	}
}

func TestAuthenticationRejectionAndUnknownResultsAreNotSuccess(t *testing.T) {
	for _, scenario := range []struct {
		name, body  string
		status      int
		credentials bool
		want        string
	}{
		{"missing credentials", `{}`, 401, false, "auth_required"},
		{"wrong password", `{}`, 401, true, "auth_required"},
		{"unknown result", `{"message":"unknown"}`, 200, false, "finished"},
		{"explicit failure", `{"success":false,"message":"Chưa mở"}`, 200, false, "finished"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			session := &models.AutoEventSession{Active: true, Status: "running", Token: "test"}
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return reply(scenario.status, scenario.body), nil })}
			var credentials *models.Credentials
			if scenario.credentials {
				credentials = &models.Credentials{Username: "user", Password: "wrong"}
			}
			portal := newPortalSession(client, "test", credentials, func(string) {}, session.AddLog)
			portal.wait = func(context.Context, time.Duration) bool {
				t.Fatal("permanent failure must not backoff forever")
				return false
			}
			runEventWithPortal(session.RunContext(), portal, models.AutoEventRequest{EventID: 1, Retries: 1}, session)
			if session.Status != scenario.want || session.Active {
				t.Fatalf("status=%s active=%v", session.Status, session.Active)
			}
		})
	}
}

func TestSixtyGoEventSessionsRefreshIndependently(t *testing.T) {
	var wg sync.WaitGroup
	for user := 0; user < 60; user++ {
		wg.Add(1)
		go func(user int) {
			defer wg.Done()
			oldToken := fmt.Sprintf("eyJhbGciOiJIUzI1NiJ9.old-%d", user)
			freshToken := fmt.Sprintf("eyJhbGciOiJIUzI1NiJ9.new-%d", user)
			registrations, logins := 0, 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/user/login") {
					logins++
					if logins < 3 {
						return reply(503, `{}`), nil
					}
					return reply(200, fmt.Sprintf(`{"success":true,"token":%q}`, freshToken)), nil
				}
				registrations++
				if r.URL.Query().Get("eventId") != fmt.Sprint(user+1) {
					t.Error("sessions mixed event IDs")
				}
				if r.Header.Get("Authorization") == "Bearer "+oldToken {
					return reply(401, `{}`), nil
				}
				if r.Header.Get("Authorization") != "Bearer "+freshToken {
					t.Error("sessions mixed tokens")
				}
				return reply(200, `{"success":true}`), nil
			})}
			session := &models.AutoEventSession{Active: true, Status: "running", Token: oldToken}
			portal := newPortalSession(client, oldToken, &models.Credentials{Username: fmt.Sprint(user), Password: "test-only"}, func(token string) { session.Token = token }, session.AddLog)
			portal.wait = noWait
			runEventWithPortal(session.RunContext(), portal, models.AutoEventRequest{EventID: int64(user + 1), Retries: 1}, session)
			if session.Status != "success" || registrations != 2 || logins != 3 || session.OwnerToken != oldToken {
				t.Errorf("user %d did not recover independently: status=%s registrations=%d logins=%d", user, session.Status, registrations, logins)
			}
		}(user)
	}
	wg.Wait()
}

func TestCourseWorkerRefreshesAndRemovesSiblingClasses(t *testing.T) {
	var requested []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/user/login") {
			return reply(200, `{"success":true,"token":"fresh"}`), nil
		}
		if r.Header.Get("Authorization") != "Bearer fresh" {
			return reply(401, `{}`), nil
		}
		requested = append(requested, r.URL.Query().Get("idLopHocPhan"))
		return reply(200, `{"success":true}`), nil
	})}
	session := &models.AutoRegisterSession{Active: true, Status: "running", Token: "old", Credentials: &models.Credentials{Username: "student", Password: "test-only"},
		Loop: 1, PendingClassIDs: []string{"101", "102", "201"}, CourseGroups: []models.CourseGroup{{ClassIDs: []interface{}{"101", "102"}}}}
	runAutoRegistrationWorker(session.RunContext(), client, session)
	if !reflect.DeepEqual(requested, []string{"101", "201"}) || len(session.PendingClassIDs) != 0 || session.Active || session.Status != "finished" || session.Token != "fresh" {
		t.Fatalf("course worker did not recover/deduplicate: requested=%v pending=%v status=%s", requested, session.PendingClassIDs, session.Status)
	}
}

func TestRejectedFreshTokensBackoffInsteadOfLoginStorm(t *testing.T) {
	logins, waits := 0, 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/user/login") {
			logins++
			return reply(200, `{"success":true,"token":"fresh"}`), nil
		}
		return reply(401, `{}`), nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	portal := newPortalSession(client, "old", &models.Credentials{Username: "user", Password: "test-only"}, func(string) {}, func(string, string) {})
	portal.wait = func(ctx context.Context, delay time.Duration) bool {
		waits++
		if delay < 2500*time.Millisecond {
			t.Fatalf("fresh-token rejection retried too quickly: %v", delay)
		}
		if waits == 3 {
			cancel()
			return false
		}
		return true
	}
	_, err := portal.Do(ctx, "POST", "/eventWeb/register?eventId=1", nil)
	if !errors.Is(err, context.Canceled) || waits != 3 || logins != 3 {
		t.Fatalf("login loop was not throttled/cancelled: err=%v waits=%d logins=%d", err, waits, logins)
	}
}
