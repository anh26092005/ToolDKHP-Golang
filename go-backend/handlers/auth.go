// Package handlers - Auth Handler
// Xử lý đăng nhập, đăng xuất, kiểm tra token
package handlers

import (
	"bytes"
	"dkhp-backend/config"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"dkhp-backend/services"
)

// PostLogin xử lý đăng nhập tài khoản sinh viên qua Portal
// POST /api/user/login
func PostLogin(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Vui lòng nhập tên đăng nhập và mật khẩu",
		})
		return
	}

	log.Printf("[Auth] Đang gửi đăng nhập cho MSSV: %s", body.Username)

	// Forward login request to Portal
	loginPayload, _ := json.Marshal(map[string]string{
		"username": body.Username,
		"password": body.Password,
	})

	req, err := http.NewRequest("POST", config.PortalBaseURL+"/user/login", bytes.NewReader(loginPayload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Lỗi hệ thống"})
		return
	}
	setPortalHeaders(req, "")

	resp, err := services.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Lỗi kết nối đến server: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.JSON(resp.StatusCode, gin.H{"success": false, "message": "Phản hồi không hợp lệ từ Portal"})
		return
	}

	success, _ := result["success"].(bool)
	if !success {
		statusCode := resp.StatusCode
		if statusCode == 200 {
			statusCode = http.StatusOK
		}
		c.JSON(statusCode, result)
		return
	}

	token, _ := result["token"].(string)

	// Lấy thông tin sinh viên
	var studentInfo interface{}
	profileReq, _ := http.NewRequest("GET", config.PortalBaseURL+"/user/getSummaryProfile", nil)
	setPortalHeaders(profileReq, token)
	profileResp, err := services.DefaultClient.Do(profileReq)
	if err == nil {
		defer profileResp.Body.Close()
		profileBody, _ := io.ReadAll(profileResp.Body)
		var profileResult map[string]interface{}
		if json.Unmarshal(profileBody, &profileResult) == nil {
			if profileResult["success"] == true {
				studentInfo = profileResult["body"]
			}
		}
	}
	if studentInfo == nil {
		studentInfo = map[string]string{"maSinhVien": body.Username}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Đăng nhập thành công",
		"token":       token,
		"studentInfo": studentInfo,
	})
}

// GetSummaryProfile proxy lấy thông tin sinh viên
// GET /api/user/getSummaryProfile
func GetSummaryProfile(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	req, _ := http.NewRequest("GET", config.PortalBaseURL+"/user/getSummaryProfile", nil)
	setPortalHeaders(req, token)

	resp, err := services.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Lỗi kết nối"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json; charset=utf-8", body)
}

// PostLogout xử lý đăng xuất
// POST /api/user/logout
func PostLogout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Đăng xuất thành công",
	})
}

// GetValidate kiểm tra session/token còn hiệu lực không
// GET /api/user/validate
func GetValidate(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Chưa đăng nhập"})
		return
	}

	req, _ := http.NewRequest("GET", config.PortalBaseURL+"/dkhp/getHocPhanHocMoi?idDot=0", nil)
	setPortalHeaders(req, token)

	resp, err := services.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Không thể kiểm tra token"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Token đã hết hạn"})
		return
	}

	if resp.StatusCode == 200 {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Token hợp lệ"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": false, "message": "Không thể kiểm tra token"})
}

// GetCurrentUser trả về thông tin user hiện tại (client-side token)
// GET /api/user/me
func GetCurrentUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "Chưa đăng nhập",
	})
}

// ========== Helper ==========

// extractToken lấy Bearer token từ header Authorization
func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// setPortalHeaders thiết lập các header chuẩn cho request proxy đến Portal
func setPortalHeaders(req *http.Request, token string) {
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Origin", config.PortalOrigin)
	req.Header.Set("Referer", config.PortalOrigin+"/")
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
