package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// Server 구조체 정의
type Server struct {
	RedisClient *redis.Client
	Ctx         context.Context
}

// 요청 데이터를 구조체로 정의
type RequestPayload struct {
	Method  string              `json:"method"`
	URI     string              `json:"uri"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

// NewServer: 서버 구조체 생성 및 Redis 연결 설정
func NewServer(redisAddr string) (Server, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   0, // 기본 DB
	})

	ctx := context.Background()

	// Redis 연결 확인
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return Server{}, fmt.Errorf("failed to connect to Redis at %s: %v", redisAddr, err)
	}

	return Server{
		RedisClient: rdb,
		Ctx:         ctx,
	}, nil
}

// 요청 바디 읽기 및 RequestPayload 생성
func (s Server) parseRequest(c *gin.Context) (RequestPayload, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return RequestPayload{}, err
	}

	return RequestPayload{
		Method:  c.Request.Method,
		URI:     c.Request.RequestURI,
		Headers: c.Request.Header,
		Body:    string(body),
	}, nil
}

// Redis에 요청 데이터 저장
func (s Server) pushToRedis(data RequestPayload) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return s.RedisClient.LPush(s.Ctx, "request_queue", jsonData).Err()
}

// 엔드포인트 핸들러 함수
func (s Server) handleRequest(c *gin.Context) {
	// 요청 데이터 처리
	requestData, err := s.parseRequest(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		slog.Error("Failed to read request body", slog.String("error", err.Error()))
		return
	}

	// Redis에 저장
	if err := s.pushToRedis(requestData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to push to Redis queue"})
		slog.Error("Failed to push to Redis queue", slog.String("error", err.Error()))
		return
	}

	// 성공적으로 Redis에 저장되었음을 응답
	slog.Info("Request added to Redis queue", slog.String("method", requestData.Method), slog.String("uri", requestData.URI))
	c.JSON(http.StatusOK, gin.H{"message": "Request added to Redis queue"})
}

func main() {
	// 명령줄 인수 처리
	redisAddr := flag.String("redis", "localhost:6379", "Redis server address")
	flag.Parse()

	// slog 기본 핸들러 설정 (표준 출력)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Server 인스턴스 생성
	server, err := NewServer(*redisAddr)
	if err != nil {
		slog.Error("Failed to initialize server", slog.String("error", err.Error()))
		return
	}
	slog.Info("Connected to Redis", slog.String("address", *redisAddr))

	// Gin 라우터 설정
	r := gin.Default()

	// 모든 요청을 수신하는 엔드포인트
	r.Any("/*proxyPath", server.handleRequest)

	// 서버 실행
	if err := r.Run(":8080"); err != nil {
		slog.Error("Failed to start server", slog.String("error", err.Error()))
	}
}
