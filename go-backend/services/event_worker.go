// Package services - Auto Event Worker
// Worker chạy nền cho chức năng auto đăng ký sự kiện
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

// AutoEventSessions lưu trữ tất cả phiên auto sự kiện (thread-safe)
var AutoEventSessions sync.Map // map[string]*models.AutoEventSession

// RunAutoEvent chạy worker sự kiện trong goroutine
func RunAutoEvent(token string, eventID, dateID interface{}, retries, delayMin, delayMax int, session *models.AutoEventSession) {
	go runAutoEventWorker(DefaultClient, token, eventID, dateID, retries, delayMin, delayMax, session)
}

// ScheduleAutoEvent hẹn giờ rồi chạy event worker
func ScheduleAutoEvent(token string, eventID, dateID interface{}, retries, delayMin, delayMax int, targetTimeStr string, session *models.AutoEventSession) {
	client := DefaultClient

	offset := SyncTimeOffset(client)
	session.AddLog(fmt.Sprintf("⏳ Máy bạn lệch %ds so với giờ chuẩn. Hệ thống đã tự động bù giờ!", int(offset.Seconds())), "warning")

	waitDuration, err := CalculateWaitDuration(targetTimeStr, offset)
	if err != nil {
		session.AddLog(fmt.Sprintf("❌ Lỗi parse thời gian hẹn: %s", err), "error")
		session.Status = "finished"
		return
	}

	session.AddLog(fmt.Sprintf("⏳ Đang đợi đến %s (Giờ chuẩn) để bắt đầu...", targetTimeStr), "info")

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
				runAutoEventWorker(client, token, eventID, dateID, retries, delayMin, delayMax, session)
			} else {
				session.Mu.Unlock()
			}
		case <-ctx.Done():
			log.Println("[AutoEvent] Timer đã bị hủy")
		}
	}()
}

func runAutoEventWorker(client *http.Client, token string, eventID, dateID interface{}, retries, delayMin, delayMax int, session *models.AutoEventSession) {
	registerURL := fmt.Sprintf("%s/eventWeb/register?eventId=%v", config.PortalBaseURL, eventID)

	dateStr := ""
	if dateID != nil {
		dateStr = fmt.Sprintf("/%v", dateID)
	}

	session.AddLog(fmt.Sprintf("🚀 Bắt đầu Auto Sự kiện ngầm: ID %v%s (%d lượt)", eventID, dateStr, retries), "info")

	for i := 0; i < retries; i++ {
		if !session.Active {
			session.Mu.Lock()
			session.Status = "stopped"
			session.Mu.Unlock()
			return
		}

		session.AddLog(fmt.Sprintf("🔄 Lượt %d/%d...", i+1, retries), "info")

		// Build request body
		var bodyReader *bytes.Reader
		if dateID != nil {
			payload, _ := json.Marshal(map[string]interface{}{"dateId": dateID})
			bodyReader = bytes.NewReader(payload)
		} else {
			bodyReader = bytes.NewReader(nil)
		}

		req, err := http.NewRequest("POST", registerURL, bodyReader)
		if err != nil {
			session.AddLog(fmt.Sprintf("❌ Lỗi tạo request: %s", err), "error")
			continue
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", config.UserAgent)
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Origin", config.PortalOrigin)
		req.Header.Set("Referer", config.PortalOrigin+"/")
		if dateID != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			session.AddLog(fmt.Sprintf("❌ Lỗi kết nối: %s", err), "error")
		} else {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var resData map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &resData); err == nil {
				isSuccess := true
				if s, ok := resData["success"]; ok {
					if b, ok := s.(bool); ok && !b {
						isSuccess = false
					}
				}
				message := getStringField(resData, "message", "Đăng ký thành công")
				lower := strings.ToLower(message)
				if strings.Contains(lower, "không thành công") || strings.Contains(lower, "lỗi") {
					isSuccess = false
				}

				if isSuccess {
					session.AddLog(fmt.Sprintf("✅ THÀNH CÔNG: %s", message), "success")
					session.Mu.Lock()
					session.Status = "success"
					session.Result = map[string]interface{}{"eventId": eventID, "message": message}
					session.Mu.Unlock()
					return
				}
				session.AddLog(fmt.Sprintf("❌ Thất bại: %s", message), "error")
			} else {
				session.AddLog(fmt.Sprintf("❌ Lỗi kết nối: %s", string(bodyBytes)), "error")
			}
		}

		// Delay trước lượt tiếp theo
		if i < retries-1 && session.Active {
			if delayMax <= delayMin {
				delayMax = delayMin + 1
			}
			delay := delayMin + rand.Intn(delayMax-delayMin+1)
			session.AddLog(fmt.Sprintf("⏳ Đợi %dms...", delay), "warning")
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	if session.Active {
		session.Mu.Lock()
		session.Status = "finished"
		session.Mu.Unlock()
		session.AddLog("🛑 Đã hoàn thành tất cả lượt thử ngầm", "warning")
	}
}
