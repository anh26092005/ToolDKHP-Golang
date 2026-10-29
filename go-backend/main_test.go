package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemovedAdminAndKeyRoutes(t *testing.T) {
	router := setupRouter()
	for _, path := range []string{"/admin.html", "/api/keys/validate", "/api/keys/admin/login"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s: got status %d, want 404", path, response.Code)
		}
	}
}
