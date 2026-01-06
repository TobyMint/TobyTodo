package main

import (
	"log"
	"net/http"
)

func main() {
	// API Endpoints
	http.HandleFunc("/api/todos", HandleTodos)
	http.HandleFunc("/api/todos/", HandleTodoItem)
	http.HandleFunc("/api/reorder", HandleReorder)
	http.HandleFunc("/api/summary", SummaryHandler)

	// Static Files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	log.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
