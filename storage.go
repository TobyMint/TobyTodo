package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const DataDir = "data"

type Todo struct {
	ID          string    `json:"id"`
	Content     string    `json:"content"`
	Completed   bool      `json:"completed"`
	Order       int       `json:"order"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type Storage struct {
	mu       sync.Mutex
	FilePath string
	Todos    []Todo
}

type StorageManager struct {
	mu       sync.Mutex
	Storages map[string]*Storage
}

func NewStorageManager() *StorageManager {
	return &StorageManager{
		Storages: make(map[string]*Storage),
	}
}

func (sm *StorageManager) GetStorage(username string) (*Storage, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, exists := sm.Storages[username]; exists {
		return s, nil
	}

	filePath := filepath.Join(DataDir, fmt.Sprintf("%s_todos.json", username))
	s := &Storage{
		FilePath: filePath,
		Todos:    []Todo{},
	}

	if err := s.Load(); err != nil {
		return nil, err
	}

	sm.Storages[username] = s
	return s, nil
}

func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.FilePath)
	if os.IsNotExist(err) {
		s.Todos = []Todo{}
		return nil
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.Todos)
}

func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.Todos, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.FilePath, data, 0644)
}

func (s *Storage) GetAll() []Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Return a copy to be safe
	result := make([]Todo, len(s.Todos))
	copy(result, s.Todos)

	// Sort by Order
	sort.Slice(result, func(i, j int) bool {
		return result[i].Order < result[j].Order
	})

	return result
}

func (s *Storage) Add(todo Todo) error {
	s.mu.Lock()
	// Set CreatedAt if not set
	if todo.CreatedAt.IsZero() {
		todo.CreatedAt = time.Now()
	}
	// Assign order if not set (append to end)
	if todo.Order == 0 {
		maxOrder := 0
		for _, t := range s.Todos {
			if t.Order > maxOrder {
				maxOrder = t.Order
			}
		}
		todo.Order = maxOrder + 1
	}
	s.Todos = append(s.Todos, todo)
	s.mu.Unlock()
	return s.Save()
}

func (s *Storage) GetCompletedTodosByPeriod(period string) []Todo {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filtered []Todo
	now := time.Now()
	// Normalize to start of day
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var startTime time.Time

	switch period {
	case "today":
		startTime = todayStart
	case "week":
		// Week starts on Monday
		offset := int(now.Weekday())
		if offset == 0 {
			offset = 7
		}
		startTime = todayStart.AddDate(0, 0, -(offset - 1))
	case "month":
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		return []Todo{}
	}

	for _, t := range s.Todos {
		if t.Completed && !t.CompletedAt.IsZero() && (t.CompletedAt.After(startTime) || t.CompletedAt.Equal(startTime)) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func (s *Storage) Update(updatedTodo Todo) error {
	s.mu.Lock()
	for i, t := range s.Todos {
		if t.ID == updatedTodo.ID {
			// Update logic:
			// Preserve CreatedAt from original if not provided (though it should be)
			if updatedTodo.CreatedAt.IsZero() {
				updatedTodo.CreatedAt = t.CreatedAt
			}

			// Handle CompletedAt
			if updatedTodo.Completed && !t.Completed {
				// Just completed
				updatedTodo.CompletedAt = time.Now()
			} else if !updatedTodo.Completed {
				// Not completed (reopened)
				updatedTodo.CompletedAt = time.Time{}
			} else if updatedTodo.Completed && t.Completed {
				// Already completed, preserve original completion time unless specified
				if updatedTodo.CompletedAt.IsZero() {
					updatedTodo.CompletedAt = t.CompletedAt
				}
			}

			s.Todos[i] = updatedTodo
			break
		}
	}
	s.mu.Unlock()
	return s.Save()
}

func (s *Storage) Delete(id string) error {
	s.mu.Lock()
	newTodos := []Todo{}
	for _, t := range s.Todos {
		if t.ID != id {
			newTodos = append(newTodos, t)
		}
	}
	s.Todos = newTodos
	s.mu.Unlock()
	return s.Save()
}

func (s *Storage) Reorder(ids []string) error {
	s.mu.Lock()
	// Create a map for quick lookup
	todoMap := make(map[string]int)
	for i, t := range s.Todos {
		todoMap[t.ID] = i
	}

	// Reassign orders based on the incoming ids list
	for order, id := range ids {
		if idx, exists := todoMap[id]; exists {
			s.Todos[idx].Order = order
		}
	}
	s.mu.Unlock()
	return s.Save()
}
