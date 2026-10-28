// Package handlers - Events Handler
// Xử lý auto đăng ký sự kiện chạy nền
package handlers

import (
	"dkhp-backend/models"
	"dkhp-backend/services"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// PostAutoEventStart bắt đầu tiến trình auto sự kiện
// POST /api/events/auto/start
func PostAutoEventStart(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Vui lòng đăng nhập"})
		return
	}

	var reqBody models.AutoEventRequest
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Dữ liệu không hợp lệ"})
		return
	}

	sessionID := token[:min(16, len(token))]

	// Kiểm tra session đang chạy
	if existing, ok := services.AutoEventSessions.Load(sessionID); ok {
		sess := existing.(*models.AutoEventSession)
		sess.Mu.Lock()
		status := sess.Status
		sess.Mu.Unlock()
		if status == "running" || status == "waiting" {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Đang có tiến trình auto sự kiện chạy ngầm"})
			return
		}
	}

	// Defaults
	if reqBody.Retries <= 0 {
		reqBody.Retries = 10
	}
	if reqBody.DelayMin <= 0 {
		reqBody.DelayMin = 50
	}
	if reqBody.DelayMax <= 0 {
		reqBody.DelayMax = 100
	}

	session := &models.AutoEventSession{
		Active:  true,
		Logs:    []models.LogEntry{},
		AllLogs: []models.LogEntry{},
	}

	if reqBody.TargetTime != "" {
		session.Status = "waiting"
	} else {
		session.Status = "running"
	}

	services.AutoEventSessions.Store(sessionID, session)

	if reqBody.TargetTime != "" {
		services.ScheduleAutoEvent(token, reqBody.EventID, reqBody.DateID, reqBody.Retries, reqBody.DelayMin, reqBody.DelayMax, reqBody.TargetTime, session)
	} else {
		services.RunAutoEvent(token, reqBody.EventID, reqBody.DateID, reqBody.Retries, reqBody.DelayMin, reqBody.DelayMax, session)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã bắt đầu tiến trình auto sự kiện ngầm"})
}

// PostAutoEventStop dừng tiến trình auto sự kiện
// POST /api/events/auto/stop
func PostAutoEventStop(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Vui lòng đăng nhập"})
		return
	}

	sessionID := token[:min(16, len(token))]
	if val, ok := services.AutoEventSessions.Load(sessionID); ok {
		sess := val.(*models.AutoEventSession)
		sess.Mu.Lock()
		sess.Active = false
		sess.Status = "stopped"
		if sess.CancelFunc != nil {
			sess.CancelFunc()
		}
		sess.Mu.Unlock()
		sess.AddLog("🛑 Đã nhận lệnh dừng từ người dùng", "warning")
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã gửi lệnh dừng"})
}

// GetAutoEventStatus trả về trạng thái tiến trình sự kiện
// GET /api/events/auto/status
func GetAutoEventStatus(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Vui lòng đăng nhập"})
		return
	}

	sessionID := token[:min(16, len(token))]
	val, ok := services.AutoEventSessions.Load(sessionID)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"status":  "idle",
			"logs":    []models.LogEntry{},
		})
		return
	}

	session := val.(*models.AutoEventSession)
	logs := session.DrainLogs()

	session.Mu.Lock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  session.Status,
		"result":  session.Result,
		"logs":    logs,
	})
	session.Mu.Unlock()
}

// GetEventViewLogs hiển thị trang nhật ký sự kiện (HTML)
// GET /api/events/view-logs
func GetEventViewLogs(c *gin.Context) {
	var htmlBuilder strings.Builder

	htmlBuilder.WriteString(`<html>
<head>
    <title>Server Event Logs</title>
    <meta charset="utf-8">
    <style>
        body { font-family: monospace; background: #1e1e1e; color: #d4d4d4; padding: 20px; }
        h1 { color: #569cd6; }
        .session { border: 1px solid #333; padding: 10px; margin-bottom: 20px; border-radius: 5px; }
        .status-running { color: #4ec9b0; }
        .status-stopped { color: #f44747; }
        .log-info { color: #9cdcfe; }
        .log-success { color: #608b4e; }
        .log-error { color: #f44747; }
        .log-warning { color: #d7ba7d; }
        .btn-stop { background: #f44747; color: white; border: none; padding: 5px 10px; border-radius: 3px; cursor: pointer; float: right; }
        .btn-stop:hover { background: #d13a3a; }
        .refresh { margin-bottom: 20px; padding: 10px; background: #0e639c; color: white; text-decoration: none; border-radius: 4px; display: inline-block; }
    </style>
</head>
<body>
    <h1>Nhật ký Auto Sự kiện chạy ngầm</h1>
    <div style="margin-bottom: 20px;">
        <a href="/api/events/view-logs" class="refresh" style="margin-bottom: 0;">Làm mới (F5)</a>
        <form action="/api/events/view-logs/clear" method="POST" style="display:inline; margin-left: 10px;">
            <button type="submit" class="refresh" style="background: #f44747; cursor: pointer; border: none; font-family: inherit; font-size: inherit; margin-bottom: 0;">Xóa log</button>
        </form>
    </div>`)

	count := 0
	services.AutoEventSessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if count == 0 {
		htmlBuilder.WriteString(`<p>Không có tiến trình nào đang lưu trong bộ nhớ.</p>`)
	} else {
		services.AutoEventSessions.Range(func(key, value interface{}) bool {
			sessionID := key.(string)
			session := value.(*models.AutoEventSession)
			session.Mu.Lock()
			defer session.Mu.Unlock()

			stopBtn := ""
			if session.Status == "running" || session.Status == "waiting" {
				stopBtn = fmt.Sprintf(`<form action="/api/events/view-logs/stop/%s" method="POST" style="display:inline;">
                    <button type="submit" class="btn-stop">Dừng Tiến Trình</button>
                </form>`, html.EscapeString(sessionID))
			}

			htmlBuilder.WriteString(fmt.Sprintf(`<div class="session">
                <h3>
                    Session ID: %s - Trạng thái: <span class="status-%s">%s</span>
                    %s
                </h3>
                <div>`,
				html.EscapeString(sessionID),
				html.EscapeString(session.Status),
				html.EscapeString(session.Status),
				stopBtn,
			))

			for _, log := range session.AllLogs {
				t, _ := time.Parse(time.RFC3339Nano, log.Time)
				loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
				timeStr := t.In(loc).Format("15:04:05")
				htmlBuilder.WriteString(fmt.Sprintf(`<div class="log-%s">[%s] %s</div>`,
					html.EscapeString(log.Type),
					timeStr,
					html.EscapeString(log.Message),
				))
			}

			htmlBuilder.WriteString(`</div></div>`)
			return true
		})
	}

	htmlBuilder.WriteString(`
    <script>
        setTimeout(() => location.reload(), 3000);
    </script>
</body></html>`)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlBuilder.String()))
}

// PostEventViewLogsClear xóa log sự kiện
// POST /api/events/view-logs/clear
func PostEventViewLogsClear(c *gin.Context) {
	services.AutoEventSessions.Range(func(key, value interface{}) bool {
		sess := value.(*models.AutoEventSession)
		sess.Mu.Lock()
		sess.AllLogs = sess.AllLogs[:0]
		sess.Logs = sess.Logs[:0]
		status := sess.Status
		sess.Mu.Unlock()
		if status != "running" && status != "waiting" {
			services.AutoEventSessions.Delete(key)
		}
		return true
	})
	c.Redirect(http.StatusFound, "/api/events/view-logs")
}

// PostEventViewLogsStop dừng session sự kiện từ trang view logs
// POST /api/events/view-logs/stop/:sessionId
func PostEventViewLogsStop(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if val, ok := services.AutoEventSessions.Load(sessionID); ok {
		sess := val.(*models.AutoEventSession)
		sess.Mu.Lock()
		sess.Active = false
		sess.Status = "stopped"
		if sess.CancelFunc != nil {
			sess.CancelFunc()
		}
		sess.Mu.Unlock()
		sess.AddLog("🛑 Đã nhận lệnh dừng từ trang View Logs", "warning")
	}
	c.Redirect(http.StatusFound, "/api/events/view-logs")
}
