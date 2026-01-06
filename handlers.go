package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func getUserStorage(r *http.Request) (*Storage, error) {
	username, ok := r.Context().Value(UserContextKey).(string)
	if !ok || username == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	return storageManager.GetStorage(username)
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func HandleTodos(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	store, err := getUserStorage(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case "GET":
		todos := store.GetAll()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(todos)
	case "POST":
		var todo Todo
		if err := json.NewDecoder(r.Body).Decode(&todo); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if todo.ID == "" {
			todo.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		if todo.CreatedAt.IsZero() {
			todo.CreatedAt = time.Now()
		}
		store.Add(todo)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(todo)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func HandleTodoItem(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	store, err := getUserStorage(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract ID from URL path
	// Path is like /api/todos/{id}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	id := parts[3]

	switch r.Method {
	case "PUT":
		var todo Todo
		if err := json.NewDecoder(r.Body).Decode(&todo); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if todo.ID != id {
			http.Error(w, "ID mismatch", http.StatusBadRequest)
			return
		}
		store.Update(todo)
		w.WriteHeader(http.StatusOK)
	case "DELETE":
		store.Delete(id)
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func HandleReorder(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	store, err := getUserStorage(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var ids []string
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	store.Reorder(ids)
	w.WriteHeader(http.StatusOK)
}
