// Package config chứa các hằng số cấu hình cho ứng dụng ĐKHP
package config

import "time"

const (
	// PortalBaseURL là URL gốc của API Portal trường UT
	PortalBaseURL = "https://portal.ut.edu.vn/api/v1"

	// PortalOrigin dùng cho header Origin khi proxy request
	PortalOrigin = "https://portal.ut.edu.vn"

	// RequestTimeout là thời gian tối đa cho mỗi HTTP request proxy
	RequestTimeout = 15 * time.Second

	// DefaultPort là port mặc định khi không có biến môi trường PORT
	DefaultPort = 3000

	// MultiPortCount là số lượng port mở khi chạy Multi-Port mode
	MultiPortCount = 10

	// UserAgent giả lập trình duyệt Chrome cho các request proxy
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)
