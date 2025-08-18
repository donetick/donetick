import { createContext, useContext } from 'react'
import { useWebSocket } from '../hooks/useWebSocket'

const WebSocketContext = createContext({
  connectionState: 3, // CLOSED
  isConnected: false,
  isConnecting: false,
  lastEvent: null,
  error: null,
  connect: () => {},
  disconnect: () => {},
  getConnectionStatus: () => 'disconnected',
})

export const useWebSocketContext = () => {
  return useContext(WebSocketContext)
}

export const WebSocketProvider = ({ children }) => {
  const webSocketState = useWebSocket()

  return (
    <WebSocketContext.Provider value={webSocketState}>
      {children}
    </WebSocketContext.Provider>
  )
}

export default WebSocketProvider
