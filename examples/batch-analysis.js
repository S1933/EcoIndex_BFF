/**
 * Exemple d'utilisation du système de tâches batch pour analyser un site web complet
 *
 * Ce script montre comment :
 * 1. Créer une tâche d'analyse batch
 * 2. Suivre la progression en temps réel
 * 3. Récupérer les résultats une fois l'analyse terminée
 */

const API_BASE_URL = 'http://localhost:3001/api';

/**
 * Crée une nouvelle tâche d'analyse batch
 * @param {string[]} urls - Liste des URLs à analyser
 * @param {number} width - Largeur de la fenêtre (défaut: 1920)
 * @param {number} height - Hauteur de la fenêtre (défaut: 1080)
 * @returns {Promise<string>} ID de la tâche créée
 */
async function createBatchTask(urls, width = 1920, height = 1080) {
  const response = await fetch(`${API_BASE_URL}/batch-tasks`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      urls,
      width,
      height
    })
  });

  if (!response.ok) {
    const error = await response.json();
    throw new Error(`Failed to create task: ${error.error}`);
  }

  const data = await response.json();
  return data.task.id;
}

/**
 * Récupère l'état actuel d'une tâche
 * @param {string} taskId - ID de la tâche
 * @returns {Promise<Object>} État de la tâche
 */
async function getTaskStatus(taskId) {
  const response = await fetch(`${API_BASE_URL}/batch-tasks/${taskId}`);

  if (!response.ok) {
    const error = await response.json();
    throw new Error(`Failed to get task status: ${error.error}`);
  }

  const data = await response.json();
  return data.task;
}

/**
 * Récupère les détails d'un résultat EcoIndex
 * @param {string} resultId - ID du résultat
 * @returns {Promise<Object>} Détails du résultat
 */
async function getEcoindexResult(resultId) {
  const response = await fetch(`${API_BASE_URL}/tasks/${resultId}`);

  if (!response.ok) {
    throw new Error(`Failed to get result: ${response.statusText}`);
  }

  return await response.json();
}

/**
 * Attend qu'une tâche soit terminée en utilisant le polling
 * @param {string} taskId - ID de la tâche
 * @param {Function} onProgress - Callback appelé à chaque mise à jour (optionnel)
 * @returns {Promise<Object>} Tâche terminée
 */
async function waitForTaskCompletion(taskId, onProgress = null) {
  return new Promise((resolve, reject) => {
    const pollInterval = 5000; // 5 secondes

    const poll = async () => {
      try {
        const task = await getTaskStatus(taskId);

        // Appeler le callback de progression si fourni
        if (onProgress) {
          onProgress(task);
        }

        // Vérifier si la tâche est terminée
        if (task.status === 'completed') {
          resolve(task);
        } else if (task.status === 'failed') {
          reject(new Error(`Task failed: ${task.error}`));
        } else {
          // Continuer le polling
          setTimeout(poll, pollInterval);
        }
      } catch (error) {
        reject(error);
      }
    };

    // Démarrer le polling
    poll();
  });
}

/**
 * Liste toutes les tâches actives
 * @returns {Promise<Object[]>} Liste des tâches
 */
async function listAllTasks() {
  const response = await fetch(`${API_BASE_URL}/batch-tasks`);

  if (!response.ok) {
    throw new Error(`Failed to list tasks: ${response.statusText}`);
  }

  const data = await response.json();
  return data.tasks;
}

/**
 * Supprime une tâche
 * @param {string} taskId - ID de la tâche à supprimer
 */
async function deleteTask(taskId) {
  const response = await fetch(`${API_BASE_URL}/batch-tasks/${taskId}`, {
    method: 'DELETE'
  });

  if (!response.ok) {
    throw new Error(`Failed to delete task: ${response.statusText}`);
  }
}

/**
 * Exemple d'utilisation : Analyser un site web complet
 */
async function analyzeWebsite() {
  console.log('🚀 Démarrage de l\'analyse du site web...\n');

  // Liste des URLs à analyser
  const urls = [
    'https://www.ecoindex.fr',
    'https://www.ecoindex.fr/comment-ca-marche/',
    'https://www.ecoindex.fr/a-propos/',
  ];

  try {
    // 1. Créer la tâche
    console.log('📝 Création de la tâche d\'analyse...');
    const taskId = await createBatchTask(urls);
    console.log(`✅ Tâche créée avec l'ID: ${taskId}\n`);

    // 2. Suivre la progression
    console.log('⏳ Analyse en cours...\n');
    const completedTask = await waitForTaskCompletion(taskId, (task) => {
      const progressBar = '█'.repeat(Math.floor(task.progress / 5)) +
                         '░'.repeat(20 - Math.floor(task.progress / 5));
      console.log(`[${progressBar}] ${task.progress}% - Page ${task.current_page}/${task.total_pages}`);
    });

    console.log('\n✅ Analyse terminée!\n');

    // 3. Récupérer et afficher les résultats
    console.log('📊 Résultats de l\'analyse:\n');
    for (let i = 0; i < completedTask.results.length; i++) {
      const resultId = completedTask.results[i];
      try {
        const result = await getEcoindexResult(resultId);
        console.log(`\n${i + 1}. ${urls[i]}`);
        console.log(`   Grade: ${result.grade || 'N/A'}`);
        console.log(`   Score: ${result.score || 'N/A'}`);
        console.log(`   ID: ${resultId}`);
      } catch (error) {
        console.log(`\n${i + 1}. ${urls[i]}`);
        console.log(`   ⚠️  Résultat non disponible: ${error.message}`);
      }
    }

    // 4. Nettoyer (optionnel)
    console.log('\n🧹 Nettoyage de la tâche...');
    await deleteTask(taskId);
    console.log('✅ Tâche supprimée\n');

  } catch (error) {
    console.error('❌ Erreur:', error.message);
    process.exit(1);
  }
}

/**
 * Exemple d'utilisation : Lister toutes les tâches en cours
 */
async function showActiveTasks() {
  console.log('📋 Tâches actives:\n');

  try {
    const tasks = await listAllTasks();

    if (tasks.length === 0) {
      console.log('Aucune tâche active');
      return;
    }

    tasks.forEach((task, index) => {
      console.log(`${index + 1}. ID: ${task.id}`);
      console.log(`   Statut: ${task.status}`);
      console.log(`   Progression: ${task.progress}%`);
      console.log(`   Pages: ${task.current_page}/${task.total_pages}`);
      console.log(`   Créée le: ${new Date(task.created_at).toLocaleString()}`);
      console.log('');
    });
  } catch (error) {
    console.error('❌ Erreur:', error.message);
  }
}

// Exécuter l'exemple si le script est lancé directement
if (require.main === module) {
  const command = process.argv[2];

  if (command === 'list') {
    showActiveTasks();
  } else {
    analyzeWebsite();
  }
}

// Exporter les fonctions pour utilisation dans d'autres modules
module.exports = {
  createBatchTask,
  getTaskStatus,
  getEcoindexResult,
  waitForTaskCompletion,
  listAllTasks,
  deleteTask
};
