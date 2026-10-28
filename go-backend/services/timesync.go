// Package services - Time Sync Service
// Đồng bộ thời gian với header Date của Portal trường để hẹn giờ chính xác
package services

import (
	"dkhp-backend/config"
	"fmt"
	"log"
	"net/http"
	"time"
)

// SyncTimeOffset tính toán độ lệch (ms) giữa giờ hệ thống và giờ server trường.
// Trả về offset = serverTime - localTime (đơn vị: time.Duration)
// Nếu không lấy được thời gian từ server, trả về 0.
func SyncTimeOffset(client *http.Client) time.Duration {
	start := time.Now()

	req, err := http.NewRequest("GET", "https://portal.ut.edu.vn", nil)
	if err != nil {
		log.Println("[TimeSync] Không thể tạo request:", err)
		return 0
	}
	req.Header.Set("User-Agent", config.UserAgent)

	// Timeout riêng cho sync time (3 giây)
	syncClient := &http.Client{
		Transport: client.Transport,
		Timeout:   3 * time.Second,
	}

	resp, err := syncClient.Do(req)
	if err != nil {
		log.Println("[TimeSync] Không thể kết nối Portal:", err)
		return 0
	}
	defer resp.Body.Close()

	end := time.Now()
	rtt := end.Sub(start)

	dateHeader := resp.Header.Get("Date")
	if dateHeader == "" {
		log.Println("[TimeSync] Không tìm thấy header Date trong response")
		return 0
	}

	serverTime, err := http.ParseTime(dateHeader)
	if err != nil {
		log.Println("[TimeSync] Không parse được header Date:", err)
		return 0
	}

	// Ước tính thời gian local tại thời điểm server trả về = start + RTT/2
	localEstimate := start.Add(rtt / 2)
	offset := serverTime.Sub(localEstimate)

	log.Printf("[TimeSync] Độ lệch giờ hệ thống so với UT Portal: %dms (RTT: %dms)", offset.Milliseconds(), rtt.Milliseconds())
	return offset
}

// CalculateWaitDuration tính thời gian chờ đến targetTime, có bù offset.
// Trả về thời gian cần chờ (>= 0).
func CalculateWaitDuration(targetTimeStr string, offset time.Duration) (time.Duration, error) {
	// Hỗ trợ nhiều format phổ biến
	var targetTime time.Time
	var err error
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		targetTime, err = time.Parse(f, targetTimeStr)
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0, fmt.Errorf("không parse được targetTime '%s': %w", targetTimeStr, err)
	}

	// Giờ hiện tại đã bù offset
	adjustedNow := time.Now().Add(offset)
	wait := targetTime.Sub(adjustedNow)
	if wait < 0 {
		wait = 0
	}
	return wait, nil
}
