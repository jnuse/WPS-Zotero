package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

const (
	ZOTERO_PORT = 23119
	PROXY_PORT  = 21931
)

var PREFLIGHT_HEADERS = map[string]string{
	"Access-Control-Allow-Origin":      "*",
	"Access-Control-Allow-Methods":     "GET,POST,OPTIONS,PUT,PATCH,DELETE",
	"Access-Control-Allow-Headers":     "*",
	"Access-Control-Allow-Credentials": "true",
}

// CorsMiddleware handles CORS preflight requests
func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			for key, value := range PREFLIGHT_HEADERS {
				c.Header(key, value)
			}
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}

func main() {
	// Set up the target URL for Zotero
	zoteroURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", ZOTERO_PORT))
	if err != nil {
		panic(fmt.Sprintf("Failed to parse Zotero URL: %v", err))
	}

	// Create a new reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(zoteroURL)

	// This Director function is called for every request.
	// We can modify the request here if needed, but for a simple proxy, it's fine as is.
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = zoteroURL.Scheme
		req.URL.Host = zoteroURL.Host
		req.Host = zoteroURL.Host
	}

	// This ModifyResponse function is called for every response.
	// We add the Access-Control-Allow-Origin header here.
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		return nil
	}

	// Set up Gin router
	router := gin.Default()

	// Use the CORS middleware for all routes
	router.Use(CorsMiddleware())

	// All other requests are handled by the reverse proxy
	router.Any("/*path", func(c *gin.Context) {
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	// Start the server
	serverAddr := fmt.Sprintf(":%d", PROXY_PORT)
	fmt.Printf("Proxy server listening on %s\n", serverAddr)
	if err := router.Run(serverAddr); err != nil {
		panic(fmt.Sprintf("Failed to start server: %v", err))
	}
}
