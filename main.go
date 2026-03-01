package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
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

		log.Println("HTTPS server starting on", addr, "(supporting automatic HTTP->HTTPS redirect)")

		// Create a custom listener that can handle both HTTP and HTTPS on the same port
		l, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatal(err)
		}

		// Channel to pass TLS connections to the HTTPS server
		tlsConnChan := make(chan net.Conn)
		tlsListener := &ChanListener{
			AddrVal:  l.Addr(),
			ConnChan: tlsConnChan,
		}

		// Start the HTTPS server using our custom listener
		go func() {
			server := &http.Server{
				Handler: r,
			}
			// ServeTLS will perform the TLS handshake on connections from tlsListener
			if err := server.ServeTLS(tlsListener, *tlsCertFile, *tlsKeyFile); err != nil {
				log.Fatal(err)
			}
		}()

		// Accept loop for the main TCP listener
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				continue
			}

			go func(c net.Conn) {
				// Peek at the first byte to determine protocol
				// We need a buffered reader to peek without consuming
				bufConn := NewBufferedConn(c)

				// Read a few bytes to sniff the protocol
				// TLS handshake starts with 0x16 (22)
				// HTTP methods start with 'G', 'P', 'D', 'O', etc.
				prefix, err := bufConn.Peek(1)
				if err != nil {
					c.Close()
					return
				}

				if prefix[0] == 0x16 {
					// This looks like TLS, pass to the HTTPS server
					tlsConnChan <- bufConn
				} else {
					// Assume HTTP, redirect to HTTPS
					handleHTTPRedirect(bufConn, addr)
				}
			}(conn)
		}

	} else {
		log.Println("HTTP server starting on", addr)
		if err := r.Run(addr); err != nil {
			log.Fatal(err)
		}
	}
}

// BufferedConn wraps a net.Conn with a bufio.Reader to allow peeking
type BufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func NewBufferedConn(c net.Conn) *BufferedConn {
	return &BufferedConn{
		Conn: c,
		r:    bufio.NewReader(c),
	}
}

func (bc *BufferedConn) Peek(n int) ([]byte, error) {
	return bc.r.Peek(n)
}

func (bc *BufferedConn) Read(p []byte) (int, error) {
	return bc.r.Read(p)
}

// ChanListener implements net.Listener but accepts connections from a channel
type ChanListener struct {
	AddrVal  net.Addr
	ConnChan chan net.Conn
}

func (l *ChanListener) Accept() (net.Conn, error) {
	c, ok := <-l.ConnChan
	if !ok {
		return nil, fmt.Errorf("listener closed")
	}
	return c, nil
}

func (l *ChanListener) Close() error {
	return nil
}

func (l *ChanListener) Addr() net.Addr {
	return l.AddrVal
}

func handleHTTPRedirect(conn net.Conn, httpsAddr string) {
	defer conn.Close()

	// Read the request to get the Host header
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return
	}

	host := req.Host
	// If the host doesn't have a port, and we are on a non-standard port, we might need to append it?
	// But usually req.Host contains the port if the client sent it.
	// However, if we are behind a proxy or using port mapping, it might be tricky.
	// For this simple case, we assume req.Host is what the browser sees.

	// Construct redirect URL
	target := "https://" + host + req.URL.String()

	// Send 301 Redirect response
	resp := fmt.Sprintf("HTTP/1.1 301 Moved Permanently\r\n"+
		"Location: %s\r\n"+
		"Connection: close\r\n"+
		"\r\n", target)

	conn.Write([]byte(resp))
}
