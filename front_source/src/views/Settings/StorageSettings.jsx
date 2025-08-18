import { Capacitor } from '@capacitor/core'
import {
  Button,
  Card,
  Chip,
  Divider,
  LinearProgress,
  Typography,
} from '@mui/joy'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useUserProfile } from '../../queries/UserQueries'
import { GetStorageUsage } from '../../utils/Fetcher'
import { isPlusAccount } from '../../utils/Helpers'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'

const StorageSettings = () => {
  const Navigate = useNavigate()
  const { data: userProfile } = useUserProfile()
  const [usage, setUsage] = useState({ used: 0, total: 0 })
  const [loading, setLoading] = useState(true)
  const [confirmModalConfig, setConfirmModalConfig] = useState({})

  const showConfirmation = (
    message,
    title,
    onConfirm,
    confirmText = 'Confirm',
    cancelText = 'Cancel',
    color = 'primary',
  ) => {
    setConfirmModalConfig({
      isOpen: true,
      message,
      title,
      confirmText,
      cancelText,
      color,
      onClose: isConfirmed => {
        if (isConfirmed) {
          onConfirm()
        }
        setConfirmModalConfig({})
      },
    })
  }

  useEffect(() => {
    if (isPlusAccount(userProfile)) {
      GetStorageUsage().then(resp => {
        resp.json().then(data => {
          setUsage(data.res)
          setLoading(false)
        })
      })
    }
  }, [userProfile])

  const percent =
    usage.total > 0 ? Math.round((usage.used / usage.total) * 100) : 0
  const usedMB = (usage.used / (1024 * 1024)).toFixed(2)
  const totalMB = (usage.total / (1024 * 1024)).toFixed(2)

  return (
    <div className='grid gap-4 py-4' id='storage'>
      <Typography level='h3'>Storage Settings</Typography>
      <Divider />
      <Card className='p-4' sx={{ maxWidth: 500, mb: 2 }}>
        <Typography level='title-md' sx={{ mb: 1 }}>
          Server Storage Usage
          {!isPlusAccount(userProfile) && (
            <Chip variant='soft' color='warning' sx={{ ml: 1 }}>
              Plus Feature
            </Chip>
          )}
        </Typography>
        <Typography level='body-sm' sx={{ mb: 1 }}>
          This is the storage used by your account on our servers (e.g. files,
          images, and data you have uploaded).
        </Typography>
        {!isPlusAccount(userProfile) ? (
          <>
            <LinearProgress
              determinate
              value={0}
              sx={{
                mb: 1,
                opacity: 0.4,
                '& .MuiLinearProgress-bar': {
                  backgroundColor: 'var(--joy-palette-neutral-400)',
                },
              }}
            />
            <Typography level='body-xs' sx={{ opacity: 0.6, mb: 1 }}>
              -- MB used / -- MB total (--)
            </Typography>
            <Typography level='body-sm' color='warning'>
              Server storage monitoring is not available in the Basic plan.
              Upgrade to Plus to track your server storage usage.
            </Typography>
          </>
        ) : loading ? (
          <>
            <LinearProgress sx={{ mb: 1 }} />
            <Typography level='body-xs'>Loading...</Typography>
          </>
        ) : (
          <>
            <LinearProgress determinate value={percent} sx={{ mb: 1 }} />
            <Typography level='body-xs'>
              {usedMB} MB used / {totalMB} MB total ({percent}%)
            </Typography>
          </>
        )}
      </Card>
      <Card className='p-4' sx={{ maxWidth: 500, mb: 2 }}>
        <Typography level='title-md' sx={{ mb: 1 }}>
          {Capacitor.isNativePlatform() ? 'App' : 'Browser'} Local Storage &
          Cache
        </Typography>
        <Typography level='body-sm' sx={{ mb: 1 }}>
          This is data stored locally in your browser for faster access and
          offline use. Clearing this will not affect your server data, but may
          log you out or remove offline tasks.
        </Typography>
        <Button
          variant='soft'
          color='danger'
          onClick={() => {
            showConfirmation(
              'Are you sure you want to clear your local storage and cache? This will remove all your data from this browser and require login.',
              'Clear All Local Storage',
              () => {
                localStorage.clear()
                Navigate('/login')
              },
              'Clear All',
              'Cancel',
              'danger',
            )
          }}
        >
          Clear All Local Storage and Cache
        </Button>
        <Button
          variant='outlined'
          color='danger'
          onClick={() => {
            showConfirmation(
              'Are you sure you want to clear only the offline cache and tasks?',
              'Clear Offline Cache',
              () => {
                localStorage.removeItem('offline_cache')
                localStorage.removeItem('offline_request_queue')
                localStorage.removeItem('offlineTasks')
              },
              'Clear Cache',
              'Cancel',
              'danger',
            )
          }}
          sx={{ mt: 1 }}
        >
          Clear Offline Cache and Offline Tasks
        </Button>
      </Card>

      {/* Modals */}
      {confirmModalConfig?.isOpen && (
        <ConfirmationModal config={confirmModalConfig} />
      )}
    </div>
  )
}

export default StorageSettings
