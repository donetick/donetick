import { CapacitorSQLite } from '@capacitor-community/sqlite'

const CACHE_TABLE = 'offline_cache'
const QUEUE_TABLE = 'offline_request_queue'
const OFFLINE_TASK = 'offlineTasks' // For storing offline tasks

class LocalStore {
  constructor() {
    this.db = null
    // this.useLocalStorage = !Capacitor.isNativePlatform()
    this.useLocalStorage = true // default to localStorage for now.
  }

  async initDatabase() {
    if (this.useLocalStorage) return null
    if (this.db) return this.db

    const db = await CapacitorSQLite.createConnection({
      database: 'offline_data',
      version: 1,
    })
    await db.open()

    // Create tables if they don't exist
    await db.execute(`
      CREATE TABLE IF NOT EXISTS ${CACHE_TABLE} (
        key TEXT PRIMARY KEY,
        value TEXT,
        timestamp INTEGER
      );
      CREATE TABLE IF NOT EXISTS ${QUEUE_TABLE} (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        url TEXT,
        requestBody TEXT
      );
    `)

    this.db = db
    return db
  }

  async saveToCache(key, data) {
    const timestamp = Date.now() // Current timestamp in milliseconds

    if (this.useLocalStorage) {
      localStorage.setItem(key, JSON.stringify({ value: data, timestamp }))

      return
    }

    const db = await this.initDatabase()
    await db.run(
      `
      INSERT OR REPLACE INTO ${CACHE_TABLE} (key, value, timestamp)
      VALUES (?, ?, ?);
    `,
      [key, JSON.stringify(data), timestamp],
    )
  }

  // async saveTemporaryTask(task) {
  //   const baseURL = apiManager.getApiURL()
  //   const fullURL = `${baseURL}/chores/${task.tempId}`
  //   const options = {
  //     method: 'GET',
  //     headers: HEADERS(),
  //     url: fullURL,
  //   }
  //   const respond = { res: task }
  //   const requestId = murmurhash.v3(JSON.stringify({ fullURL, options }))

  //   if (this.useLocalStorage) {
  //     this.saveToCache(requestId, respond)
  //     return
  //   }
  //   const db = await this.initDatabase()

  //   await db.run(
  //     `
  //     INSERT INTO ${CACHE_TABLE} (url, requestBody)
  //     VALUES (?, ?);
  //   `,
  //     [
  //       requestId,
  //       JSON.stringify({
  //         url: fullURL,
  //         options: { method: 'GET', headers: HEADERS() },
  //       }),
  //     ],
  //   )
  //   console.log('Saved temporary task to queue:', task)
  //   return
  // }

  async getFromCache(key, ttl = 0) {
    const now = Date.now()

    if (this.useLocalStorage) {
      const cachedItem = localStorage.getItem(key)
      if (!cachedItem) return null

      const { value, timestamp } = JSON.parse(cachedItem)
      if (ttl > 0 && now - timestamp > ttl) {
        localStorage.removeItem(key) // Remove expired item
        return null
      }
      return value
    }

    const db = await this.initDatabase()
    const result = await db.query(
      `
      SELECT value, timestamp FROM ${CACHE_TABLE} WHERE key = ?;
    `,
      [key],
    )

    if (result.values.length === 0) return null

    const { value, timestamp } = result.values[0]
    if (ttl > 0 && now - timestamp > ttl) {
      // Remove expired item
      await db.run(`DELETE FROM ${CACHE_TABLE} WHERE key = ?;`, [key])
      return null
    }

    return JSON.parse(value)
  }

  async cleanExpiredCache(ttl) {
    const now = Date.now()

    if (this.useLocalStorage) {
      const keys = Object.keys(localStorage)
      for (const key of keys) {
        const cachedItem = localStorage.getItem(key)
        if (!cachedItem) continue

        const { timestamp } = JSON.parse(cachedItem)
        if (now - timestamp > ttl) {
          localStorage.removeItem(key) // Remove expired item
        }
      }
      return
    }

    const db = await this.initDatabase()
    await db.run(
      `
      DELETE FROM ${CACHE_TABLE} WHERE ? - timestamp > ?;
    `,
      [now, ttl],
    )
  }

  async queueRequest(requestId, requestPayload) {
    if (this.useLocalStorage) {
      const queue = JSON.parse(localStorage.getItem(QUEUE_TABLE)) || []
      console.log('requestPayload', requestPayload)

      if (typeof requestPayload?.options?.body['id'] === 'string') {
        requestPayload['id'] = null
      }
      queue.push({ requestId, requestBody: requestPayload })
      localStorage.setItem(QUEUE_TABLE, JSON.stringify(queue))
      return
    }

    const db = await this.initDatabase()
    await db.run(
      `
      INSERT INTO ${QUEUE_TABLE} (url, requestBody)
      VALUES (?, ?);
    `,
      [requestId, JSON.stringify(requestPayload)],
    )
  }

  async syncQueuedRequests() {
    console.log('Syncing queued requests...')

    var queueSize = 0
    if (this.useLocalStorage) {
      const queue = JSON.parse(localStorage.getItem(QUEUE_TABLE)) || []
      console.log('LocalStore: queue: ', queue)
      queueSize = queue.length
      for (const request of queue) {
        try {
          await fetch(request.requestBody.url, request.requestBody.options)
          console.log('LocalStore: Synced request:', request)
        } catch (error) {
          console.error('LocalStore: Failed to sync request:', request, error)
        }
      }

      // Clear the queue after syncing
      localStorage.removeItem(QUEUE_TABLE)
      localStorage.removeItem(OFFLINE_TASK)
      return queueSize > 0
    }

    const db = await this.initDatabase()
    const result = await db.query(`SELECT * FROM ${QUEUE_TABLE};`)
    queueSize = result.values.length
    for (const request of result.values) {
      try {
        await fetch(
          request.requestBody.url,
          JSON.parse(request.requestBody.options),
        )
        console.log('Synced request:', request)
      } catch (error) {
        console.error('Failed to sync request:', request, error)
      }
    }

    // Clear the queue after syncing
    await db.run(`DELETE FROM ${QUEUE_TABLE};`)
    return queueSize > 0
  }
}

export const localStore = new LocalStore()
