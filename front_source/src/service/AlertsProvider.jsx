import { Alert, Box } from '@mui/joy'
import PropTypes from 'prop-types'
import { createContext, useCallback, useContext, useState } from 'react'
import Z_INDEX from '../constants/zIndex'

const FADE_DURATION = 400 // ms
const ALERT_DURATION = 5000 // ms

const AlertsContext = createContext()

// Helper function to create a delay
const delay = ms => new Promise(res => setTimeout(res, ms))

export const AlertsProvider = ({ children }) => {
  const [show, setShow] = useState(false)
  const [visibleAlert, setVisibleAlert] = useState(null)

  const showAlert = useCallback(async alertObj => {
    setVisibleAlert(alertObj)
    setShow(false)

    await delay(10)

    setShow(true)
    await delay(ALERT_DURATION)

    setShow(false)
    await delay(FADE_DURATION)

    setVisibleAlert(null)
  }, [])

  const hideAlert = useCallback(() => {
    setShow(false)
    // Wait for the fade out transition to complete before unmounting
    setTimeout(() => {
      setVisibleAlert(null)
    }, FADE_DURATION)
  }, [])

  return (
    <AlertsContext.Provider value={{ showAlert, hideAlert }}>
      {children}
      {visibleAlert && (
        <Box
          sx={{
            pt: `calc( env(safe-area-inset-top, 0px))`,
            position: 'fixed',
            top: 0,
            left: 0,
            width: '100%',
            zIndex: Z_INDEX.ALERTS,
            overflow: 'hidden',
          }}
        >
          <Alert
            variant='soft'
            color={visibleAlert.color || 'primary'}
            startDecorator={visibleAlert.icon}
            onClick={hideAlert}
            sx={{
              transition: `transform ${FADE_DURATION}ms ease-in-out, opacity ${FADE_DURATION}ms ease-in-out`,
              transform: show ? 'translateY(0)' : 'translateY(-100%)',
              opacity: show ? 1 : 0,
              pointerEvents: show ? 'auto' : 'none',
              width: '100%',
              justifyContent: 'center',
              alignItems: 'center',
              padding: '4px',
              fontSize: '10px',
              fontWeight: 'md',
            }}
          >
            {visibleAlert.message}
          </Alert>
        </Box>
      )}
    </AlertsContext.Provider>
  )
}

AlertsProvider.propTypes = {
  children: PropTypes.node.isRequired,
}

export const useAlerts = () => useContext(AlertsContext)
