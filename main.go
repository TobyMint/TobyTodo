package main

import (
	"log"
	"net/http"
)

var (
	userManager    *UserManager
	sessionManager *SessionManager
	storageManager *StorageManager
)

func main() {
	// Initialize Managers
	userManager = NewUserManager()
	sessionManager = NewSessionManager()
	storageManager = NewStorageManager()

	// Auth Endpoints
	http.HandleFunc("/api/login", LoginHandler)
	http.HandleFunc("/api/register", RegisterHandler)
	http.HandleFunc("/api/logout", LogoutHandler)

	// API Endpoints (Protected)
	http.Handle("/api/todos", AuthMiddleware(http.HandlerFunc(HandleTodos)))
	http.Handle("/api/todos/", AuthMiddleware(http.HandlerFunc(HandleTodoItem)))
	http.Handle("/api/reorder", AuthMiddleware(http.HandlerFunc(HandleReorder)))
	http.Handle("/api/summary", AuthMiddleware(http.HandlerFunc(SummaryHandler)))

	// Static Files (Protected, except login)
	// We need a custom handler for static files to allow login.html
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", AuthMiddleware(fs))

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
