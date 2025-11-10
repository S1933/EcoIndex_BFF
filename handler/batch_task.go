package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cnumr/ecoindex-bff/config"
	"github.com/cnumr/ecoindex-bff/models"
	"github.com/cnumr/ecoindex-bff/services"
	"github.com/gofiber/fiber/v2"
)

// CreateBatchTask crée une tâche d'analyse batch (plusieurs URLs)
func CreateBatchTask(c *fiber.Ctx) error {
	var req models.TaskCreateRequest
	if err := c.BodyParser(&req); err != nil {
		fmt.Printf("[ERROR] Failed to parse request: %v\n", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Valider la requête
	urls := req.URLs
	if len(urls) == 0 && req.URL != "" {
		urls = []string{req.URL}
	}

	if len(urls) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "At least one URL is required"})
	}

	// Valeurs par défaut pour width et height
	if req.Width == 0 {
		req.Width = 1920
	}
	if req.Height == 0 {
		req.Height = 1080
	}

	// Créer la tâche
	task, err := services.CreateTask(req.URL, urls, req.Width, req.Height)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create task: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create task"})
	}

	// Lancer le traitement en arrière-plan
	go processBatchTask(task.ID, urls, req.Width, req.Height)

	return c.Status(201).JSON(models.TaskResponse{Task: *task})
}

// GetBatchTask récupère l'état d'une tâche batch
func GetBatchTask(c *fiber.Ctx) error {
	taskID := c.Params("id")

	task, err := services.GetTask(taskID)
	if err != nil {
		fmt.Printf("[ERROR] Task not found: %v\n", err)
		return c.Status(404).JSON(fiber.Map{"error": "Task not found"})
	}

	return c.JSON(models.TaskResponse{Task: *task})
}

// ListBatchTasks liste toutes les tâches batch
func ListBatchTasks(c *fiber.Ctx) error {
	tasks, err := services.ListTasks()
	if err != nil {
		fmt.Printf("[ERROR] Failed to list tasks: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list tasks"})
	}

	return c.JSON(models.TaskListResponse{
		Tasks: tasks,
		Total: len(tasks),
	})
}

// DeleteBatchTask supprime une tâche batch
func DeleteBatchTask(c *fiber.Ctx) error {
	taskID := c.Params("id")

	if err := services.DeleteTask(taskID); err != nil {
		fmt.Printf("[ERROR] Failed to delete task: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete task"})
	}

	return c.Status(204).Send(nil)
}

// processBatchTask traite une tâche batch en arrière-plan
func processBatchTask(taskID string, urls []string, width, height int) {
	fmt.Printf("[INFO] Starting batch task %s with %d URLs\n", taskID, len(urls))

	// Mettre à jour le statut à "processing"
	if err := services.UpdateTaskStatus(taskID, models.TaskStatusProcessing, 0, 0, ""); err != nil {
		fmt.Printf("[ERROR] Failed to update task status: %v\n", err)
		return
	}

	// Traiter chaque URL
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // Limiter à 3 requêtes simultanées
	results := make(chan string, len(urls))
	errors := make(chan error, len(urls))

	for i, url := range urls {
		wg.Add(1)
		go func(index int, pageURL string) {
			defer wg.Done()

			// Acquérir le sémaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fmt.Printf("[INFO] Processing URL %d/%d: %s\n", index+1, len(urls), pageURL)

			// Créer une tâche EcoIndex pour cette URL
			resultID, err := createEcoindexTask(pageURL, width, height)
			if err != nil {
				fmt.Printf("[ERROR] Failed to create ecoindex task for %s: %v\n", pageURL, err)
				errors <- err
				return
			}

			// Attendre que la tâche soit terminée
			if err := waitForEcoindexTask(resultID); err != nil {
				fmt.Printf("[ERROR] Failed to wait for ecoindex task %s: %v\n", resultID, err)
				errors <- err
				return
			}

			results <- resultID

			// Mettre à jour la progression
			progress := int(float64(index+1) / float64(len(urls)) * 100)
			if err := services.UpdateTaskStatus(taskID, models.TaskStatusProcessing, progress, index+1, ""); err != nil {
				fmt.Printf("[ERROR] Failed to update task progress: %v\n", err)
			}

			// Ajouter le résultat à la tâche
			if err := services.AddTaskResult(taskID, resultID); err != nil {
				fmt.Printf("[ERROR] Failed to add result to task: %v\n", err)
			}
		}(i, url)
	}

	// Attendre que toutes les goroutines se terminent
	wg.Wait()
	close(results)
	close(errors)

	// Vérifier s'il y a eu des erreurs
	errorCount := len(errors)
	if errorCount > 0 {
		errorMsg := fmt.Sprintf("%d/%d URLs failed", errorCount, len(urls))
		if errorCount == len(urls) {
			services.UpdateTaskStatus(taskID, models.TaskStatusFailed, 100, len(urls), errorMsg)
		} else {
			services.UpdateTaskStatus(taskID, models.TaskStatusCompleted, 100, len(urls), errorMsg)
		}
	} else {
		services.UpdateTaskStatus(taskID, models.TaskStatusCompleted, 100, len(urls), "")
	}

	fmt.Printf("[INFO] Batch task %s completed\n", taskID)
}

// createEcoindexTask crée une tâche EcoIndex et retourne son ID
func createEcoindexTask(url string, width, height int) (string, error) {
	ecoReq := EcoindexTaskRequest{
		CustomHeaders: make(map[string]string),
	}
	ecoReq.WebPage.URL = url
	ecoReq.WebPage.Width = width
	ecoReq.WebPage.Height = height

	jsonData, err := json.Marshal(ecoReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	targetURL := config.ENV.ApiUrl + "/v1/tasks/ecoindexes/"
	if config.ENV.ApiUrl == "https://ecoindex.p.rapidapi.com" {
		targetURL = "https://api.ecoindex.fr/v1/tasks/ecoindexes/"
	}

	req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 201 {
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	id, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("no id in response")
	}

	return id, nil
}

// waitForEcoindexTask attend qu'une tâche EcoIndex soit terminée
func waitForEcoindexTask(taskID string) error {
	targetURL := config.ENV.ApiUrl + "/v1/tasks/ecoindexes/" + taskID
	if config.ENV.ApiUrl == "https://ecoindex.p.rapidapi.com" {
		targetURL = "https://api.ecoindex.fr/v1/tasks/ecoindexes/" + taskID
	}

	maxAttempts := 60 // 60 tentatives = 5 minutes max
	for i := 0; i < maxAttempts; i++ {
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}

		status, ok := result["status"].(string)
		if !ok {
			return fmt.Errorf("no status in response")
		}

		if status == "SUCCESS" {
			return nil
		}

		if status == "FAILURE" {
			return fmt.Errorf("task failed")
		}

		// Attendre 5 secondes avant de réessayer
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("task timeout")
}
