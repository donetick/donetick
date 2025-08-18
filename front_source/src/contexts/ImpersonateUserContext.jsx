import { createContext, useContext, useState } from 'react'

const ImpersonateUserContext = createContext()

export const useImpersonateUser = () => useContext(ImpersonateUserContext)

export const ImpersonateUserProvider = ({ children }) => {
  const [impersonatedUser, setImpersonatedUser] = useState(null)
  return (
    <ImpersonateUserContext.Provider
      value={{ impersonatedUser, setImpersonatedUser }}
    >
      {children}
    </ImpersonateUserContext.Provider>
  )
}
