package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	UsersFile  = "data/users.json"
	CookieName = "session_token"
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

// Context Key
type contextKey string

const UserContextKey contextKey = "user"

// isPublicPath checks if the path is accessible without authentication
func isPublicPath(path string) bool {
	publicPaths := map[string]bool{
		"/login.html":    true,
		"/register.html": true,
		"/style.css":     true,
		"/app.js":        true,
		"/api/login":     true,
		"/api/register":  true,
	}
	return publicPaths[path]
}

// Middleware
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err != nil {
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}

		username, ok := sessionManager.GetUsername(c.Value)
		if !ok {
			// Cookie is invalid (e.g. server restarted), clear it
			http.SetCookie(w, &http.Cookie{
				Name:    CookieName,
				Value:   "",
				Path:    "/",
				Expires: time.Unix(0, 0),
				MaxAge:  -1,
			})

			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}

		// Refresh session cookie
		http.SetCookie(w, &http.Cookie{
			Name:    CookieName,
			Value:   c.Value,
			Expires: time.Now().Add(24 * time.Hour),
			Path:    "/",
		})

		ctx := context.WithValue(r.Context(), UserContextKey, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Auth Handlers
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := userManager.Login(creds.Username, creds.Password); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token := sessionManager.CreateSession(creds.Username)
	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   token,
		Expires: time.Now().Add(24 * time.Hour),
		Path:    "/",
	})

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if creds.Username == "" || creds.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	if err := userManager.Register(creds.Username, creds.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Auto login
	token := sessionManager.CreateSession(creds.Username)
	http.SetCookie(w, &http.Cookie{
		Name:    CookieName,
		Value:   token,
		Expires: time.Now().Add(24 * time.Hour),
		Path:    "/",
	})

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(CookieName)
	if err == nil {
		sessionManager.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   CookieName,
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})
	http.Redirect(w, r, "/login.html", http.StatusFound)
}
