package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cnumr/ecoindex-bff/config"
	"github.com/cnumr/ecoindex-bff/models"
	"github.com/go-redis/cache/v8"
	"github.com/google/uuid"
)

const (
	taskKeyPrefix = "task:"
	taskListKey   = "tasks:list"
	taskTTL       = 7 * 24 * time.Hour // 7 jours
)

// CreateTask crée une nouvelle tâche et la stocke dans Redis
func CreateTask(url string, urls []string, width, height int) (*models.Task, error) {
	task := &models.Task{
		ID:          uuid.New().String(),
		URL:         url,
		Width:       width,
		Height:      height,
		Status:      models.TaskStatusPending,
		Progress:    0,
		TotalPages:  1,
		CurrentPage: 0,
		Results:     []string{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Si plusieurs URLs sont fournies, on les compte
	if len(urls) > 0 {
		task.TotalPages = len(urls)
	}

	// Sauvegarder la tâche dans Redis
	if err := SaveTask(task); err != nil {
		return nil, err
	}

	// Ajouter l'ID de la tâche à la liste des tâches
	if err := addTaskToList(task.ID); err != nil {
		return nil, err
	}

	return task, nil
}

// SaveTask sauvegarde une tâche dans Redis
func SaveTask(task *models.Task) error {
	task.UpdatedAt = time.Now()

	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	ctx := context.Background()
	key := taskKeyPrefix + task.ID

	return config.CACHE.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: data,
		TTL:   taskTTL,
	})
}

// GetTask récupère une tâche depuis Redis
func GetTask(taskID string) (*models.Task, error) {
	ctx := context.Background()
	key := taskKeyPrefix + taskID

	var data []byte
	if err := config.CACHE.Get(ctx, key, &data); err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	var task models.Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

// UpdateTaskStatus met à jour le statut d'une tâche
func UpdateTaskStatus(taskID string, status models.TaskStatus, progress int, currentPage int, error string) error {
	task, err := GetTask(taskID)
	if err != nil {
		return err
	}

	task.Status = status
	task.Progress = progress
	task.CurrentPage = currentPage

	if error != "" {
		task.Error = error
	}

	if status == models.TaskStatusCompleted || status == models.TaskStatusFailed {
		now := time.Now()
		task.CompletedAt = &now
	}

	return SaveTask(task)
}

// AddTaskResult ajoute un résultat EcoIndex à une tâche
func AddTaskResult(taskID string, resultID string) error {
	task, err := GetTask(taskID)
	if err != nil {
		return err
	}

	task.Results = append(task.Results, resultID)
	return SaveTask(task)
}

// ListTasks récupère toutes les tâches actives
func ListTasks() ([]models.Task, error) {
	ctx := context.Background()

	// Récupérer la liste des IDs de tâches
	var taskIDs []string
	if err := config.CACHE.Get(ctx, taskListKey, &taskIDs); err != nil {
		// Si la liste n'existe pas, retourner une liste vide
		return []models.Task{}, nil
	}

	tasks := make([]models.Task, 0, len(taskIDs))
	for _, id := range taskIDs {
		task, err := GetTask(id)
		if err != nil {
			// Si une tâche n'existe plus, on la saute
			continue
		}
		tasks = append(tasks, *task)
	}

	return tasks, nil
}

// addTaskToList ajoute un ID de tâche à la liste des tâches
func addTaskToList(taskID string) error {
	ctx := context.Background()

	var taskIDs []string
	// Ignorer l'erreur si la liste n'existe pas encore
	_ = config.CACHE.Get(ctx, taskListKey, &taskIDs)

	// Ajouter le nouvel ID
	taskIDs = append(taskIDs, taskID)

	data, err := json.Marshal(taskIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal task list: %w", err)
	}

	return config.CACHE.Set(&cache.Item{
		Ctx:   ctx,
		Key:   taskListKey,
		Value: data,
		TTL:   taskTTL,
	})
}

// DeleteTask supprime une tâche de Redis
func DeleteTask(taskID string) error {
	ctx := context.Background()
	key := taskKeyPrefix + taskID

	// Supprimer la tâche
	if err := config.CACHE.Delete(ctx, key); err != nil {
		return err
	}

	// Retirer l'ID de la liste
	var taskIDs []string
	if err := config.CACHE.Get(ctx, taskListKey, &taskIDs); err != nil {
		return nil // Si la liste n'existe pas, c'est OK
	}

	// Filtrer l'ID à supprimer
	newTaskIDs := make([]string, 0, len(taskIDs))
	for _, id := range taskIDs {
		if id != taskID {
			newTaskIDs = append(newTaskIDs, id)
		}
	}

	data, err := json.Marshal(newTaskIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal task list: %w", err)
	}

	return config.CACHE.Set(&cache.Item{
		Ctx:   ctx,
		Key:   taskListKey,
		Value: data,
		TTL:   taskTTL,
	})
}
