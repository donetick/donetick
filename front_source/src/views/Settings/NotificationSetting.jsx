import { Capacitor } from '@capacitor/core'
import { LocalNotifications } from '@capacitor/local-notifications'
import { Preferences } from '@capacitor/preferences'
import {
  Box,
  Button,
  Card,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  Input,
  Option,
  Select,
  Switch,
  Typography,
} from '@mui/joy'
import { useEffect, useState } from 'react'

import { useUserProfile } from '../../queries/UserQueries'
import { useNotification } from '../../service/NotificationProvider'
import {
  UpdateNotificationTarget,
  UpdateUserDetails,
} from '../../utils/Fetcher'

const NotificationSetting = () => {
  const { showWarning } = useNotification()
  const { data: userProfile, refetch: refetchUserProfile } = useUserProfile()

  const getNotificationPreferences = async () => {
    const ret = await Preferences.get({ key: 'notificationPreferences' })
    return JSON.parse(ret.value)
  }
  const setNotificationPreferences = async value => {
    if (value.granted === false) {
      await Preferences.set({
        key: 'notificationPreferences',
        value: JSON.stringify({ granted: false }),
      })
      return
    }
    const currentSettings = await getNotificationPreferences()
    await Preferences.set({
      key: 'notificationPreferences',
      value: JSON.stringify({ ...currentSettings, ...value }),
    })
  }

  const getPushNotificationPreferences = async () => {
    const ret = await Preferences.get({ key: 'pushNotificationPreferences' })
    return JSON.parse(ret.value)
  }

  const setPushNotificationPreferences = async value => {
    await Preferences.set({
      key: 'pushNotificationPreferences',
      value: JSON.stringify(value),
    })
  }

  const [deviceNotification, setDeviceNotification] = useState(false)

  const [dueNotification, setDueNotification] = useState(true)
  const [preDueNotification, setPreDueNotification] = useState(false)
  const [naggingNotification, setNaggingNotification] = useState(false)
  const [pushNotification, setPushNotification] = useState(false)

  useEffect(() => {
    getNotificationPreferences().then(resp => {
      if (resp) {
        setDeviceNotification(Boolean(resp.granted))
        setDueNotification(Boolean(resp.dueNotification ?? true))
        setPreDueNotification(Boolean(resp.preDueNotification))
        setNaggingNotification(Boolean(resp.naggingNotification))
      }
    })
    getPushNotificationPreferences().then(resp => {
      if (resp) {
        setPushNotification(Boolean(resp.granted))
      }
    })
  }, [])

  const [notificationTarget, setNotificationTarget] = useState(
    userProfile?.notification_target
      ? String(userProfile.notification_target.type)
      : '0',
  )

  const [chatID, setChatID] = useState(
    userProfile?.notification_target?.target_id ?? 0,
  )
  const [error, setError] = useState('')
  const SaveValidation = () => {
    switch (notificationTarget) {
      case '1':
        if (chatID === '') {
          setError('Chat ID is required')
          return false
        } else if (isNaN(chatID) || chatID === '0') {
          setError('Invalid Chat ID')
          return false
        }
        break
      case '2':
        if (chatID === '') {
          setError('User key is required')
          return false
        }
        break
      default:
        break
    }
    setError('')
    return true
  }
  const handleSave = () => {
    if (!SaveValidation()) return

    UpdateNotificationTarget({
      target: chatID,
      type: Number(notificationTarget),
    }).then(resp => {
      if (resp.status != 200) {
        alert(`Error while updating notification target: ${resp.statusText}`)
        return
      }

      refetchUserProfile()
      alert('Notification target updated')
    })
  }
  return (
    <div className='grid gap-4 py-4' id='notifications'>
      <Typography level='h3'>Device Notification</Typography>
      <Divider />
      <Typography level='body-md'>Manage your Device Notificaiton</Typography>

      <FormControl orientation='horizontal'>
        <Switch
          disabled={!Capacitor.isNativePlatform()}
          checked={deviceNotification}
          onClick={event => {
            event.preventDefault()
            if (deviceNotification === false) {
              LocalNotifications.requestPermissions().then(resp => {
                if (resp.display === 'granted') {
                  setDeviceNotification(true)
                  setNotificationPreferences({ granted: true })
                } else if (resp.display === 'denied') {
                  showWarning({
                    title: 'Notification Permission Denied',
                    message:
                      'You have denied notification permissions. You can enable them later in your device settings.',
                  })
                  setDeviceNotification(false)
                  setNotificationPreferences({ granted: false })
                }
              })
            } else {
              setDeviceNotification(false)
            }
          }}
          color={deviceNotification ? 'success' : 'neutral'}
          variant={deviceNotification ? 'solid' : 'outlined'}
          slotProps={{
            endDecorator: {
              sx: {
                minWidth: 24,
              },
            },
          }}
          sx={{ mr: 2 }}
        />
        <div>
          <FormLabel>Device Notification</FormLabel>
          <FormHelperText sx={{ mt: 0 }}>
            {Capacitor.isNativePlatform()
              ? 'Receive notification on your device when a task is due'
              : 'This feature is only available on mobile devices'}{' '}
          </FormHelperText>
        </div>
      </FormControl>
      {deviceNotification && (
        <Card>
          {[
            {
              title: 'Due Date Notification',
              checked: dueNotification,
              set: setDueNotification,
              label: 'Notification when the task is due',
              property: 'dueNotification',
              disabled: false,
            },
            {
              title: 'Pre-Due Date Notification',
              checked: preDueNotification,
              set: setPreDueNotification,
              label: 'Notification a few hours before the task is due',
              property: 'preDueNotification',
              disabled: false,
            },
            {
              title: 'Overdue Notification',
              checked: naggingNotification,
              set: setNaggingNotification,
              label: 'Notification when the task is overdue',
              property: 'naggingNotification',
              disabled: false,
            },
          ].map(item => (
            <FormControl
              key={item.property}
              orientation='horizontal'
              sx={{ width: 385, justifyContent: 'space-between' }}
            >
              <div>
                <FormLabel>{item.title}</FormLabel>
                <FormHelperText sx={{ mt: 0 }}>{item.label} </FormHelperText>
              </div>

              <Switch
                checked={item.checked}
                disabled={item.disabled}
                onClick={() => {
                  setNotificationPreferences({ [item.property]: !item.checked })
                  item.set(!item.checked)
                }}
                color={item.checked ? 'success' : ''}
                variant='solid'
                endDecorator={item.checked ? 'On' : 'Off'}
                slotProps={{ endDecorator: { sx: { minWidth: 24 } } }}
              />
            </FormControl>
          ))}
        </Card>
      )}
      {/* <FormControl
      orientation="horizontal"
      sx={{ width: 400, justifyContent: 'space-between' }}
    >
      <div>
        <FormLabel>Push Notifications</FormLabel>
        <FormHelperText sx={{ mt: 0 }}>{Capacitor.isNativePlatform()? 'Receive push notification when someone complete task' : 'This feature is only available on mobile devices'} </FormHelperText>
      </div>
      <Switch
      disabled={!Capacitor.isNativePlatform()}
        checked={pushNotification}
        onClick={(event) =>{
          event.preventDefault()
          if (pushNotification === false){
            PushNotifications.requestPermissions().then((resp) => {
              console.log("user PushNotifications permission",resp);
              if (resp.receive === 'granted') {

                setPushNotification(true)
                setPushNotificationPreferences({granted: true})
              }
              if (resp.receive !== 'granted') {
                showWarning({
                  title: 'Push Notification Permission Denied',
                  message: 'Push notifications have been disabled. You can enable them in your device settings if needed.',
                })
                setPushNotification(false)
                setPushNotificationPreferences({granted: false})
                console.log("User denied permission", resp)
              }
            })
          }
          else{
            setPushNotification(false)
          }
        }
        }
        color={pushNotification ? 'success' : 'neutral'}
        variant={pushNotification ? 'solid' : 'outlined'}
        endDecorator={pushNotification ? 'On' : 'Off'}
        slotProps={{
          endDecorator: {
            sx: {
              minWidth: 24,
            },
          },
        }}
      />
    </FormControl> */}

      <Button
        variant='soft'
        color='primary'
        sx={{
          width: '210px',
          mb: 1,
        }}
        onClick={() => {
          // schedule a local notification in 5 seconds
          LocalNotifications.schedule({
            notifications: [
              {
                title: 'Task Reminder',
                body: 'You have a task due soon',
                id: 1,
                schedule: { at: new Date(Date.now() + 3000) },
                sound: null,
                attachments: null,
                actionTypeId: '',
                extra: null,
              },
            ],
          })
        }}
      >
        Test Notification{' '}
      </Button>
      <Typography level='h3'>Custom Notification</Typography>
      <Divider />
      <Typography level='body-md'>
        Notificaiton through other platform like Telegram or Pushover
      </Typography>

      <FormControl orientation='horizontal'>
        <Switch
          checked={Boolean(chatID !== 0)}
          onClick={event => {
            event.preventDefault()
            if (chatID !== 0) {
              setChatID(0)
            } else {
              setChatID('')
              UpdateUserDetails({
                chatID: Number(0),
              }).then(resp => {
                resp.json().then(data => {
                  refetchUserProfile()
                })
              })
            }
            setNotificationTarget('0')
            handleSave()
          }}
          color={chatID !== 0 ? 'success' : 'neutral'}
          variant={chatID !== 0 ? 'solid' : 'outlined'}
          slotProps={{
            endDecorator: {
              sx: {
                minWidth: 24,
              },
            },
          }}
          sx={{ mr: 2 }}
        />
        <div>
          <FormLabel>Custom Notification</FormLabel>
          <FormHelperText sx={{ mt: 0 }}>
            Receive notification on other platform
          </FormHelperText>
        </div>
      </FormControl>
      {chatID !== 0 && (
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
          }}
        >
          <Select
            value={notificationTarget}
            sx={{ maxWidth: '200px' }}
            onChange={(e, selected) => setNotificationTarget(selected)}
          >
            <Option value='0'>None</Option>
            <Option value='1'>Telegram</Option>
            <Option value='2'>Pushover</Option>
            <Option value='3'>Webhooks</Option>
          </Select>
          {notificationTarget === '1' && (
            <>
              <Typography level='body-xs'>
                You need to initiate a message to the bot in order for the
                Telegram notification to work{' '}
                <a
                  style={{
                    textDecoration: 'underline',
                    color: '#0891b2',
                  }}
                  href='https://t.me/DonetickBot'
                >
                  Click here
                </a>{' '}
                to start a chat
              </Typography>

              <Typography level='body-sm'>Chat ID</Typography>

              <Input
                value={chatID}
                onChange={e => setChatID(e.target.value)}
                placeholder='User ID / Chat ID'
                sx={{
                  width: '200px',
                }}
              />
              <Typography mt={0} level='body-xs'>
                If you don't know your Chat ID, start chat with userinfobot and
                it will send you your Chat ID.{' '}
                <a
                  style={{
                    textDecoration: 'underline',
                    color: '#0891b2',
                  }}
                  href='https://t.me/userinfobot'
                >
                  Click here
                </a>{' '}
                to start chat with userinfobot{' '}
              </Typography>
            </>
          )}
          {notificationTarget === '2' && (
            <>
              <Typography level='body-sm'>User key</Typography>
              <Input
                value={chatID}
                onChange={e => setChatID(e.target.value)}
                placeholder='User ID'
                sx={{
                  width: '200px',
                }}
              />
            </>
          )}
          {error && (
            <Typography color='warning' level='body-sm'>
              {error}
            </Typography>
          )}

          <Button
            sx={{
              width: '110px',
              mb: 1,
            }}
            onClick={handleSave}
          >
            Save
          </Button>
        </Box>
      )}
    </div>
  )
}

export default NotificationSetting
