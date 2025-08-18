import { CreateChore, SaveChore } from './Fetcher'
import { localStore } from './LocalStore'

class SyncManager {
  async syncTasks() {
    console.log('SYNCMANAGER: Starting sync process for offline tasks.')
    const offlineTasks = (await localStore.getFromCache('offlineTasks')) || []
    for (const task of offlineTasks) {
      // if task.needSync then it's need to be created:
      var resp
      if (task.needSync) {
        resp = await CreateChore(task)
      } else {
        resp = await SaveChore(task)
      }
      if (!resp.ok) {
        console.log(
          `SYNCMANAGER: Failed to sync task with id: ${task.id}. Error: ${resp.statusText}`,
        )
      } else {
        console.log(
          `SYNCMANAGER: Successfully synced task with id: ${task.id}.`,
        )
        console.log(`SYNCMANAGER: Response:`, resp)
      }
    }
    // Clear the offline tasks cache after syncing
    await localStore.saveToCache('offlineTasks', [])
    return true
  }
}

export const syncManager = new SyncManager()
