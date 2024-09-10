package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Server 구조체 정의
type Server struct {
	KafkaClient *kgo.Client
}

// 요청 데이터를 구조체로 정의
type RequestPayload struct {
	Method  string              `json:"method"`
	URI     string              `json:"uri"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

// NewServer: 서버 구조체 생성 및 Redis 연결 설정
func NewServer(seeds []string) (*Server, error) {
	client, err := kgo.NewClient(
		kgo.ConsumerGroup("my-group-identifier"),
		kgo.ConsumeTopics("foo"),
	)
	if err != nil {
		return nil, err
	}

	return &Server{
		KafkaClient: client,
	}, nil
}

// 요청 바디 읽기 및 RequestPayload 생성
func (s *Server) parseRequest(c *gin.Context) (RequestPayload, error) {
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
func (s *Server) produce(data RequestPayload) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	record := &kgo.Record{Topic: "foo", Value: jsonData}

	return s.KafkaClient.ProduceSync(context.Background(), record).FirstErr()
}

// 엔드포인트 핸들러 함수
func (s *Server) handleRequest(c *gin.Context) {
	// 요청 데이터 처리
	requestData, err := s.parseRequest(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
		slog.Error("Failed to read request body", slog.String("error", err.Error()))
		return
	}

	// Redis에 저장
	if err := s.produce(requestData); err != nil {
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
	kafkaAddrRaw := flag.String("kafka address", "localhost:9092", "kafka server address")
	flag.Parse()

	kafkaAddr := strings.Split(*kafkaAddrRaw, ",")

	// slog 기본 핸들러 설정 (표준 출력)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Server 인스턴스 생성
	server, err := NewServer(kafkaAddr)
	if err != nil {
		slog.Error("Failed to initialize server", slog.String("error", err.Error()))
		return
	}
	slog.Info("Connected to kafka", slog.String("address", *kafkaAddrRaw))

	// Gin 라우터 설정
	r := gin.Default()

	// 모든 요청을 수신하는 엔드포인트
	r.Any("/*proxyPath", server.handleRequest)

	// 서버 실행
	if err := r.Run(":8080"); err != nil {
		slog.Error("Failed to start server", slog.String("error", err.Error()))
	}
}
