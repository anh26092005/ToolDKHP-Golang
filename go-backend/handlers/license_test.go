package handlers

import (
	"dkhp-backend/config"
	"dkhp-backend/services"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDesktopLicenseValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	portal := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/keys/validate" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(readTestBody(t, r), `"key":"GOOD"`) {
			_, _ = w.Write([]byte(`{"allowed":true}`))
		} else {
			_, _ = w.Write([]byte(`{"allowed":false,"message":"invalid"}`))
		}
	}))
	defer portal.Close()
	oldURL, oldClient := config.LicenseServerURL, services.DefaultClient
	config.LicenseServerURL, services.DefaultClient = portal.URL, portal.Client()
	t.Cleanup(func() { config.LicenseServerURL, services.DefaultClient = oldURL, oldClient })

	r := gin.New()
	r.Use(RequireDesktopLicense())
	r.POST("/api/license/validate", PostDesktopLicense)
	r.GET("/api/private", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		r.ServeHTTP(w, req)
		return w
	}
	if got := request("GET", "/api/private", "", nil).Code; got != http.StatusForbidden {
		t.Fatalf("unlicensed status = %d", got)
	}
	bad := request("POST", "/api/license/validate", `{"key":"BAD"}`, nil)
	if len(bad.Result().Cookies()) != 0 {
		t.Fatal("invalid key created a session")
	}
	good := request("POST", "/api/license/validate", `{"key":"GOOD"}`, nil)
	cookies := good.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected license cookie, got %d", len(cookies))
	}
	if got := request("GET", "/api/private", "", cookies[0]).Code; got != http.StatusNoContent {
		t.Fatalf("licensed status = %d", got)
	}
}

func readTestBody(t *testing.T, r *http.Request) string {
	t.Helper()
	buf, _ := io.ReadAll(r.Body)
	return string(buf)
}
