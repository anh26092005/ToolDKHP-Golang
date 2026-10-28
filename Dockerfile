# ==========================================
# Multi-stage Dockerfile cho Golang Backend
# Dung lượng siêu nhỏ gọn (~20MB), tốc độ khởi động tức thì
# ==========================================

# Stage 1: Build binary
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Cài đặt git và certs cần thiết
RUN apk add --no-cache git ca-certificates

# Copy go mod và go sum
COPY go-backend/go.mod go-backend/go.sum ./
RUN go mod download

# Copy source code go-backend
COPY go-backend/ ./

# Compile binary tĩnh không phụ thuộc CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dkhp-server main.go

# Stage 2: Runtime image siêu nhẹ
FROM alpine:3.20

WORKDIR /app

# Cài đặt SSL root certificates & múi giờ VN (ICT / Asia/Ho_Chi_Minh)
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Ho_Chi_Minh

# Copy binary từ build stage
COPY --from=builder /build/dkhp-server /app/dkhp-server

# Copy static frontend và data ban đầu
COPY public/ /app/public/
COPY data/ /app/data/

# Expose port (mặc định single port 3000 khi deploy VPS)
EXPOSE 3000

ENV PORT=3000

CMD ["/app/dkhp-server"]
