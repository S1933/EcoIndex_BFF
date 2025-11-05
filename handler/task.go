package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cnumr/ecoindex-bff/config"
	"github.com/gofiber/fiber/v2"
)

type TaskRequest struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type EcoindexTaskRequest struct {
	WebPage struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"web_page"`
	CustomHeaders map[string]string `json:"custom_headers"`
}

func CreateTask(c *fiber.Ctx) error {
	// Parse incoming request
	var taskReq TaskRequest
	if err := c.BodyParser(&taskReq); err != nil {
		fmt.Printf("[ERROR] Failed to parse request: %v\n", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Transform to ecoindex API format
	ecoReq := EcoindexTaskRequest{
		CustomHeaders: make(map[string]string),
	}
	ecoReq.WebPage.URL = taskReq.URL
	ecoReq.WebPage.Width = taskReq.Width
	ecoReq.WebPage.Height = taskReq.Height

	jsonData, err := json.Marshal(ecoReq)
	if err != nil {
		fmt.Printf("[ERROR] Failed to marshal request: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	// Determine target URL based on API_URL
	targetURL := config.ENV.ApiUrl + "/v1/tasks/ecoindexes/"

	// If using RapidAPI, use the direct API instead to avoid redirect issues
	if config.ENV.ApiUrl == "https://ecoindex.p.rapidapi.com" {
		targetURL = "https://api.ecoindex.fr/v1/tasks/ecoindexes/"
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("[ERROR] Failed to create request: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("[ERROR] Request failed: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("API request failed: %v", err)})
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[ERROR] Failed to read response: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to read API response"})
	}

	// Forward response
	c.Status(resp.StatusCode)
	c.Set("Content-Type", "application/json")
	return c.Send(body)
}

func GetTask(c *fiber.Ctx) error {
	taskID := c.Params("id")

	// Determine target URL
	targetURL := config.ENV.ApiUrl + "/v1/tasks/ecoindexes/" + taskID

	// If using RapidAPI, use the direct API instead
	if config.ENV.ApiUrl == "https://ecoindex.p.rapidapi.com" {
		targetURL = "https://api.ecoindex.fr/v1/tasks/ecoindexes/" + taskID
	}

	// Create HTTP request
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create request: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("[ERROR] Request failed: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("API request failed: %v", err)})
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[ERROR] Failed to read response: %v\n", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to read API response"})
	}

	// Forward response
	c.Status(resp.StatusCode)
	c.Set("Content-Type", "application/json")
	return c.Send(body)
}
