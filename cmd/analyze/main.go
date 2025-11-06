package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cnumr/ecoindex-bff/config"
	"github.com/joho/godotenv"
)

type TaskRequest struct {
	WebPage struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"web_page"`
	CustomHeaders map[string]string `json:"custom_headers"`
}

type TaskResponse struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Detail json.RawMessage `json:"detail,omitempty"`
}

type ErrorResponse struct {
	Detail struct {
		DailyLimitPerHost int    `json:"daily_limit_per_host"`
		Limit             int    `json:"limit"`
		Host              string `json:"host"`
		Message           string `json:"message"`
		LatestResult      struct {
			ID              string  `json:"id"`
			URL             string  `json:"url"`
			Width           int     `json:"width"`
			Height          int     `json:"height"`
			Size            float64 `json:"size"`
			Nodes           int     `json:"nodes"`
			Requests        int     `json:"requests"`
			Grade           string  `json:"grade"`
			Score           float64 `json:"score"`
			Ges             float64 `json:"ges"`
			Water           float64 `json:"water"`
			Date            string  `json:"date"`
			EcoindexVersion string  `json:"ecoindex_version"`
		} `json:"latest_result"`
	} `json:"detail"`
}

type TaskResult struct {
	ID              string  `json:"id"`
	URL             string  `json:"url"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	Size            float64 `json:"size"`
	Nodes           int     `json:"nodes"`
	Requests        int     `json:"requests"`
	Grade           string  `json:"grade"`
	Score           float64 `json:"score"`
	Ges             float64 `json:"ges"`
	Water           float64 `json:"water"`
	Date            string  `json:"date"`
	EcoindexVersion string  `json:"ecoindex_version"`
}

func main() {
	// Définir les flags
	url := flag.String("url", "", "URL à analyser (obligatoire)")
	width := flag.Int("width", 1920, "Largeur de la fenêtre du navigateur")
	height := flag.Int("height", 1080, "Hauteur de la fenêtre du navigateur")
	envFile := flag.String("env", ".env", "Chemin vers le fichier .env")
	flag.Parse()

	// Vérifier que l'URL est fournie
	if *url == "" {
		fmt.Println("❌ Erreur: L'URL est obligatoire")
		flag.Usage()
		os.Exit(1)
	}

	// Charger les variables d'environnement
	if err := godotenv.Load(*envFile); err != nil {
		fmt.Printf("⚠️  Avertissement: Impossible de charger le fichier .env: %v\n", err)
	}

	config.ENV = config.GetEnvironment()

	fmt.Printf("🔍 Analyse de l'URL: %s\n", *url)
	fmt.Printf("📐 Dimensions: %dx%d\n", *width, *height)
	fmt.Println()

	// Créer la tâche
	taskID, err := createTask(*url, *width, *height)
	if err != nil {
		fmt.Printf("❌ Erreur lors de la création de la tâche: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Tâche créée avec l'ID: %s\n", taskID)
	fmt.Println("⏳ Attente des résultats...")

	// Attendre et récupérer les résultats
	result, err := waitForResult(taskID)
	if err != nil {
		fmt.Printf("❌ Erreur lors de la récupération des résultats: %v\n", err)
		os.Exit(1)
	}

	// Afficher les résultats
	displayResults(result)
}

func createTask(url string, width, height int) (string, error) {
	// Préparer la requête
	taskReq := TaskRequest{
		CustomHeaders: make(map[string]string),
	}
	taskReq.WebPage.URL = url
	taskReq.WebPage.Width = width
	taskReq.WebPage.Height = height

	jsonData, err := json.Marshal(taskReq)
	if err != nil {
		return "", fmt.Errorf("erreur de sérialisation: %w", err)
	}

	// Déterminer l'URL cible
	targetURL := "https://api.ecoindex.fr/v1/tasks/ecoindexes/"

	// Créer la requête HTTP
	req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("erreur de création de requête: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Exécuter la requête
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erreur d'exécution de requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erreur de lecture de réponse: %w", err)
	}

	// Gérer l'erreur 429 (limite quotidienne atteinte)
	if resp.StatusCode == http.StatusTooManyRequests {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			if errResp.Detail.LatestResult.ID != "" {
				fmt.Printf("⚠️  Limite quotidienne atteinte pour %s\n", errResp.Detail.Host)
				fmt.Printf("📊 Affichage du dernier résultat disponible...\n\n")

				// Convertir le dernier résultat en TaskResult et l'afficher
				result := &TaskResult{
					ID:       errResp.Detail.LatestResult.ID,
					URL:      errResp.Detail.LatestResult.URL,
					Width:    errResp.Detail.LatestResult.Width,
					Height:   errResp.Detail.LatestResult.Height,
					Size:     errResp.Detail.LatestResult.Size,
					Nodes:    errResp.Detail.LatestResult.Nodes,
					Requests: errResp.Detail.LatestResult.Requests,
					Grade:    errResp.Detail.LatestResult.Grade,
					Score:    errResp.Detail.LatestResult.Score,
					Ges:      errResp.Detail.LatestResult.Ges,
					Water:    errResp.Detail.LatestResult.Water,
					Date:     errResp.Detail.LatestResult.Date,
				}
				displayResults(result)
				os.Exit(0)
			}
		}
		return "", fmt.Errorf("limite quotidienne atteinte: %s", string(body))
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("erreur API (status %d): %s", resp.StatusCode, string(body))
	}

	// Parser la réponse - l'API peut retourner soit un objet soit directement l'ID
	var taskResp TaskResponse
	if err := json.Unmarshal(body, &taskResp); err != nil {
		// Si le parsing échoue, essayer de parser comme une simple chaîne (ID direct)
		var taskID string
		if err2 := json.Unmarshal(body, &taskID); err2 != nil {
			return "", fmt.Errorf("erreur de parsing de réponse: %w (body: %s)", err, string(body))
		}
		return taskID, nil
	}

	return taskResp.ID, nil
}

func waitForResult(taskID string) (*TaskResult, error) {
	targetURL := fmt.Sprintf("https://api.ecoindex.fr/v1/ecoindexes/%s", taskID)

	maxAttempts := 60 // 60 tentatives = 2 minutes max
	for i := 0; i < maxAttempts; i++ {
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			return nil, fmt.Errorf("erreur de création de requête: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("erreur d'exécution de requête: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("erreur de lecture de réponse: %w", err)
		}

		// Si on obtient un 200, c'est que le résultat est prêt
		if resp.StatusCode == http.StatusOK {
			var result TaskResult
			if err := json.Unmarshal(body, &result); err != nil {
				return nil, fmt.Errorf("erreur de parsing de réponse: %w", err)
			}
			return &result, nil
		}

		// Si 404, la tâche n'est pas encore terminée
		if resp.StatusCode == http.StatusNotFound {
			time.Sleep(2 * time.Second)
			fmt.Print(".")
			continue
		}

		// Autre erreur
		return nil, fmt.Errorf("erreur API (status %d): %s", resp.StatusCode, string(body))
	}

	return nil, fmt.Errorf("timeout: l'analyse a pris trop de temps")
}

func displayResults(result *TaskResult) {
	fmt.Println("\n")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("                    RÉSULTATS ECOINDEX")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("🌐 URL analysée: %s\n", result.URL)
	fmt.Printf("📅 Date: %s\n", result.Date)
	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println("                    SCORE ECOINDEX")
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Printf("🏆 Note: %s\n", result.Grade)
	fmt.Printf("📊 Score: %.2f/100\n", result.Score)
	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println("                  IMPACT ENVIRONNEMENTAL")
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Printf("🌍 GES (gCO2e): %.2f\n", result.Ges)
	fmt.Printf("💧 Eau (cl): %.2f\n", result.Water)
	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println("                  MÉTRIQUES TECHNIQUES")
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Printf("📦 Taille: %.2f Ko\n", result.Size)
	fmt.Printf("🔢 Nœuds DOM: %d\n", result.Nodes)
	fmt.Printf("📡 Requêtes HTTP: %d\n", result.Requests)
	fmt.Printf("📐 Dimensions: %dx%d\n", result.Width, result.Height)
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	// Afficher des recommandations basées sur la note
	displayRecommendations(result.Grade)
}

func displayRecommendations(grade string) {
	fmt.Println("💡 RECOMMANDATIONS:")
	fmt.Println()

	switch grade {
	case "A":
		fmt.Println("✨ Excellent! Votre site est très performant sur le plan environnemental.")
		fmt.Println("   Continuez à maintenir ces bonnes pratiques.")
	case "B":
		fmt.Println("👍 Très bien! Votre site a un bon score environnemental.")
		fmt.Println("   Quelques optimisations mineures pourraient encore l'améliorer.")
	case "C":
		fmt.Println("👌 Bien. Votre site a un score correct.")
		fmt.Println("   Considérez les optimisations suivantes:")
		fmt.Println("   - Réduire le nombre de requêtes HTTP")
		fmt.Println("   - Optimiser les images et ressources")
		fmt.Println("   - Simplifier la structure DOM")
	case "D":
		fmt.Println("⚠️  Moyen. Votre site pourrait être amélioré.")
		fmt.Println("   Actions recommandées:")
		fmt.Println("   - Minimiser et compresser les ressources")
		fmt.Println("   - Réduire le nombre d'éléments DOM")
		fmt.Println("   - Limiter les requêtes tierces")
	case "E":
		fmt.Println("❌ Attention! Votre site a un impact environnemental élevé.")
		fmt.Println("   Actions prioritaires:")
		fmt.Println("   - Audit complet des ressources chargées")
		fmt.Println("   - Optimisation drastique des images")
		fmt.Println("   - Refactorisation de la structure HTML")
		fmt.Println("   - Mise en cache agressive")
	case "F":
		fmt.Println("🚨 Critique! Votre site a un très fort impact environnemental.")
		fmt.Println("   Refonte nécessaire avec focus sur:")
		fmt.Println("   - Architecture légère et performante")
		fmt.Println("   - Chargement différé des ressources")
		fmt.Println("   - Suppression des éléments non essentiels")
		fmt.Println("   - Optimisation complète du code")
	case "G":
		fmt.Println("💥 Très critique! Impact environnemental maximal.")
		fmt.Println("   Refonte complète urgente requise!")
	}
	fmt.Println()
}
