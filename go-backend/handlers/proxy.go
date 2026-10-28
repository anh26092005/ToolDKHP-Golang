// Package handlers - Proxy Handler
// Proxy chuyển tiếp request đến Portal trường (dkhp, dkhpdk, eventWeb, gateway)
package handlers

import (
	"bytes"
	"dkhp-backend/config"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"dkhp-backend/services"
)

// ProxyHandler chuyển tiếp generic request tới Portal
// Dùng cho: /api/dkhp/*, /api/dkhpdk/*, /api/eventWeb/*
func ProxyHandler(c *gin.Context) {
	// Construct path: /api/dkhp/... → /dkhp/...
	originalPath := c.Request.URL.Path
	path := strings.Replace(originalPath, "/api", "", 1)
	if c.Request.URL.RawQuery != "" {
		path = path + "?" + c.Request.URL.RawQuery
	}
	targetURL := config.PortalBaseURL + path
	method := c.Request.Method

	// Token from header
	token := extractToken(c)

	headers := map[string]string{
		"User-Agent":      config.UserAgent,
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7",
		"Origin":          config.PortalOrigin,
		"Referer":         config.PortalOrigin + "/",
	}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}

	log.Printf("Proxying %s %s", method, targetURL)

	// Read request body
	var bodyReader io.Reader
	if c.Request.Body != nil {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		if len(bodyBytes) > 0 {
			bodyReader = bytes.NewReader(bodyBytes)
		}
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Lỗi tạo request"})
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := services.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Lỗi kết nối đến Portal"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json; charset=utf-8", respBody)
}

// GatewayHandler xử lý endpoint /api/gateway (obfuscated proxy)
// Frontend gửi payload mã hóa base64, server giải mã rồi proxy, sau đó mã hóa response trả về
// POST /api/gateway
func GatewayHandler(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid gateway payload"})
		return
	}

	// Decode base64 payload nếu có field "p"
	if p, ok := body["p"].(string); ok && p != "" {
		decoded, err := base64.StdEncoding.DecodeString(p)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid gateway payload"})
			return
		}
		if err := json.Unmarshal(decoded, &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid gateway payload"})
			return
		}
	}

	method, _ := body["method"].(string)
	if method == "" {
		method = "GET"
	}
	endpoint, _ := body["endpoint"].(string)
	if endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Missing endpoint"})
		return
	}
	data := body["data"]
	payloadToken, _ := body["token"].(string)

	// Token priority: payload → header
	effectiveToken := payloadToken
	if effectiveToken == "" {
		effectiveToken = extractToken(c)
	}

	headers := map[string]string{
		"User-Agent":      config.UserAgent,
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7",
		"Origin":          config.PortalOrigin,
		"Referer":         config.PortalOrigin + "/",
	}
	if effectiveToken != "" {
		headers["Authorization"] = "Bearer " + effectiveToken
	}

	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	targetURL := config.PortalBaseURL + endpoint

	var bodyReader io.Reader
	if data != nil {
		dataBytes, _ := json.Marshal(data)
		bodyReader = bytes.NewReader(dataBytes)
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		sendGatewayError(c, http.StatusInternalServerError, "Lỗi tạo request")
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := services.DefaultClient.Do(req)
	if err != nil {
		sendGatewayError(c, http.StatusInternalServerError, "Lỗi kết nối đến Portal")
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Encode response as base64
	encoded := base64.StdEncoding.EncodeToString(respBody)
	c.JSON(resp.StatusCode, gin.H{"r": encoded})
}

// sendGatewayError gửi lỗi dạng gateway (base64-encoded)
func sendGatewayError(c *gin.Context, status int, message string) {
	errObj, _ := json.Marshal(map[string]interface{}{"success": false, "message": message})
	encoded := base64.StdEncoding.EncodeToString(errObj)
	c.JSON(status, gin.H{"r": encoded})
}
