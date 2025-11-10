# Système de Tâches Batch

Ce document décrit le système de tâches batch pour l'analyse de sites web complets avec EcoIndex.

## Vue d'ensemble

Le système de tâches batch permet de lancer des analyses complètes sur plusieurs pages d'un site web sans perdre le traitement en cas de rafraîchissement du navigateur. Les tâches sont stockées dans Redis et traitées en arrière-plan.

## Architecture

### Composants

1. **Models** ([`models/task.go`](../models/task.go))
   - Définit les structures de données pour les tâches
   - Statuts possibles : `pending`, `processing`, `completed`, `failed`

2. **Services** ([`services/task.go`](../services/task.go))
   - Gestion du stockage Redis des tâches
   - CRUD des tâches (Create, Read, Update, Delete)
   - TTL de 7 jours pour les tâches

3. **Handlers** ([`handler/batch_task.go`](../handler/batch_task.go))
   - Endpoints API REST
   - Traitement asynchrone des tâches
   - Limitation à 3 requêtes simultanées vers l'API EcoIndex

## API Endpoints

### 1. Créer une tâche batch

Crée une nouvelle tâche d'analyse pour une ou plusieurs URLs.

```http
POST /api/batch-tasks
Content-Type: application/json

{
  "url": "https://www.example.com",
  "urls": [
    "https://www.example.com",
    "https://www.example.com/about",
    "https://www.example.com/contact"
  ],
  "width": 1920,
  "height": 1080
}
```

**Paramètres :**
- `url` (string, optionnel) : URL unique à analyser
- `urls` (array, optionnel) : Liste d'URLs à analyser
- `width` (int, optionnel) : Largeur de la fenêtre du navigateur (défaut: 1920)
- `height` (int, optionnel) : Hauteur de la fenêtre du navigateur (défaut: 1080)

**Réponse (201 Created) :**
```json
{
  "task": {
    "id": "a7c3d264-62c6-4f45-b1db-51d7db31d085",
    "url": "https://www.example.com",
    "width": 1920,
    "height": 1080,
    "status": "pending",
    "progress": 0,
    "total_pages": 3,
    "current_page": 0,
    "results": [],
    "created_at": "2024-01-10T10:00:00Z",
    "updated_at": "2024-01-10T10:00:00Z"
  }
}
```

### 2. Récupérer l'état d'une tâche

Permet de suivre la progression d'une tâche en cours.

```http
GET /api/batch-tasks/{id}
```

**Réponse (200 OK) :**
```json
{
  "task": {
    "id": "a7c3d264-62c6-4f45-b1db-51d7db31d085",
    "url": "https://www.example.com",
    "width": 1920,
    "height": 1080,
    "status": "processing",
    "progress": 66,
    "total_pages": 3,
    "current_page": 2,
    "results": [
      "result-id-1",
      "result-id-2"
    ],
    "created_at": "2024-01-10T10:00:00Z",
    "updated_at": "2024-01-10T10:02:30Z"
  }
}
```

### 3. Lister toutes les tâches

Récupère la liste de toutes les tâches actives.

```http
GET /api/batch-tasks
```

**Réponse (200 OK) :**
```json
{
  "tasks": [
    {
      "id": "task-1",
      "status": "completed",
      "progress": 100,
      ...
    },
    {
      "id": "task-2",
      "status": "processing",
      "progress": 50,
      ...
    }
  ],
  "total": 2
}
```

### 4. Supprimer une tâche

Supprime une tâche de Redis.

```http
DELETE /api/batch-tasks/{id}
```

**Réponse (204 No Content)**

## Statuts des tâches

| Statut | Description |
|--------|-------------|
| `pending` | Tâche créée, en attente de traitement |
| `processing` | Tâche en cours de traitement |
| `completed` | Tâche terminée avec succès |
| `failed` | Tâche échouée |

## Utilisation depuis une application Node.js

### Exemple : Créer et suivre une tâche

```javascript
// 1. Créer une tâche
async function createAnalysisTask(urls) {
  const response = await fetch('http://localhost:3001/api/batch-tasks', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      urls: urls,
      width: 1920,
      height: 1080
    })
  });

  const data = await response.json();
  return data.task.id;
}

// 2. Suivre la progression avec polling
async function pollTaskStatus(taskId) {
  const response = await fetch(`http://localhost:3001/api/batch-tasks/${taskId}`);
  const data = await response.json();
  return data.task;
}

// 3. Utilisation complète
async function analyzeWebsite(urls) {
  // Créer la tâche
  const taskId = await createAnalysisTask(urls);
  console.log('Tâche créée:', taskId);

  // Polling toutes les 5 secondes
  const interval = setInterval(async () => {
    const task = await pollTaskStatus(taskId);
    console.log(`Progression: ${task.progress}% (${task.current_page}/${task.total_pages})`);

    if (task.status === 'completed') {
      clearInterval(interval);
      console.log('Analyse terminée!');
      console.log('Résultats:', task.results);
    } else if (task.status === 'failed') {
      clearInterval(interval);
      console.error('Analyse échouée:', task.error);
    }
  }, 5000);
}

// Exemple d'utilisation
analyzeWebsite([
  'https://www.example.com',
  'https://www.example.com/about',
  'https://www.example.com/contact'
]);
```

### Exemple : React Hook pour suivre une tâche

```javascript
import { useState, useEffect } from 'react';

function useTaskStatus(taskId) {
  const [task, setTask] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!taskId) return;

    const fetchTask = async () => {
      try {
        const response = await fetch(`http://localhost:3001/api/batch-tasks/${taskId}`);
        const data = await response.json();
        setTask(data.task);
        setLoading(false);

        // Continuer le polling si la tâche n'est pas terminée
        if (data.task.status === 'processing' || data.task.status === 'pending') {
          setTimeout(fetchTask, 5000);
        }
      } catch (err) {
        setError(err);
        setLoading(false);
      }
    };

    fetchTask();
  }, [taskId]);

  return { task, loading, error };
}

// Utilisation dans un composant
function TaskMonitor({ taskId }) {
  const { task, loading, error } = useTaskStatus(taskId);

  if (loading) return <div>Chargement...</div>;
  if (error) return <div>Erreur: {error.message}</div>;
  if (!task) return null;

  return (
    <div>
      <h2>Analyse en cours</h2>
      <div>Statut: {task.status}</div>
      <div>Progression: {task.progress}%</div>
      <div>Pages: {task.current_page}/{task.total_pages}</div>
      <progress value={task.progress} max="100" />

      {task.status === 'completed' && (
        <div>
          <h3>Résultats disponibles:</h3>
          <ul>
            {task.results.map(resultId => (
              <li key={resultId}>
                <a href={`/results/${resultId}`}>{resultId}</a>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
```

## Limitations et considérations

### Limites de l'API EcoIndex

- **10 requêtes par jour par domaine** : L'API EcoIndex limite le nombre d'analyses par domaine
- Si la limite est atteinte, utilisez l'endpoint [`/api/results`](../README.md#get-latest-results-info) pour récupérer les derniers résultats disponibles

### Performance

- **3 requêtes simultanées maximum** : Pour éviter de surcharger l'API EcoIndex
- **Timeout de 5 minutes par URL** : Chaque analyse a un timeout de 5 minutes
- **TTL de 7 jours** : Les tâches sont automatiquement supprimées après 7 jours

### Recommandations

1. **Polling intelligent** : Utilisez un intervalle de 5 secondes pour le polling
2. **Gestion des erreurs** : Vérifiez toujours le champ `error` dans la réponse
3. **Nettoyage** : Supprimez les tâches terminées avec `DELETE /api/batch-tasks/{id}`
4. **Persistance** : Stockez les IDs de tâches dans votre base de données pour permettre la reprise après un crash

## Stockage Redis

Les tâches sont stockées dans Redis avec les clés suivantes :

- `task:{id}` : Données de la tâche individuelle
- `tasks:list` : Liste des IDs de toutes les tâches actives

Le TTL est de 7 jours pour toutes les clés.

## Dépannage

### La tâche reste bloquée en "pending"

Vérifiez les logs du serveur pour voir si le traitement en arrière-plan a démarré.

### La tâche échoue systématiquement

- Vérifiez que l'API EcoIndex est accessible
- Vérifiez que vous n'avez pas atteint la limite de 10 requêtes par jour
- Consultez le champ `error` de la tâche pour plus de détails

### Les résultats ne sont pas disponibles

Les résultats sont des IDs de tâches EcoIndex. Utilisez l'endpoint [`/api/tasks/{id}`](../README.md#get-the-result-of-a-task) pour récupérer les détails de chaque résultat.

## Exemple complet d'intégration

Voir le fichier [`examples/batch-analysis.js`](../examples/batch-analysis.js) pour un exemple complet d'intégration avec une application Node.js.
