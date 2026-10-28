// Package handlers - Registration Handler
// Xử lý đăng ký học phần thủ công và tự động (AutoDKHP Worker)
package handlers

import (
	"bytes"
	"dkhp-backend/config"
	"dkhp-backend/models"
	"dkhp-backend/services"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// PostRegisterMain đăng ký lớp học phần chính
// POST /api/registration/main
func PostRegisterMain(c *gin.Context) {
	proxyRegistration(c, config.PortalBaseURL+"/dkhp/dangKyLopHocPhan")
}

// PostRegisterExtra đăng ký lớp học phần điều kiện
// POST /api/registration/extra
func PostRegisterExtra(c *gin.Context) {
	proxyRegistration(c, config.PortalBaseURL+"/dkhpdk/dangKyLopHocPhan")
}

// PostRegisterImprovement đăng ký lớp cải thiện
// POST /api/registration/improvement
func PostRegisterImprovement(c *gin.Context) {
	proxyRegistration(c, config.PortalBaseURL+"/dkhp/dangKyLopHocPhan")
}

// DeleteCancelRegistration hủy đăng ký
// DELETE /api/registration/cancel/:idDangKy
func DeleteCancelRegistration(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Vui lòng đăng nhập"})
		return
	}

	idDangKy := c.Param("idDangKy")
	if idDangKy == "" || idDangKy == "undefined" || idDangKy == "null" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "ID đăng ký không hợp lệ"})
		return
	}

	url := fmt.Sprintf("%s/dkhp/huyDangKy?idDangKy=%s", config.PortalBaseURL, idDangKy)
	req, _ := http.NewRequest("DELETE", url, nil)
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

// PostAutoStart bắt đầu tiến trình đăng ký tự động trên server
// POST /api/registration/auto/start
func PostAutoStart(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Vui lòng đăng nhập"})
		return
	}

	var reqBody models.AutoRegisterRequest
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Dữ liệu không hợp lệ"})
		return
	}

	if len(reqBody.ClassIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Danh sách ID lớp không hợp lệ"})
		return
	}

	// Tạo sessionID dựa trên MSSV hoặc token
	sessionID := reqBody.MSSV
	if sessionID == "" {
		sessionID = token[:min(16, len(token))]
	}

	// Kiểm tra session đang chạy
	if existing, ok := services.AutoRegisterSessions.Load(sessionID); ok {
		sess := existing.(*models.AutoRegisterSession)
		sess.Mu.Lock()
		status := sess.Status
		sess.Mu.Unlock()
		if status == "running" || status == "waiting" {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Đang có tiến trình đăng ký chạy"})
			return
		}
	}

	// Chuyển classIDs sang string
	classIDStrings := make([]string, len(reqBody.ClassIDs))
	for i, id := range reqBody.ClassIDs {
		classIDStrings[i] = fmt.Sprintf("%v", id)
	}

	// Defaults
	if reqBody.Loop <= 0 {
		reqBody.Loop = 1
	}
	if reqBody.Delay <= 0 {
		reqBody.Delay = 50
	}
	if reqBody.DelayMax <= 0 {
		reqBody.DelayMax = 100
	}
	if reqBody.Type == "" {
		reqBody.Type = "main"
	}

	session := &models.AutoRegisterSession{
		SessionID:          sessionID,
		Token:              token,
		Credentials:        reqBody.Credentials,
		MSSV:               reqBody.MSSV,
		Active:             true,
		Type:               reqBody.Type,
		Loop:               reqBody.Loop,
		Delay:              reqBody.Delay,
		DelayMax:           reqBody.DelayMax,
		ClassIDs:           classIDStrings,
		CourseGroups:       reqBody.CourseGroups,
		PendingClassIDs:    make([]string, len(classIDStrings)),
		SuccessfulClassIDs: []string{},
		Logs:               []models.LogEntry{},
		AllLogs:            []models.LogEntry{},
		ProxyURL:           reqBody.ProxyURL,
	}
	copy(session.PendingClassIDs, classIDStrings)

	if reqBody.TargetTime != "" {
		session.Status = "waiting"
	} else {
		session.Status = "running"
	}

	services.AutoRegisterSessions.Store(sessionID, session)

	if reqBody.TargetTime != "" {
		services.ScheduleAutoRegistration(session, reqBody.TargetTime)
	} else {
		services.StartAutoRegistration(session)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Đã bắt đầu đăng ký tự động trên máy chủ",
	})
}

// GetAutoStatus trả về trạng thái tiến trình đăng ký tự động
// GET /api/registration/auto/status
func GetAutoStatus(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Vui lòng đăng nhập"})
		return
	}

	// Tìm session theo token
	var session *models.AutoRegisterSession
	services.AutoRegisterSessions.Range(func(key, value interface{}) bool {
		sess := value.(*models.AutoRegisterSession)
		sess.Mu.Lock()
		sessToken := sess.Token
		sess.Mu.Unlock()
		if sessToken == token {
			session = sess
			return false
		}
		return true
	})

	if session == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Không tìm thấy tiến trình",
		})
		return
	}

	logs := session.DrainLogs()

	session.Mu.Lock()
	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"status":             session.Status,
		"result":             session.Result,
		"pendingClassIds":    session.PendingClassIDs,
		"successfulClassIds": session.SuccessfulClassIDs,
		"logs":               logs,
	})
	session.Mu.Unlock()
}

// PostAutoStop dừng tiến trình đăng ký tự động
// POST /api/registration/auto/stop
func PostAutoStop(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Vui lòng đăng nhập"})
		return
	}

	var session *models.AutoRegisterSession
	services.AutoRegisterSessions.Range(func(key, value interface{}) bool {
		sess := value.(*models.AutoRegisterSession)
		sess.Mu.Lock()
		if sess.Token == token && (sess.Status == "running" || sess.Status == "waiting") {
			session = sess
			sess.Mu.Unlock()
			return false
		}
		sess.Mu.Unlock()
		return true
	})

	if session != nil {
		session.Mu.Lock()
		session.Active = false
		session.Status = "stopped"
		if session.CancelFunc != nil {
			session.CancelFunc()
		}
		session.Mu.Unlock()
		session.AddLog("🛑 Đã nhận lệnh dừng từ người dùng", "warning")
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã gửi lệnh dừng"})
}

// GetViewLogs hiển thị trang nhật ký ĐKHP (HTML)
// GET /api/registration/view-logs
func GetViewLogs(c *gin.Context) {
	var htmlBuilder strings.Builder

	htmlBuilder.WriteString(`<!DOCTYPE html>
<html lang="vi">
<head>
    <title>Server Registration Logs - UT DKHP</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { box-sizing: border-box; }
        body { font-family: 'Segoe UI', Consolas, Monaco, monospace; background: #121316; color: #e2e8f0; padding: 20px; margin: 0; }
        .container { max-width: 1000px; margin: 0 auto; }
        .header { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #2d3748; padding-bottom: 15px; margin-bottom: 20px; flex-wrap: wrap; gap: 10px; }
        h1 { color: #60a5fa; margin: 0; font-size: 22px; display: flex; align-items: center; gap: 8px; }
        .btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; font-weight: 600; font-size: 13px; text-decoration: none; border-radius: 6px; border: none; cursor: pointer; transition: 0.15s; }
        .btn-refresh { background: #2563eb; color: white; }
        .btn-refresh:hover { background: #1d4ed8; }
        .btn-clear { background: #dc2626; color: white; }
        .btn-clear:hover { background: #b91c1c; }
        .btn-stop { background: #e11d48; color: white; padding: 5px 12px; font-size: 12px; border-radius: 4px; border: none; cursor: pointer; }
        .btn-stop:hover { background: #be123c; }
        .btn-home { background: #374151; color: white; }
        .btn-home:hover { background: #4b5563; }
        .session { background: #1e222b; border: 1px solid #374151; padding: 16px; margin-bottom: 20px; border-radius: 8px; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.2); }
        .session-title { display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #2d3748; padding-bottom: 10px; margin-bottom: 12px; flex-wrap: wrap; gap: 8px; }
        .session-meta { font-size: 15px; font-weight: 600; }
        .status-badge { padding: 3px 8px; border-radius: 4px; font-size: 12px; font-weight: 700; text-transform: uppercase; }
        .status-running { background: #064e3b; color: #34d399; border: 1px solid #059669; }
        .status-waiting { background: #78350f; color: #fbbf24; border: 1px solid #d97706; }
        .status-finished { background: #1e3a8a; color: #93c5fd; border: 1px solid #3b82f6; }
        .status-stopped { background: #4c0519; color: #fb7185; border: 1px solid #e11d48; }
        .summary-box { background: #14171f; border: 1px solid #2d3748; border-radius: 6px; padding: 10px 14px; margin-bottom: 12px; font-size: 13px; line-height: 1.6; }
        .log-box { background: #0b0d11; border: 1px solid #1f242e; border-radius: 6px; padding: 12px; max-height: 380px; overflow-y: auto; font-family: Consolas, monospace; font-size: 12.5px; line-height: 1.5; }
        .log-info { color: #93c5fd; }
        .log-success { color: #4ade80; font-weight: 600; }
        .log-error { color: #f87171; }
        .log-warning { color: #fbbf24; }
        .empty-state { text-align: center; padding: 40px 20px; color: #94a3b8; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>⚡ Nhật ký Đăng ký Học phần (Server Background)</h1>
            <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap;">
                <a href="/" class="btn btn-home">🏠 Về Trang Chủ</a>
                <a href="/api/registration/view-logs" class="btn btn-refresh">🔄 Làm mới (F5)</a>
                <form action="/api/registration/view-logs/clear" method="POST" style="display:inline; margin: 0;">
                    <button type="submit" class="btn btn-clear" onclick="return confirm('Bạn có chắc muốn xóa lịch sử log?')">🗑️ Xóa log</button>
                </form>
            </div>
        </div>`)

	// Check if any sessions exist
	count := 0
	services.AutoRegisterSessions.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if count == 0 {
		htmlBuilder.WriteString(`
        <div class="session empty-state">
            <p style="font-size: 16px; margin-bottom: 8px;">Không có tiến trình đăng ký nào đang lưu trong bộ nhớ máy chủ.</p>
            <p style="font-size: 13px; color: #64748b;">Hãy quay lại trang chủ, nhập ID môn và nhấn "Bắt đầu" để khởi chạy tiến trình ngầm.</p>
        </div>`)
	} else {
		services.AutoRegisterSessions.Range(func(key, value interface{}) bool {
			sessionID := key.(string)
			session := value.(*models.AutoRegisterSession)
			session.Mu.Lock()
			defer session.Mu.Unlock()

			statusClass := "status-" + session.Status

			// Pending list HTML
			pendingHTML := ""
			if len(session.PendingClassIDs) > 0 {
				parts := make([]string, len(session.PendingClassIDs))
				for i, id := range session.PendingClassIDs {
					parts[i] = fmt.Sprintf(`<code style="background:#1e3a8a;color:#bfdbfe;padding:2px 6px;border-radius:3px;">%s</code>`, html.EscapeString(id))
				}
				pendingHTML = strings.Join(parts, " ")
			} else {
				pendingHTML = `<i style="color:#94a3b8;">(Đã hoàn thành toàn bộ)</i>`
			}

			// Success list HTML
			successHTML := ""
			if len(session.SuccessfulClassIDs) > 0 {
				parts := make([]string, len(session.SuccessfulClassIDs))
				for i, id := range session.SuccessfulClassIDs {
					parts[i] = fmt.Sprintf(`<code style="background:#064e3b;color:#86efac;padding:2px 6px;border-radius:3px;">%s</code>`, html.EscapeString(id))
				}
				successHTML = strings.Join(parts, " ")
			} else {
				successHTML = `<i style="color:#94a3b8;">(Chưa có môn nào)</i>`
			}

			stopButton := ""
			if session.Status == "running" || session.Status == "waiting" {
				stopButton = fmt.Sprintf(`<form action="/api/registration/view-logs/stop/%s" method="POST" style="display:inline; margin: 0;">
                    <button type="submit" class="btn-stop">⏹ Dừng Tiến Trình</button>
                </form>`, html.EscapeString(sessionID))
			}

			htmlBuilder.WriteString(fmt.Sprintf(`
        <div class="session">
            <div class="session-title">
                <div class="session-meta">
                    MSSV: <span style="color: #fbbf24; font-weight: bold;">%s</span>
                    <span style="color: #64748b; font-size: 12px; margin-left: 8px;">(Session: %s)</span>
                    <span class="status-badge %s" style="margin-left: 10px;">%s</span>
                </div>
                %s
            </div>
            <div class="summary-box">
                <div>📚 <b>Tổng số môn ban đầu:</b> %d (%s)</div>
                <div style="margin-top: 4px;">⏳ <b>Môn đang chờ đăng ký (%d):</b> %s</div>
                <div style="margin-top: 4px;">✅ <b>Môn đã đăng ký thành công (%d):</b> %s</div>
            </div>
            <div class="log-box">`,
				html.EscapeString(session.MSSV),
				html.EscapeString(sessionID),
				statusClass,
				html.EscapeString(session.Status),
				stopButton,
				len(session.ClassIDs),
				html.EscapeString(strings.Join(session.ClassIDs, ", ")),
				len(session.PendingClassIDs),
				pendingHTML,
				len(session.SuccessfulClassIDs),
				successHTML,
			))

			// Logs
			if len(session.AllLogs) > 0 {
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
			} else {
				htmlBuilder.WriteString(`<div style="color: #64748b;">(Chưa có log)</div>`)
			}

			htmlBuilder.WriteString(`
            </div>
        </div>`)
			return true
		})
	}

	htmlBuilder.WriteString(`
        <div style="text-align: center; font-size: 12px; color: #64748b; margin-top: 20px;">
            Trang tự động làm mới mỗi 3 giây • <span id="clock"></span>
        </div>
    </div>
    <script>
        function updateClock() {
            const now = new Date();
            document.getElementById('clock').textContent = now.toLocaleTimeString('vi-VN');
        }
        updateClock();
        setInterval(updateClock, 1000);
        setTimeout(() => location.reload(), 3000);
    </script>
</body>
</html>`)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlBuilder.String()))
}

// PostViewLogsClear xóa log
// POST /api/registration/view-logs/clear
func PostViewLogsClear(c *gin.Context) {
	services.AutoRegisterSessions.Range(func(key, value interface{}) bool {
		sess := value.(*models.AutoRegisterSession)
		sess.Mu.Lock()
		sess.AllLogs = sess.AllLogs[:0]
		sess.Logs = sess.Logs[:0]
		status := sess.Status
		sess.Mu.Unlock()
		if status != "running" && status != "waiting" {
			services.AutoRegisterSessions.Delete(key)
		}
		return true
	})
	c.Redirect(http.StatusFound, "/api/registration/view-logs")
}

// PostViewLogsStop dừng session từ trang view logs
// POST /api/registration/view-logs/stop/:sessionId
func PostViewLogsStop(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if val, ok := services.AutoRegisterSessions.Load(sessionID); ok {
		sess := val.(*models.AutoRegisterSession)
		sess.Mu.Lock()
		sess.Active = false
		sess.Status = "stopped"
		if sess.CancelFunc != nil {
			sess.CancelFunc()
		}
		sess.Mu.Unlock()
		sess.AddLog("🛑 Đã nhận lệnh dừng từ trang View Logs", "warning")
	}
	c.Redirect(http.StatusFound, "/api/registration/view-logs")
}

// ========== Helpers ==========

func proxyRegistration(c *gin.Context, baseURL string) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Vui lòng đăng nhập"})
		return
	}

	var body map[string]interface{}
	c.ShouldBindJSON(&body)

	classID := ""
	if id, ok := body["classId"]; ok {
		classID = fmt.Sprintf("%v", id)
	}

	url := fmt.Sprintf("%s?idLopHocPhan=%s", baseURL, classID)
	req, _ := http.NewRequest("POST", url, bytes.NewReader([]byte("{}")))
	setPortalHeaders(req, token)

	resp, err := services.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Lỗi kết nối"})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json; charset=utf-8", respBody)
}

