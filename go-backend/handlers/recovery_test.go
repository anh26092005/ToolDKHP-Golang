package handlers

import (
	"context"
	"dkhp-backend/models"
	"dkhp-backend/services"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func invoke(handler gin.HandlerFunc, path, body, token string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest("POST", path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if token != "" {
		c.Request.Header.Set("Authorization", "Bearer "+token)
	}
	handler(c)
	return response
}

func TestLoginTimeoutAndGatewayReturnRetryableStructuredErrors(t *testing.T) {
	previous := services.DefaultClient
	t.Cleanup(func() { services.DefaultClient = previous })
	services.DefaultClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: context.DeadlineExceeded}
	})}
	for _, scenario := range []struct {
		handler    gin.HandlerFunc
		path, body string
		gateway    bool
	}{
		{PostLogin, "/api/user/login", `{"username":"test","password":"test-only"}`, false},
		{GatewayHandler, "/api/gateway", `{"method":"POST","endpoint":"/eventWeb/register?eventId=1"}`, true},
	} {
		response := invoke(scenario.handler, scenario.path, scenario.body, "test")
		if response.Code != http.StatusGatewayTimeout {
			t.Fatalf("got %d: %s", response.Code, response.Body)
		}
		if !scenario.gateway {
			var data map[string]interface{}
			json.Unmarshal(response.Body.Bytes(), &data)
			if data["isNetworkError"] != true || data["success"] != false {
				t.Fatalf("missing recovery flags: %v", data)
			}
		}
		if strings.Contains(response.Body.String(), "test-only") {
			t.Fatal("password leaked in error")
		}
	}
}

func TestSuccessfulLoginDoesNotWaitForOptionalProfile(t *testing.T) {
	previous := services.DefaultClient
	t.Cleanup(func() { services.DefaultClient = previous })
	calls := 0
	services.DefaultClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if !strings.HasSuffix(r.URL.Path, "/user/login") {
			t.Fatal("login waited for optional profile")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"success":true,"token":"test-token"}`))}, nil
	})}
	response := invoke(PostLogin, "/api/user/login", `{"username":"test","password":"test-only"}`, "")
	var data map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &data)
	if response.Code != 200 || calls != 1 || data["success"] != true || data["token"] != "test-token" {
		t.Fatalf("login failed: %s", response.Body)
	}
}

func TestJWTEventSessionsDoNotCollideAndRemainAccessibleAfterRefresh(t *testing.T) {
	firstToken, secondToken := "eyJhbGciOiJIUzI1NiJ9.first", "eyJhbGciOiJIUzI1NiJ9.second"
	firstID, secondID := services.TokenSessionID(firstToken), services.TokenSessionID(secondToken)
	if firstID == secondID {
		t.Fatal("JWT session keys collide")
	}
	first := &models.AutoEventSession{Token: "refreshed-first", OwnerToken: firstToken, Active: true, Status: "running"}
	second := &models.AutoEventSession{Token: secondToken, Active: true, Status: "running"}
	first.RunContext()
	second.RunContext()
	services.AutoEventSessions.Store(firstID, first)
	services.AutoEventSessions.Store(secondID, second)
	t.Cleanup(func() {
		first.Finish("stopped", nil)
		second.Finish("stopped", nil)
		services.AutoEventSessions.Delete(firstID)
		services.AutoEventSessions.Delete(secondID)
	})
	if findEventSession(firstToken) != first || findEventSession("refreshed-first") != first || findEventSession(secondToken) != second {
		t.Fatal("wrong session returned after refresh")
	}
	invoke(PostAutoEventStop, "/api/events/auto/stop", `{}`, firstToken)
	if first.Status != "stopped" || !second.Active {
		t.Fatal("stopping one JWT affected another session")
	}
}

func TestRefreshedTokenCannotStartDuplicateActiveEventWorker(t *testing.T) {
	first := &models.AutoEventSession{Token: "fresh", OwnerToken: "old", Active: true, Status: "running"}
	first.RunContext()
	id := services.TokenSessionID("old")
	services.AutoEventSessions.Store(id, first)
	t.Cleanup(func() { first.Finish("stopped", nil); services.AutoEventSessions.Delete(id) })
	response := invoke(PostAutoEventStart, "/api/events/auto/start", `{"eventId":114}`, "fresh")
	var data map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &data)
	if data["success"] != false {
		t.Fatalf("started duplicate worker: %s", response.Body)
	}
}
