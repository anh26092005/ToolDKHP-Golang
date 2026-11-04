package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

func sendPortalConnectionError(c *gin.Context, err error, gateway bool) {
	status := http.StatusBadGateway
	message := "Không kết nối được Portal. Tiến trình tự động sẽ thử lại sau ít giây."
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		status = http.StatusGatewayTimeout
		message = "Portal chưa phản hồi trong thời gian cho phép (timeout). Tiến trình tự động sẽ thử lại."
	}
	data := gin.H{"success": false, "message": message, "isNetworkError": true, "isServerError": true, "statusCode": status}
	if gateway {
		encoded, _ := json.Marshal(data)
		c.JSON(status, gin.H{"r": base64.StdEncoding.EncodeToString(encoded)})
		return
	}
	c.JSON(status, data)
}
