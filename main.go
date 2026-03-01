package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	userManager    *UserManager
	sessionManager *SessionManager
	storageManager *StorageManager
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func main() {
	// Initialize Managers
	userManager = NewUserManager()
	sessionManager = NewSessionManager()
	storageManager = NewStorageManager()

	r := gin.Default()
	r.Use(CORSMiddleware())

	// Public Static Files
	r.StaticFile("/login.html", "./static/login.html")
	r.StaticFile("/register.html", "./static/register.html")
	r.StaticFile("/style.css", "./static/style.css")
	r.StaticFile("/app.js", "./static/app.js")

	// Public API
	r.POST("/api/login", HandleLogin)
	r.POST("/api/register", HandleRegister)
	r.Any("/api/logout", HandleLogout) // Logout can be GET or POST

	// Protected Routes
	authorized := r.Group("/")
	authorized.Use(AuthMiddleware())
	{
		// Static Home
		authorized.StaticFile("/", "./static/index.html")
		authorized.StaticFile("/index.html", "./static/index.html")

		// API
		api := authorized.Group("/api")
		{
			api.GET("/todos", GetTodos)
			api.POST("/todos", CreateTodo)
			api.PUT("/todos/:id", UpdateTodo)
			api.DELETE("/todos/:id", DeleteTodo)
			api.POST("/reorder", ReorderTodos)
			api.GET("/summary", GetSummary)
		}
	}

	port := flag.Int("port", 8080, "server listen port")
	enableHTTPS := flag.Bool("https", false, "enable HTTPS")
	tlsCertFile := flag.String("tls-cert", "", "path to TLS certificate file")
	tlsKeyFile := flag.String("tls-key", "", "path to TLS private key file")
	flag.Parse()
	addr := fmt.Sprintf(":%d", *port)

	// Check for inconsistent flags
	if !*enableHTTPS && (*tlsCertFile != "" || *tlsKeyFile != "") {
		log.Fatal("HTTPS 未启用 (--https=false)，但指定了证书文件。请添加 --https 参数以启用 HTTPS，或移除证书参数以使用 HTTP。")
	}

	if *enableHTTPS {
		if *tlsCertFile == "" || *tlsKeyFile == "" {
			log.Fatal("HTTPS 已启用，但未指定证书文件 (--tls-cert) 或私钥文件 (--tls-key)")
		}
		log.Println("HTTPS server starting on", addr)
		server := &http.Server{
			Addr:    addr,
			Handler: r,
		}
		if err := server.ListenAndServeTLS(*tlsCertFile, *tlsKeyFile); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("HTTP server starting on", addr)
		if err := r.Run(addr); err != nil {
			log.Fatal(err)
		}
	}
}
