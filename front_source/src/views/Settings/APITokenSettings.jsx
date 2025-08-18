import { CopyAll } from '@mui/icons-material'
import {
  Box,
  Button,
  Card,
  Chip,
  Divider,
  IconButton,
  Input,
  Typography,
} from '@mui/joy'
import moment from 'moment'

import { useEffect, useState } from 'react'
import { useUserProfile } from '../../queries/UserQueries'
import { useNotification } from '../../service/NotificationProvider'
import {
  CreateLongLiveToken,
  DeleteLongLiveToken,
  GetLongLiveTokens,
} from '../../utils/Fetcher'
import { isPlusAccount } from '../../utils/Helpers'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import TextModal from '../Modals/Inputs/TextModal'

const APITokenSettings = () => {
  const { data: userProfile } = useUserProfile()
  const { showNotification } = useNotification()
  const [tokens, setTokens] = useState([])
  const [isGetTokenNameModalOpen, setIsGetTokenNameModalOpen] = useState(false)
  const [showTokenId, setShowTokenId] = useState(null)
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
    GetLongLiveTokens().then(resp => {
      resp.json().then(data => {
        setTokens(data.res)
      })
    })
  }, [])

  const handleSaveToken = name => {
    CreateLongLiveToken(name).then(resp => {
      if (resp.ok) {
        resp.json().then(data => {
          // add the token to the list:
          console.log(data)
          const newTokens = [...tokens]
          newTokens.push(data.res)
          setTokens(newTokens)
        })
      }
    })
  }

  return (
    <div className='grid gap-4 py-4' id='apitokens'>
      <Typography level='h3'>Access Token</Typography>
      <Divider />
      <Typography level='body-sm'>
        Create token to use with the API to update things that trigger task or
        chores
      </Typography>
      {!isPlusAccount(userProfile) && (
        <>
          <Chip variant='soft' color='warning'>
            Plus Feature
          </Chip>
          <Typography level='body-sm' color='warning' sx={{ mt: 1 }}>
            API tokens are not available in the Basic plan. Upgrade to Plus to
            generate API tokens for integrating with external systems and
            automating your tasks.
          </Typography>
        </>
      )}

      {tokens.map(token => (
        <Card key={token.token} className='p-4'>
          <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
            <Box>
              <Typography level='body-md'>{token.name}</Typography>
              <Typography level='body-xs'>
                {moment(token.createdAt).fromNow()}(
                {moment(token.createdAt).format('lll')})
              </Typography>
            </Box>
            <Box>
              <Button
                variant='outlined'
                color='primary'
                sx={{ mr: 1 }}
                onClick={() => {
                  if (showTokenId === token.id) {
                    setShowTokenId(null)
                    return
                  }

                  setShowTokenId(token.id)
                }}
              >
                {showTokenId === token?.id ? 'Hide' : 'Show'} Token
              </Button>

              <Button
                variant='outlined'
                color='danger'
                onClick={() => {
                  showConfirmation(
                    `Are you sure you want to remove ${token.name}?`,
                    'Remove Token',
                    () => {
                      DeleteLongLiveToken(token.id).then(resp => {
                        if (resp.ok) {
                          showNotification({
                            type: 'success',
                            title: 'Removed',
                            message: 'API token has been removed',
                          })
                          const newTokens = tokens.filter(
                            t => t.id !== token.id,
                          )
                          setTokens(newTokens)
                        }
                      })
                    },
                    'Remove',
                    'Cancel',
                    'danger',
                  )
                }}
              >
                Remove
              </Button>
            </Box>
          </Box>
          {showTokenId === token?.id && (
            <Box>
              <Input
                value={token.token}
                sx={{ width: '100%', mt: 2 }}
                readOnly
                endDecorator={
                  <IconButton
                    variant='outlined'
                    color='primary'
                    onClick={() => {
                      navigator.clipboard.writeText(token.token)
                      showNotification({
                        type: 'success',
                        message: 'Token copied to clipboard',
                      })
                      setShowTokenId(null)
                    }}
                  >
                    <CopyAll />
                  </IconButton>
                }
              />
            </Box>
          )}
        </Card>
      ))}

      <Button
        variant='soft'
        color='primary'
        disabled={!isPlusAccount(userProfile)}
        sx={{
          width: '210px',
          mb: 1,
        }}
        onClick={() => {
          setIsGetTokenNameModalOpen(true)
        }}
      >
        Generate New Token
      </Button>
      <TextModal
        isOpen={isGetTokenNameModalOpen}
        title='Give a name for your new token, something to remember it by.'
        onClose={() => {
          setIsGetTokenNameModalOpen(false)
        }}
        okText={'Generate Token'}
        onSave={handleSaveToken}
      />

      {/* Modals */}
      {confirmModalConfig?.isOpen && (
        <ConfirmationModal config={confirmModalConfig} />
      )}
    </div>
  )
}

export default APITokenSettings
