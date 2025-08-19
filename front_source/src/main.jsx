import { QueryClient } from '@tanstack/react-query'
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.jsx'
import Contexts from './contexts/Contexts.jsx'
import './index.css'
import './i18n'

const queryClient = new QueryClient({})

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <React.Suspense fallback="loading">
      <Contexts queryClient={queryClient}>
        <App />
      </Contexts>
    </React.Suspense>
  </React.StrictMode>,
)
