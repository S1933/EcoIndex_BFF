package models

import "time"

// TaskStatus représente l'état d'une tâche
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// Task représente une tâche d'analyse stockée
type Task struct {
	ID          string     `json:"id"`
	URL         string     `json:"url"`
	Width       int        `json:"width"`
	Height      int        `json:"height"`
	Status      TaskStatus `json:"status"`
	Progress    int        `json:"progress"`     // Pourcentage de progression (0-100)
	TotalPages  int        `json:"total_pages"`  // Nombre total de pages à analyser
	CurrentPage int        `json:"current_page"` // Page en cours d'analyse
	Results     []string   `json:"results"`      // IDs des résultats EcoIndex
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TaskListResponse représente la réponse pour la liste des tâches
type TaskListResponse struct {
	Tasks []Task `json:"tasks"`
	Total int    `json:"total"`
}

// TaskCreateRequest représente la requête de création d'une tâche
type TaskCreateRequest struct {
	URL    string   `json:"url"`
	URLs   []string `json:"urls,omitempty"` // Pour analyser plusieurs URLs
	Width  int      `json:"width"`
	Height int      `json:"height"`
}

// TaskResponse représente la réponse pour une tâche individuelle
type TaskResponse struct {
	Task Task `json:"task"`
}
