package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	UsersFile  = "data/users.json"
	CookieName = "session_token"
	UserKey    = "user"
)

type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

type UserManager struct {
	mu    sync.RWMutex
	Users map[string]User
}

func NewUserManager() *UserManager {
	um := &UserManager{
		Users: make(map[string]User),
	}
	um.Load()
	return um
}

func (um *UserManager) Load() error {
	um.mu.Lock()
	defer um.mu.Unlock()

	data, err := os.ReadFile(UsersFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &um.Users)
}

func (um *UserManager) save() error {
	data, err := json.MarshalIndent(um.Users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(UsersFile, data, 0644)
}

func (um *UserManager) Save() error {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.save()
}

func (um *UserManager) Register(username, password string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	if _, exists := um.Users[username]; exists {
		return errors.New("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	um.Users[username] = User{
		Username:     username,
		PasswordHash: string(hash),
	}
	return um.save() // Note: calling save() inside lock
}

func (um *UserManager) Login(username, password string) error {
	um.mu.RLock()
	user, exists := um.Users[username]
	um.mu.RUnlock()

	if !exists {
		return errors.New("invalid credentials")
	}

	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

// Session Management
type SessionManager struct {
	mu       sync.RWMutex
	Sessions map[string]string // token -> username
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		Sessions: make(map[string]string),
	}
}

func (sm *SessionManager) CreateSession(username string) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	token := uuid.New().String()
	sm.Sessions[token] = username
	return token
}

func (sm *SessionManager) GetUsername(token string) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	username, exists := sm.Sessions[token]
	return username, exists
}

func (sm *SessionManager) DeleteSession(token string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.Sessions, token)
}

// Middleware
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CookieName)
		if err != nil {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.AbortWithStatus(http.StatusUnauthorized)
			} else {
				c.Redirect(http.StatusFound, "/login.html")
				c.Abort()
			}
			return
		}

		username, ok := sessionManager.GetUsername(token)
		if !ok {
			// Cookie is invalid (e.g. server restarted), clear it
			c.SetCookie(CookieName, "", -1, "/", "", false, false)

			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.AbortWithStatus(http.StatusUnauthorized)
			} else {
				c.Redirect(http.StatusFound, "/login.html")
				c.Abort()
			}
			return
		}

		// Refresh session cookie
		c.SetCookie(CookieName, token, 3600*24, "/", "", false, false)

		c.Set(UserKey, username)
		c.Next()
	}
}

// Auth Handlers
func HandleLogin(c *gin.Context) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := userManager.Login(creds.Username, creds.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token := sessionManager.CreateSession(creds.Username)
	c.SetCookie(CookieName, token, 3600*24, "/", "", false, false)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func HandleRegister(c *gin.Context) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if creds.Username == "" || creds.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password required"})
		return
	}

	if err := userManager.Register(creds.Username, creds.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Auto login
	token := sessionManager.CreateSession(creds.Username)
	c.SetCookie(CookieName, token, 3600*24, "/", "", false, false)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func HandleLogout(c *gin.Context) {
	token, err := c.Cookie(CookieName)
	if err == nil {
		sessionManager.DeleteSession(token)
	}
	c.SetCookie(CookieName, "", -1, "/", "", false, false)
	c.Redirect(http.StatusFound, "/login.html")
}
