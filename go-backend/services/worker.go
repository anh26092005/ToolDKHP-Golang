// Package services - Auto DKHP Worker Engine
// Engine xử lý đăng ký học phần tự động chạy nền bằng Goroutines
package services

import (
	"bytes"
	"context"
	"dkhp-backend/config"
	"dkhp-backend/models"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AutoRegisterSessions lưu trữ tất cả phiên đăng ký tự động (thread-safe)
var AutoRegisterSessions sync.Map // map[string]*models.AutoRegisterSession

// StartAutoRegistration khởi tạo và chạy worker đăng ký tự động
func StartAutoRegistration(session *models.AutoRegisterSession) {
	// Chọn HTTP client (có proxy hoặc mặc định)
	client := DefaultClient
	if session.ProxyURL != "" {
		proxyClient, err := NewHTTPClientWithProxy(session.ProxyURL)
		if err != nil {
			session.AddLog(fmt.Sprintf("❌ Lỗi cấu hình Proxy '%s': %s. Dùng kết nối trực tiếp.", session.ProxyURL, err), "error")
		} else {
			client = proxyClient
			session.AddLog(fmt.Sprintf("🔒 Sử dụng Proxy: %s", session.ProxyURL), "info")
		}
	}

	if session.Status == "waiting" {
		// Đã có target time, chờ trước rồi chạy
		return
	}

	// Chạy worker trong goroutine
	go runAutoRegistrationWorker(client, session)
}

// ScheduleAutoRegistration hẹn giờ rồi chạy worker
func ScheduleAutoRegistration(session *models.AutoRegisterSession, targetTimeStr string) {
	client := DefaultClient
	if session.ProxyURL != "" {
		proxyClient, err := NewHTTPClientWithProxy(session.ProxyURL)
		if err != nil {
			session.AddLog(fmt.Sprintf("❌ Lỗi cấu hình Proxy: %s. Dùng kết nối trực tiếp.", err), "error")
		} else {
			client = proxyClient
			session.AddLog(fmt.Sprintf("🔒 Sử dụng Proxy: %s", session.ProxyURL), "info")
		}
	}

	// Đồng bộ giờ với server trường
	offset := SyncTimeOffset(client)
	session.AddLog(fmt.Sprintf("⏳ Máy bạn lệch %ds so với giờ chuẩn. Hệ thống đã tự động bù giờ!", int(offset.Seconds())), "warning")

	waitDuration, err := CalculateWaitDuration(targetTimeStr, offset)
	if err != nil {
		session.AddLog(fmt.Sprintf("❌ Lỗi parse thời gian hẹn: %s", err), "error")
		session.Status = "finished"
		return
	}

	session.AddLog(fmt.Sprintf("⏳ Đang đợi đến %s (Giờ chuẩn) để bắt đầu ĐKHP... (còn ~%.1f giây)", targetTimeStr, waitDuration.Seconds()), "info")

	ctx, cancel := context.WithCancel(context.Background())
	session.Mu.Lock()
	session.CancelFunc = cancel
	session.Mu.Unlock()

	go func() {
		select {
		case <-time.After(waitDuration):
			session.Mu.Lock()
			if session.Active {
				session.Status = "running"
				session.Mu.Unlock()
				runAutoRegistrationWorker(client, session)
			} else {
				session.Mu.Unlock()
			}
		case <-ctx.Done():
			log.Printf("[AutoDKHP][%s] Timer đã bị hủy", session.SessionID)
		}
	}()
}

// runAutoRegistrationWorker là vòng lặp chính của engine đăng ký
func runAutoRegistrationWorker(client *http.Client, session *models.AutoRegisterSession) {
	currentToken := session.Token

	// Xác định URL API dựa trên type
	var registerBaseURL string
	switch session.Type {
	case "extra":
		registerBaseURL = config.PortalBaseURL + "/dkhpdk/dangKyLopHocPhan"
	default: // main, improvement
		registerBaseURL = config.PortalBaseURL + "/dkhp/dangKyLopHocPhan"
	}

	session.AddLog(
		fmt.Sprintf("🚀 Bắt đầu tiến trình ĐKHP ngầm: %d môn (%s) - Tối đa %d lượt",
			len(session.PendingClassIDs),
			strings.Join(session.PendingClassIDs, ", "),
			session.Loop),
		"info",
	)
	session.AddLog("🛡️ Tự phục hồi: Đã bật Auto Re-login khi đứt phiên & chống mất lượt khi cổng trường quá tải.", "info")

	currentIteration := 0

	for currentIteration < session.Loop && session.Active && len(session.PendingClassIDs) > 0 {
		session.Mu.Lock()
		pendingCount := len(session.PendingClassIDs)
		session.Mu.Unlock()

		session.AddLog(
			fmt.Sprintf("===== Lượt thử %d/%d (Còn %d môn cần ĐK) =====", currentIteration+1, session.Loop, pendingCount),
			"info",
		)

		session.Mu.Lock()
		currentBatch := make([]string, len(session.PendingClassIDs))
		copy(currentBatch, session.PendingClassIDs)
		session.Mu.Unlock()

		iterationRetryNeeded := false

		for _, classID := range currentBatch {
			if !session.Active {
				session.Mu.Lock()
				session.Status = "stopped"
				session.Mu.Unlock()
				return
			}

			// Kiểm tra xem classID còn trong pending không
			session.Mu.Lock()
			found := false
			for _, pid := range session.PendingClassIDs {
				if pid == classID {
					found = true
					break
				}
			}
			session.Mu.Unlock()
			if !found {
				continue
			}

			session.AddLog(fmt.Sprintf("🔄 Thử đăng ký lớp: %s", classID), "info")

			// Gửi request đăng ký
			registerURL := fmt.Sprintf("%s?idLopHocPhan=%s", registerBaseURL, classID)
			req, err := http.NewRequest("POST", registerURL, bytes.NewReader([]byte("{}")))
			if err != nil {
				session.AddLog(fmt.Sprintf("❌ Lỗi tạo request: %s (Lớp: %s)", err, classID), "error")
				continue
			}
			req.Header.Set("Authorization", "Bearer "+currentToken)
			req.Header.Set("User-Agent", config.UserAgent)
			req.Header.Set("Accept", "application/json, text/plain, */*")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", config.PortalOrigin)
			req.Header.Set("Referer", config.PortalOrigin+"/")

			resp, err := client.Do(req)
			if err != nil {
				// Network error
				errMsg := err.Error()
				if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "connection reset") || strings.Contains(errMsg, "EOF") {
					session.AddLog(fmt.Sprintf("⚠️ Cổng trường quá tải/nghẽn mạng (%s). Giữ nguyên lượt thử, tự động chờ 2.5s rồi thử lại...", errMsg), "warning")
					time.Sleep(2500 * time.Millisecond)
					iterationRetryNeeded = true
					break
				}
				session.AddLog(fmt.Sprintf("❌ Lỗi kết nối: %s (Lớp: %s)", errMsg, classID), "error")
				continue
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			statusCode := resp.StatusCode

			if statusCode == 401 {
				session.AddLog("⚠️ Token hết hạn hoặc bị ngắt! Đang thử đăng nhập lại...", "warning")
				newToken := tryServerRelogin(client, session)
				if newToken != "" {
					currentToken = newToken
					iterationRetryNeeded = true
					break
				}
			} else if statusCode >= 500 {
				errMsg := string(bodyBytes)
				session.AddLog(fmt.Sprintf("⚠️ Cổng trường quá tải/nghẽn mạng (HTTP %d: %s). Giữ nguyên lượt thử, tự động chờ 2.5s rồi thử lại...", statusCode, truncate(errMsg, 100)), "warning")
				time.Sleep(2500 * time.Millisecond)
				iterationRetryNeeded = true
				break
			} else {
				// Parse response
				var resData map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &resData); err != nil {
					session.AddLog(fmt.Sprintf("❌ Lỗi parse response: %s (Lớp: %s)", err, classID), "error")
					continue
				}

				isSuccess := checkRegistrationSuccess(resData)
				message := getStringField(resData, "message", "")
				if message == "" {
					if isSuccess {
						message = "Đăng ký thành công"
					} else {
						message = "Đăng ký thất bại"
					}
				}

				if isSuccess {
					session.AddLog(fmt.Sprintf("✅ THÀNH CÔNG: %s (Lớp: %s)", message, classID), "success")

					session.Mu.Lock()
					// Thêm vào danh sách thành công
					alreadySuccess := false
					for _, sid := range session.SuccessfulClassIDs {
						if sid == classID {
							alreadySuccess = true
							break
						}
					}
					if !alreadySuccess {
						session.SuccessfulClassIDs = append(session.SuccessfulClassIDs, classID)
					}

					// Tìm nhóm môn học và loại bỏ các lớp dự phòng
					removedSiblings := removeSiblingClasses(session, classID)
					session.Mu.Unlock()

					if removedSiblings != "" {
						session.AddLog(removedSiblings, "success")
					} else {
						session.Mu.Lock()
						session.PendingClassIDs = removeFromSlice(session.PendingClassIDs, classID)
						remaining := len(session.PendingClassIDs)
						session.Mu.Unlock()
						session.AddLog(fmt.Sprintf("🗑️ Đã xóa lớp %s khỏi danh sách chờ. Còn lại: %d môn", classID, remaining), "info")
					}

					session.Mu.Lock()
					if len(session.PendingClassIDs) == 0 {
						session.Status = "finished"
						session.Result = map[string]interface{}{
							"message":            "Đã đăng ký thành công tất cả các lớp",
							"successfulClassIds": session.SuccessfulClassIDs,
						}
						session.Mu.Unlock()
						session.AddLog("🎉 ĐÃ ĐĂNG KÝ THÀNH CÔNG TẤT CẢ CÁC LỚP!", "success")
						return
					}
					session.Mu.Unlock()
				} else {
					session.AddLog(fmt.Sprintf("❌ Thất bại: %s (Lớp: %s)", message, classID), "error")
				}
			}
		}

		if iterationRetryNeeded {
			continue // Thử lại batch, không tăng iteration
		}

		currentIteration++

		// Chờ delay ngẫu nhiên trước lượt tiếp theo
		session.Mu.Lock()
		remainingLoop := currentIteration < session.Loop && session.Active && len(session.PendingClassIDs) > 0
		session.Mu.Unlock()

		if remainingLoop {
			delay := session.Delay
			delayMax := session.DelayMax
			if delayMax <= delay {
				delayMax = delay + 1
			}
			randomDelay := delay + rand.Intn(delayMax-delay+1)
			session.AddLog(fmt.Sprintf("⏳ Chờ %dms trước lượt tiếp theo...", randomDelay), "warning")
			time.Sleep(time.Duration(randomDelay) * time.Millisecond)
		}
	}

	session.Mu.Lock()
	session.Status = "finished"
	pendingLeft := len(session.PendingClassIDs)
	session.Mu.Unlock()

	if pendingLeft == 0 {
		session.AddLog("🎉 ĐÃ ĐĂNG KÝ THÀNH CÔNG TẤT CẢ CÁC LỚP!", "success")
	} else {
		session.Mu.Lock()
		pendingStr := strings.Join(session.PendingClassIDs, ", ")
		session.Mu.Unlock()
		session.AddLog(
			fmt.Sprintf("🛑 Đã hoàn thành tất cả %d lượt thử. Chưa đăng ký được %d môn: %s", session.Loop, pendingLeft, pendingStr),
			"warning",
		)
	}
}

// tryServerRelogin đăng nhập lại bằng credentials lưu sẵn
func tryServerRelogin(client *http.Client, session *models.AutoRegisterSession) string {
	session.Mu.Lock()
	creds := session.Credentials
	session.Mu.Unlock()

	if creds == nil || creds.Username == "" || creds.Password == "" {
		return ""
	}

	session.AddLog("🔄 [AutoReLogin] Phiên làm việc hết hạn! Máy chủ đang tự động đăng nhập lại cổng trường...", "warning")

	loginBody, _ := json.Marshal(map[string]string{
		"username": creds.Username,
		"password": creds.Password,
	})

	req, err := http.NewRequest("POST", config.PortalBaseURL+"/user/login", bytes.NewReader(loginBody))
	if err != nil {
		session.AddLog(fmt.Sprintf("❌ [AutoReLogin] Lỗi tạo request: %s", err), "error")
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Origin", config.PortalOrigin)
	req.Header.Set("Referer", config.PortalOrigin+"/")

	resp, err := client.Do(req)
	if err != nil {
		session.AddLog(fmt.Sprintf("❌ [AutoReLogin] Đăng nhập lại thất bại: %s", err), "error")
		return ""
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		session.AddLog(fmt.Sprintf("❌ [AutoReLogin] Lỗi parse response: %s", err), "error")
		return ""
	}

	if success, ok := result["success"].(bool); ok && success {
		if token, ok := result["token"].(string); ok && token != "" {
			session.Mu.Lock()
			session.Token = token
			session.Mu.Unlock()
			session.AddLog("✅ [AutoReLogin] Đã tự động cấp lại token mới trên máy chủ!", "success")
			return token
		}
	}

	session.AddLog(fmt.Sprintf("❌ [AutoReLogin] Đăng nhập lại thất bại: %s", string(bodyBytes)), "error")
	return ""
}

// checkRegistrationSuccess kiểm tra response đăng ký có thành công không
func checkRegistrationSuccess(resData map[string]interface{}) bool {
	// Nếu có field success=false rõ ràng → thất bại
	if success, ok := resData["success"]; ok {
		if b, ok := success.(bool); ok {
			return b
		}
	}

	// Kiểm tra các dấu hiệu thất bại trong message
	if msg, ok := resData["message"].(string); ok {
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "thất bại") ||
			strings.Contains(lower, "không thành công") ||
			strings.Contains(lower, "lỗi") {
			return false
		}
	}

	// Kiểm tra isError
	if isErr, ok := resData["isError"]; ok {
		if b, ok := isErr.(bool); ok && b {
			return false
		}
	}

	return true
}

// removeSiblingClasses loại bỏ các lớp cùng nhóm môn đã đăng ký thành công
// Gọi trong session.Mu.Lock()
func removeSiblingClasses(session *models.AutoRegisterSession, classID string) string {
	for _, group := range session.CourseGroups {
		groupIDs := toStringSlice(group.ClassIDs)
		if !contains(groupIDs, classID) {
			continue
		}

		if len(groupIDs) <= 1 {
			continue
		}

		// Xóa tất cả lớp trong nhóm khỏi pending
		otherSiblings := []string{}
		for _, gid := range groupIDs {
			if gid != classID {
				otherSiblings = append(otherSiblings, gid)
			}
		}
		session.PendingClassIDs = removeMultipleFromSlice(session.PendingClassIDs, groupIDs)

		courseName := group.TenHocPhan
		if courseName == "" {
			courseName = group.MaHocPhan
		}
		return fmt.Sprintf("🎉 Môn [%s] đã đăng ký thành công lớp %s! Tự động loại bỏ %d lớp dự phòng còn lại (%s) khỏi hàng đợi.",
			courseName, classID, len(otherSiblings), strings.Join(otherSiblings, ", "))
	}
	return ""
}

// ========== Utility functions ==========

func toStringSlice(arr []interface{}) []string {
	result := make([]string, len(arr))
	for i, v := range arr {
		result[i] = fmt.Sprintf("%v", v)
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func removeMultipleFromSlice(slice []string, items []string) []string {
	itemSet := make(map[string]bool)
	for _, item := range items {
		itemSet[item] = true
	}
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if !itemSet[s] {
			result = append(result, s)
		}
	}
	return result
}

func getStringField(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
