import { Network } from '@capacitor/network'
import { localStore } from '../utils/LocalStore'
import { syncManager } from '../utils/SyncManager.jsx' // Ensure you import syncManager if needed for syncing
class NetworkManager {
  constructor() {
    this.isOnline = true
    this.isNetworkOn = null
    this.init()
    this.connectionStatusListeners = []
    this.queueSyncListeners = []
    this.lastChecked = null
    this.offlineSince = null
  }
  async init() {
    const status = await Network.getStatus()
    this.isNetworkOn = status.connected
    this.lastChecked = Date.now()

    Network.addListener('networkStatusChange', status => {
      if (this.isNetworkOn !== status.connected) {
        this.isNetworkOn = status.connected
        this.lastChecked = Date.now()
        this.isOnline = status.connected
      }
    })
    const syncQueue = () => {
      localStore
        .syncQueuedRequests()
        .then(hasMessages => {
          console.log(
            'Queued requests synced successfully. Queue has messaage is: ',
            hasMessages,
          )
          if (hasMessages) {
            this.notifyBackendSync()
          }
        })
        .catch(error => {
          console.error('Error syncing queued requests:', error)
        })
    }
    this.registerNetworkListener(async isOnline => {
      if (isOnline && this.isNetworkOn) {
        // TODO: Delete when Sync manager Implemented
        syncQueue()
        console.log('NetworkManager: Network is back online. with SYNCMANAGER')

        await syncManager.syncTasks()
        console.log('Finished syncing queued requests.')
      }
    })
    syncQueue()
  }

  setOffline() {
    if (this.isOnline === true) {
      this.isOnline = false
      this.notifyConnectionStatus()
      this.offlineSince = Date.now() // Record the time when we went offline
    }
  }
  setOnline() {
    if (this.isOnline === false) {
      this.isOnline = true
      this.notifyConnectionStatus()
    }
  }

  notifyConnectionStatus() {
    this.connectionStatusListeners.forEach(callback => {
      callback(this.isOnline)
    })
  }

  notifyBackendSync() {
    this.queueSyncListeners.forEach(callback => {
      callback()
    })
  }

  registerNetworkListener(callback) {
    this.connectionStatusListeners.push(callback)
  }
  registerBackendSyncListener(callback) {
    // if callback is not in the list already, add it
    if (!this.queueSyncListeners.includes(callback)) {
      this.queueSyncListeners.push(callback)
    }
  }
}

export const networkManager = new NetworkManager()
