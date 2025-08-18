import { createContext, useContext } from 'react'
import { useSSE } from '../hooks/useSSE'

export const SSEContext = createContext({
  connectionState: 2, // CLOSED
  isConnected: false,
  isConnecting: false,
  lastEvent: null,
  error: null,
  connect: () => {},
  disconnect: () => {},
  getConnectionStatus: () => 'disconnected',
})

export const useSSEContext = () => {
  return useContext(SSEContext)
}

export const SSEProvider = ({ children }) => {
  const sseState = useSSE()

  return <SSEContext.Provider value={sseState}>{children}</SSEContext.Provider>
}

export default SSEProvider
