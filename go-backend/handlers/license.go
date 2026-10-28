package handlers

import (
	"bytes"
	"crypto/rand"
	"dkhp-backend/config"
	"dkhp-backend/services"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var licenseSessions sync.Map

const licenseCookie = "dkhp_license_session"

func GetLicenseConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"required": config.LicenseServerURL != ""})
}

// PostDesktopLicense validates against the same Pages API used by the admin.
func PostDesktopLicense(c *gin.Context) {
	if config.LicenseServerURL == "" {
		c.JSON(http.StatusOK, gin.H{"allowed": true})
		return
	}
	var input struct {
		Key string `json:"key"`
	}
	if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.Key) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"allowed": false, "message": "Vui lòng nhập mã key"})
		return
	}
	base, err := url.Parse(config.LicenseServerURL)
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"allowed": false, "message": "Địa chỉ máy chủ key chưa được cấu hình hợp lệ"})
		return
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/keys/validate"
	base.RawQuery = ""
	payload, _ := json.Marshal(gin.H{"key": strings.TrimSpace(input.Key), "deviceId": getMachineID()})
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, base.String(), bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"allowed": false, "message": "Không thể tạo yêu cầu xác thực key"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := services.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"allowed": false, "message": "Không thể kết nối máy chủ key Cloudflare"})
		return
	}
	defer resp.Body.Close()
	var result struct {
		Allowed bool   `json:"allowed"`
		Message string `json:"message"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result) != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"allowed": false, "message": "Máy chủ key trả phản hồi không hợp lệ"})
		return
	}
	if !result.Allowed {
		c.JSON(http.StatusOK, gin.H{"allowed": false, "message": result.Message})
		return
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"allowed": false, "message": "Không thể tạo phiên kích hoạt"})
		return
	}
	value := hex.EncodeToString(token[:])
	licenseSessions.Store(value, time.Now().Add(12*time.Hour))
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(licenseCookie, value, 12*60*60, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"allowed": true, "message": result.Message})
}

func RequireDesktopLicense() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.LicenseServerURL == "" || c.Request.Method == http.MethodOptions ||
			!strings.HasPrefix(c.Request.URL.Path, "/api/") ||
			c.Request.URL.Path == "/api/license/config" || c.Request.URL.Path == "/api/license/validate" {
			c.Next()
			return
		}
		if token, err := c.Cookie(licenseCookie); err == nil {
			if value, ok := licenseSessions.Load(token); ok {
				if time.Now().Before(value.(time.Time)) {
					c.Next()
					return
				}
				licenseSessions.Delete(token)
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "Vui lòng kích hoạt key"})
	}
}
