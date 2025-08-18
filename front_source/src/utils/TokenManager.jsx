import { Preferences } from '@capacitor/preferences'
import Cookies from 'js-cookie'
import murmurhash from 'murmurhash'
import { API_URL } from '../Config'
import { networkManager } from '../hooks/NetworkManager'
import { RefreshToken } from './Fetcher'
import { localStore } from './LocalStore'

class ApiManager {
  constructor() {
    this.customServerURL = `${API_URL}/api/v1`
    this.initialized = false
    this.navigateToLogin = () => {}
  }

  async init() {
    if (this.initialized) {
      return
    }

    const { value: serverURL } = await Preferences.get({
      key: 'customServerUrl',
    })

    this.customServerURL = `${serverURL || API_URL}/api/v1`
    await localStore.initDatabase()
    this.initialized = true
  }

  getApiURL() {
    return this.customServerURL
  }

  updateApiURL(url) {
    this.customServerURL = url
    this.init()
  }
  setNavigateToLogin(callback) {
    this.navigateToLogin = callback
  }
}

export const apiManager = new ApiManager()

export const getAssetURL = path => {
  const baseURL = apiManager.getApiURL()
  return `${baseURL}/assets/${path}`
}
export async function UploadFile(url, options) {
  if (!isTokenValid()) {
    Cookies.set('ca_redirect', window.location.pathname)
    window.location.href = '/login'
  }

  if (!options) {
    options = {}
  }
  const headers = HEADERS()
  options.headers = { Authorization: headers['Authorization'] }

  const baseURL = apiManager.getApiURL()
  const fullURL = `${baseURL}${url}`

  return fetch(fullURL, options)
}

export async function Fetch(url, options) {
  if (!isTokenValid()) {
    Cookies.set('ca_redirect', window.location.pathname)
    if (!window.location.pathname === '/login') {
      window.location.href = '/login'
    }
  }

  if (!options) {
    options = {}
  }
  // clone options to avoid mutation
  options.headers = { ...options.headers, ...HEADERS() }

  const baseURL = apiManager.getApiURL()
  const fullURL = `${baseURL}${url}`

  // const networkStatus = await Network.getStatus()

  // if (!networkStatus.connected) {
  //   return handleOfflineRequest(fullURL, options)
  // }

  // Online: Perform the fetch
  try {
    const response = await fetch(fullURL, options)

    if (response.ok) {
      const data = await response.clone().json()
      const optionsHash = murmurhash.v3(JSON.stringify(options))
      await localStore.saveToCache(fullURL + optionsHash, data)
      networkManager.setOnline()
    } else if (response.status === 401) {
      // Handle 401 Unauthorized
      const errorData = await response.json()
      console.error('Unauthorized:', errorData)
      localStorage.removeItem('ca_token')
      localStorage.removeItem('ca_expiration')
      apiManager.navigateToLogin()
    } else if (
      response.status === 503 ||
      response.type === 'opaque' ||
      response.status === 0
    ) {
      networkManager.setOffline()
      return handleOfflineRequest(fullURL, options)
    }
    // return promise that resolves to response object:
    return Promise.resolve(response)
  } catch (error) {
    networkManager.setOffline()
    console.error('Fetch error:', error)
    // throw error
    return handleOfflineRequest(fullURL, options)
  }
}

export const HEADERS = () => {
  return {
    'Content-Type': 'application/json',
    Authorization: 'Bearer ' + localStorage.getItem('ca_token'),
  }
}

export const isTokenValid = () => {
  const expiration = localStorage.getItem('ca_expiration')
  const token = localStorage.getItem('ca_token')

  if (token) {
    const now = new Date()
    const expire = new Date(expiration)
    if (now < expire) {
      if (now.getTime() + 24 * 60 * 60 * 1000 > expire.getTime()) {
        refreshAccessToken()
      }
      return true
    } else {
      localStorage.removeItem('ca_token')
      localStorage.removeItem('ca_expiration')
    }
    return false
  }
}

export const refreshAccessToken = () => {
  RefreshToken().then(res => {
    if (res.status === 200) {
      res.json().then(data => {
        localStorage.setItem('ca_token', data.token)
        localStorage.setItem('ca_expiration', data.expire)
      })
    } else {
      return res.json().then(error => {
        console.log(error)
      })
    }
  })
}

async function handleOfflineRequest(url, options) {
  // if get request then attempt to fetch from cache otherewise queue it :
  if (options.method === 'GET') {
    return attemptFetchFromCache(url, options)
  } else {
    // Queue the request for later processing
    const requestId = murmurhash.v3(JSON.stringify({ url, options }))
    await localStore.queueRequest(requestId, { url, options })
    console.log('Request queued for later processing:', requestId)
    return Promise.reject({
      error: 'Offline and request queued',
      requestId,
      queued: true,
    })
  }
}
async function attemptFetchFromCache(url, options) {
  const optionsHash = murmurhash.v3(JSON.stringify(options))
  const cachedData = await localStore.getFromCache(url + optionsHash)
  networkManager.setOffline()

  if (cachedData) {
    return Promise.resolve({
      ok: true,
      status: 200,
      json: async () => cachedData,
    })
  } else {
    // TODO: change this to throw error instead of returning promise
    return Promise.reject(
      new Error(
        'No cached data found for URL: ' +
          url +
          ' with options hash: ' +
          optionsHash,
      ),
    )
  }
}
