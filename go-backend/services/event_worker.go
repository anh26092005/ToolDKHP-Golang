// Auto event workers recover from Portal outages without spending the business retry budget.
package services

import (
	"context"
	"dkhp-backend/models"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

var AutoEventSessions sync.Map // map[string]*models.AutoEventSession

func RunAutoEvent(token string, request models.AutoEventRequest, session *models.AutoEventSession) {
	ctx := session.RunContext()
	go runAutoEventWorker(ctx, DefaultClient, token, request, session)
}

func ScheduleAutoEvent(token string, request models.AutoEventRequest, session *models.AutoEventSession) {
	ctx := session.RunContext()
	go func() {
		offset := SyncTimeOffsetContext(ctx, DefaultClient)
		wait, err := CalculateWaitDuration(request.TargetTime, offset)
		if err != nil {
			session.AddLog("❌ Giờ hẹn không hợp lệ.", "error")
			session.Finish("finished", nil)
			return
		}
		session.AddLog(fmt.Sprintf("⏳ Đang chờ giờ hẹn (còn %.1fs).", wait.Seconds()), "info")
		if !WaitContext(ctx, wait) {
			return
		}
		session.Mu.Lock()
		if session.Active {
			session.Status = "running"
		}
		session.Mu.Unlock()
		runAutoEventWorker(ctx, DefaultClient, token, request, session)
	}()
}

func runAutoEventWorker(ctx context.Context, client *http.Client, token string, request models.AutoEventRequest, session *models.AutoEventSession) {
	portal := newPortalSession(client, token, request.Credentials, func(token string) {
		session.Mu.Lock()
		session.Token = token
		session.Mu.Unlock()
	}, session.AddLog)
	runEventWithPortal(ctx, portal, request, session)
}

func runEventWithPortal(ctx context.Context, portal *portalSession, request models.AutoEventRequest, session *models.AutoEventSession) {
	defer session.Finish("finished", nil)
	endpoint := fmt.Sprintf("/eventWeb/register?eventId=%d", request.EventID)
	payload := map[string]interface{}{}
	if request.DateID != nil {
		payload["dateId"] = *request.DateID
	}
	if request.ObjectID != nil {
		payload["objectId"] = *request.ObjectID
	}
	session.AddLog(fmt.Sprintf("🚀 Bắt đầu Auto Sự kiện: ID %d, tối đa %d lượt phản hồi nghiệp vụ.", request.EventID, request.Retries), "info")
	for attempt := 0; attempt < request.Retries && ctx.Err() == nil; attempt++ {
		session.AddLog(fmt.Sprintf("🔄 Lượt %d/%d (lỗi mạng/Portal không trừ lượt).", attempt+1, request.Retries), "info")
		response, err := portal.Do(ctx, http.MethodPost, endpoint, payload)
		if err != nil {
			if errors.Is(err, ErrAuthenticationRequired) {
				session.AddLog("❌ "+err.Error(), "error")
				session.Finish("auth_required", map[string]interface{}{"message": err.Error()})
			}
			return
		}
		message := getStringField(response.Data, "message", "Portal chưa xác nhận thành công")
		if response.Success() {
			session.AddLog("✅ THÀNH CÔNG: "+message, "success")
			session.Finish("success", map[string]interface{}{"eventId": request.EventID, "message": message})
			return
		}
		session.AddLog("❌ Thất bại: "+message, "error")
		if attempt+1 < request.Retries {
			minimum := max(request.DelayMin, 1)
			maximum := max(request.DelayMax, minimum)
			delay := time.Duration(minimum+rand.Intn(maximum-minimum+1)) * time.Millisecond
			if !portal.wait(ctx, delay) {
				return
			}
		}
	}
	if ctx.Err() == nil {
		session.AddLog("🛑 Đã hoàn thành tất cả lượt thử nghiệp vụ.", "warning")
	}
}
