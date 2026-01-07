package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func getUserStorage(c *gin.Context) (*Storage, error) {
	username := c.GetString(UserKey)
	if username == "" {
		return nil, fmt.Errorf("unauthorized")
	}
	return storageManager.GetStorage(username)
}

func GetTodos(c *gin.Context) {
	store, err := getUserStorage(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	todos := store.GetAll()
	c.JSON(http.StatusOK, todos)
}

func CreateTodo(c *gin.Context) {
	store, err := getUserStorage(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var todo Todo
	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if todo.ID == "" {
		todo.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if todo.CreatedAt.IsZero() {
		todo.CreatedAt = time.Now()
	}
	store.Add(todo)
	c.JSON(http.StatusOK, todo)
}

func UpdateTodo(c *gin.Context) {
	store, err := getUserStorage(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	id := c.Param("id")
	var todo Todo
	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if todo.ID != id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID mismatch"})
		return
	}
	store.Update(todo)
	c.Status(http.StatusOK)
}

func DeleteTodo(c *gin.Context) {
	store, err := getUserStorage(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id := c.Param("id")
	store.Delete(id)
	c.Status(http.StatusOK)
}

func ReorderTodos(c *gin.Context) {
	store, err := getUserStorage(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var ids []string
	if err := c.ShouldBindJSON(&ids); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	store.Reorder(ids)
	c.Status(http.StatusOK)
}
