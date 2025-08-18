/* eslint-env node */
export const API_URL =
  import.meta.env.VITE_APP_API_URL === 'AUTO'
    ? `${window.location.hostname}/api`
    : import.meta.env.VITE_APP_API_URL
export const REDIRECT_URL = import.meta.env.VITE_APP_REDIRECT_URL //|| 'http://localhost:3000'
export const GOOGLE_CLIENT_ID = import.meta.env.VITE_APP_GOOGLE_CLIENT_ID
export const ENVIROMENT = import.meta.env.VITE_APP_ENVIROMENT
