// ĐKHP Web Application - Go Backend Server
// High-concurrency course registration tool
//
// Thay thế Node.js Express bằng Go + Gin để tận dụng:
// - Goroutines thay vì Single-Thread Event Loop
// - Connection Pooling tối ưu với net/http
// - sync.Map cho trạng thái in-memory thread-safe
// - Multi-port mode chạy 10 server song song trong cùng process
package main

import (
	"context"
	"dkhp-backend/config"
	"dkhp-backend/handlers"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	// Kiểm tra biến môi trường PORT (deploy mode)
	portEnv := os.Getenv("PORT")

	if portEnv != "" {
		// ===== Single port mode (VPS/Docker/Cloud) =====
		port, err := strconv.Atoi(portEnv)
		if err != nil {
			log.Fatalf("PORT không hợp lệ: %s", portEnv)
		}
		runSingleServer(port)
	} else {
		// ===== Multi-port mode (Offline / .exe) =====
		fmt.Println("==================================================")
		fmt.Println("🚀 Khởi động ĐKHP Tool - Multi-Port (3000-3009)...")
		fmt.Println("==================================================")
		fmt.Printf("Đang mở %d luồng server độc lập, vui lòng chờ...\n\n", config.MultiPortCount)
		runMultiPortServers()
	}
}

// setupRouter cấu hình Gin router với tất cả các route
func setupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// Logger tùy chỉnh
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s\n",
			param.TimeStamp.Format("15:04:05"),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
		)
	}))

	// CORS middleware
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// ===== Static Files =====
	// Phục vụ thư mục /public từ thư mục cha (nơi chứa Node.js project)
	publicDir := findPublicDir()
	r.Static("/css", filepath.Join(publicDir, "css"))
	r.Static("/js", filepath.Join(publicDir, "js"))
	r.Static("/images", filepath.Join(publicDir, "images"))

	// ===== API Routes =====

	// Auth routes (/api/user/*)
	userGroup := r.Group("/api/user")
	{
		userGroup.POST("/login", handlers.PostLogin)
		userGroup.GET("/getSummaryProfile", handlers.GetSummaryProfile)
		userGroup.GET("/validate", handlers.GetValidate)
		userGroup.POST("/logout", handlers.PostLogout)
		userGroup.GET("/me", handlers.GetCurrentUser)
	}

	// Events routes (/api/events/*)
	eventsGroup := r.Group("/api/events")
	{
		eventsGroup.POST("/auto/start", handlers.PostAutoEventStart)
		eventsGroup.POST("/auto/stop", handlers.PostAutoEventStop)
		eventsGroup.GET("/auto/status", handlers.GetAutoEventStatus)
		eventsGroup.GET("/view-logs", handlers.GetEventViewLogs)
		eventsGroup.POST("/view-logs/clear", handlers.PostEventViewLogsClear)
		eventsGroup.POST("/view-logs/stop/:sessionId", handlers.PostEventViewLogsStop)
	}

	// Registration routes (/api/registration/*)
	regGroup := r.Group("/api/registration")
	{
		regGroup.POST("/main", handlers.PostRegisterMain)
		regGroup.POST("/extra", handlers.PostRegisterExtra)
		regGroup.POST("/improvement", handlers.PostRegisterImprovement)
		regGroup.DELETE("/cancel/:idDangKy", handlers.DeleteCancelRegistration)
		regGroup.POST("/auto/start", handlers.PostAutoStart)
		regGroup.GET("/auto/status", handlers.GetAutoStatus)
		regGroup.POST("/auto/stop", handlers.PostAutoStop)
		regGroup.GET("/view-logs", handlers.GetViewLogs)
		regGroup.POST("/view-logs/clear", handlers.PostViewLogsClear)
		regGroup.POST("/view-logs/stop/:sessionId", handlers.PostViewLogsStop)
	}

	// Gateway route (obfuscated proxy)
	r.POST("/api/gateway", handlers.GatewayHandler)

	// Proxy routes (/api/dkhp/*, /api/dkhpdk/*, /api/eventWeb/*)
	r.Any("/api/dkhp/*path", handlers.ProxyHandler)
	r.Any("/api/dkhpdk/*path", handlers.ProxyHandler)
	r.Any("/api/eventWeb/*path", handlers.ProxyHandler)

	// ===== Redirect routes =====
	r.GET("/event", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api/events/view-logs")
	})
	r.GET("/logs", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api/registration/view-logs")
	})
	r.GET("/dkhp-logs", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api/registration/view-logs")
	})

	// SPA fallback: serve index.html for all other routes
	r.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == "/admin.html" || c.Request.URL.Path == "/api/keys" || strings.HasPrefix(c.Request.URL.Path, "/api/keys/") {
			c.Status(http.StatusNotFound)
			return
		}
		c.File(filepath.Join(publicDir, "index.html"))
	})

	return r
}

// findPublicDir tìm thư mục public (hỗ trợ cả dev và build)
func findPublicDir() string {
	// Ưu tiên 1: thư mục public cùng cấp với go-backend
	parentPublic := filepath.Join("..", "public")
	if info, err := os.Stat(parentPublic); err == nil && info.IsDir() {
		abs, _ := filepath.Abs(parentPublic)
		return abs
	}

	// Ưu tiên 2: thư mục public cùng thư mục với executable
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	exePublic := filepath.Join(exeDir, "public")
	if info, err := os.Stat(exePublic); err == nil && info.IsDir() {
		return exePublic
	}

	// Ưu tiên 3: thư mục public tuyệt đối
	cwd, _ := os.Getwd()
	cwdPublic := filepath.Join(cwd, "public")
	if info, err := os.Stat(cwdPublic); err == nil && info.IsDir() {
		return cwdPublic
	}

	// Fallback: parent dir
	log.Println("⚠️ Không tìm thấy thư mục /public. Sử dụng fallback:", parentPublic)
	abs, _ := filepath.Abs(parentPublic)
	return abs
}

// runSingleServer chạy 1 server trên port chỉ định
func runSingleServer(port int) {
	router := setupRouter()
	addr := fmt.Sprintf("0.0.0.0:%d", port)

	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("🛑 Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal("Server forced to shutdown:", err)
		}
	}()

	log.Printf("✅ [Luồng %d] Sẵn sàng tại: http://localhost:%d", port, port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Lỗi khởi động server port %d: %v", port, err)
	}
}

// runMultiPortServers chạy 10 server song song trên các port 3000-3009
func runMultiPortServers() {
	var wg sync.WaitGroup
	servers := make([]*http.Server, config.MultiPortCount)

	for i := 0; i < config.MultiPortCount; i++ {
		port := config.DefaultPort + i
		router := setupRouter()
		addr := fmt.Sprintf("0.0.0.0:%d", port)

		srv := &http.Server{
			Addr:    addr,
			Handler: router,
		}
		servers[i] = srv

		wg.Add(1)
		go func(p int, s *http.Server) {
			defer wg.Done()
			log.Printf("✅ [Luồng %d] Sẵn sàng tại: http://localhost:%d", p, p)
			if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("❌ Lỗi ở port %d: %v", p, err)
			}
		}(port, srv)
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n🛑 Đang đóng tất cả các server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("⚠️ Lỗi shutdown: %v", err)
		}
	}
	wg.Wait()
	log.Println("✅ Đã đóng tất cả thành công.")
}
