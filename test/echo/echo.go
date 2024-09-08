package main

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// 모든 종류의 요청을 받아 처리 (GET, POST, PUT, DELETE, etc.)
	r.Any("/*proxyPath", func(c *gin.Context) {
		// 요청 메서드와 URI 출력
		fmt.Printf("Method: %s\n", c.Request.Method)
		fmt.Printf("URI: %s\n", c.Request.RequestURI)

		// 요청 헤더 출력
		fmt.Println("Headers:")
		for k, v := range c.Request.Header {
			fmt.Printf("%s: %s\n", k, v)
		}

		// 요청 본문(body) 출력 (POST, PUT 요청 시)
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut {
			body, err := ioutil.ReadAll(c.Request.Body)
			if err == nil {
				fmt.Printf("Body: %s\n", string(body))
			}
		}

		// 간단한 응답 (원하는 대로 수정 가능)
		c.JSON(http.StatusOK, gin.H{
			"message": "Request received and printed to console",
		})
	})

	// 서버 실행
	r.Run(":8080")
}
