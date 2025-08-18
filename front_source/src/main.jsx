import { QueryClient } from '@tanstack/react-query'
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.jsx'
import Contexts from './contexts/Contexts.jsx'
import './index.css'

const queryClient = new QueryClient({})

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <Contexts queryClient={queryClient}>
      <App />
    </Contexts>
  </React.StrictMode>,
)
