import { useContext } from 'react'
import { SSEContext } from '../contexts/SSEContext'

export const useSSEContext = () => {
  return useContext(SSEContext)
}
