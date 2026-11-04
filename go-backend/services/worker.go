// Package services - Auto DKHP Worker Engine
package services

import (
	"context"
	"dkhp-backend/models"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var AutoRegisterSessions sync.Map // map[string]*models.AutoRegisterSession

func registrationClient(session *models.AutoRegisterSession) *http.Client {
	if session.ProxyURL != "" {
		client, err := NewHTTPClientWithProxy(session.ProxyURL)
		if err == nil {
			session.AddLog("🔒 Đang dùng Proxy riêng cho phiên đăng ký.", "info")
			return client
		}
		session.AddLog("❌ Cấu hình Proxy không hợp lệ; dùng kết nối trực tiếp.", "error")
	}
	return DefaultClient
}

func StartAutoRegistration(session *models.AutoRegisterSession) {
	ctx := session.RunContext()
	client := registrationClient(session)
	go runAutoRegistrationWorker(ctx, client, session)
}

func ScheduleAutoRegistration(session *models.AutoRegisterSession, targetTime string) {
	ctx := session.RunContext()
	client := registrationClient(session)
	go func() {
		offset := SyncTimeOffsetContext(ctx, client)
		wait, err := CalculateWaitDuration(targetTime, offset)
		if err != nil {
			session.AddLog("❌ Giờ hẹn không hợp lệ.", "error")
			session.Finish("finished", nil)
			return
		}
		session.AddLog(fmt.Sprintf("⏳ Đang chờ giờ hẹn (còn %.1fs; đã bù lệch giờ).", wait.Seconds()), "info")
		if !WaitContext(ctx, wait) {
			return
		}
		session.Mu.Lock()
		if session.Active {
			session.Status = "running"
		}
		session.Mu.Unlock()
		runAutoRegistrationWorker(ctx, client, session)
	}()
}

func runAutoRegistrationWorker(ctx context.Context, client *http.Client, session *models.AutoRegisterSession) {
	defer session.Finish("finished", nil)
	portal := newPortalSession(client, session.Token, session.Credentials, func(token string) {
		session.Mu.Lock()
		session.Token = token
		session.Mu.Unlock()
	}, session.AddLog)
	base := "/dkhp/dangKyLopHocPhan"
	if session.Type == "extra" {
		base = "/dkhpdk/dangKyLopHocPhan"
	}
	session.AddLog("🚀 Bắt đầu ĐKHP ngầm. Lỗi mạng/Portal không trừ lượt; tự đăng nhập lại khi hết phiên.", "info")
	for iteration := 0; iteration < session.Loop && ctx.Err() == nil; iteration++ {
		session.Mu.Lock()
		batch := append([]string(nil), session.PendingClassIDs...)
		session.Mu.Unlock()
		if len(batch) == 0 {
			return
		}
		session.AddLog(fmt.Sprintf("===== Lượt thử %d/%d (còn %d lớp) =====", iteration+1, session.Loop, len(batch)), "info")
		for _, classID := range batch {
			if ctx.Err() != nil {
				return
			}
			session.Mu.Lock()
			pending := contains(session.PendingClassIDs, classID)
			session.Mu.Unlock()
			if !pending {
				continue
			}
			session.AddLog(fmt.Sprintf("🔄 Đang đăng ký lớp %s (mỗi yêu cầu tối đa 15 giây)...", classID), "info")
			response, err := portal.Do(ctx, http.MethodPost, base+"?idLopHocPhan="+url.QueryEscape(classID), nil)
			if err != nil {
				if errors.Is(err, ErrAuthenticationRequired) {
					session.AddLog("❌ "+err.Error(), "error")
					session.Finish("auth_required", map[string]interface{}{"message": err.Error()})
				}
				return
			}
			message := getStringField(response.Data, "message", "Portal chưa xác nhận thành công")
			if !response.Success() {
				session.AddLog(fmt.Sprintf("❌ Lớp %s: %s", classID, message), "error")
				continue
			}
			session.Mu.Lock()
			if !session.Active {
				session.Mu.Unlock()
				return
			}
			if !contains(session.SuccessfulClassIDs, classID) {
				session.SuccessfulClassIDs = append(session.SuccessfulClassIDs, classID)
			}
			removed := removeSiblingClasses(session, classID)
			session.PendingClassIDs = removeFromSlice(session.PendingClassIDs, classID)
			finished := len(session.PendingClassIDs) == 0
			successful := append([]string(nil), session.SuccessfulClassIDs...)
			session.Mu.Unlock()
			session.AddLog(fmt.Sprintf("✅ THÀNH CÔNG lớp %s: %s", classID, message), "success")
			if removed != "" {
				session.AddLog(removed, "success")
			}
			if finished {
				session.AddLog("🎉 ĐÃ ĐĂNG KÝ THÀNH CÔNG TẤT CẢ CÁC LỚP!", "success")
				session.Finish("finished", map[string]interface{}{"successfulClassIds": successful})
				return
			}
		}
		if iteration+1 < session.Loop {
			minimum := max(session.Delay, 1)
			maximum := max(session.DelayMax, minimum)
			delay := time.Duration(minimum+rand.Intn(maximum-minimum+1)) * time.Millisecond
			if !WaitContext(ctx, delay) {
				return
			}
		}
	}
	if ctx.Err() == nil {
		session.AddLog("🛑 Đã hoàn thành tất cả lượt thử nghiệp vụ.", "warning")
	}
}

// Must be called with session.Mu held.
func removeSiblingClasses(session *models.AutoRegisterSession, classID string) string {
	for _, group := range session.CourseGroups {
		ids := toStringSlice(group.ClassIDs)
		if len(ids) <= 1 || !contains(ids, classID) {
			continue
		}
		session.PendingClassIDs = removeMultipleFromSlice(session.PendingClassIDs, ids)
		name := group.TenHocPhan
		if name == "" {
			name = group.MaHocPhan
		}
		return fmt.Sprintf("🎉 Môn [%s] đã vào lớp %s; loại %d lớp dự phòng khỏi hàng đợi.", name, classID, len(ids)-1)
	}
	return ""
}

func toStringSlice(values []interface{}) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = fmt.Sprint(value)
	}
	return result
}

func contains(values []string, item string) bool {
	for _, value := range values {
		if value == item {
			return true
		}
	}
	return false
}

func removeFromSlice(values []string, item string) []string {
	return removeMultipleFromSlice(values, []string{item})
}

func removeMultipleFromSlice(values, items []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !contains(items, value) {
			result = append(result, value)
		}
	}
	return result
}

func getStringField(data map[string]interface{}, key, fallback string) string {
	if value, ok := data[key].(string); ok && value != "" {
		return value
	}
	return fallback
}
