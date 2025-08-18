import {
  CalendarMonth,
  CancelScheduleSend,
  Check,
  Checklist,
  CloseFullscreen,
  Edit,
  History,
  LowPriority,
  OpenInFull,
  PeopleAlt,
  Person,
  PlayArrow,
  SwitchAccessShortcut,
} from '@mui/icons-material'
import {
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Chip,
  Container,
  Dropdown,
  FormControl,
  Grid,
  IconButton,
  Input,
  Menu,
  MenuButton,
  MenuItem,
  Sheet,
  Snackbar,
  Typography,
} from '@mui/joy'
import { Divider } from '@mui/material'
import { useQueryClient } from '@tanstack/react-query'
import moment from 'moment'
import { useEffect, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'

import { useImpersonateUser } from '../../contexts/ImpersonateUserContext.jsx'
import { useChoreDetails } from '../../queries/ChoreQueries.jsx'
import { useCircleMembers } from '../../queries/UserQueries.jsx'
import { notInCompletionWindow } from '../../utils/Chores.jsx'
import { getTextColorFromBackgroundColor } from '../../utils/Colors.jsx'
import {
  DeleteTimeSession,
  GetChoreDetailById,
  GetChoreTimer,
  MarkChoreComplete,
  PauseChore,
  ResetChoreTimer,
  SkipChore,
  StartChore,
  UpdateChorePriority,
} from '../../utils/Fetcher'
import Priorities from '../../utils/Priorities'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import LoadingComponent from '../components/Loading.jsx'
import RichTextEditor from '../components/RichTextEditor.jsx'
import SubTasks from '../components/SubTask.jsx'
import TimePassedCard from './TimePassedCard.jsx'
import TimerSplitButton from './TimerSplitButton.jsx'

const ChoreView = () => {
  const [chore, setChore] = useState({})
  const navigate = useNavigate()
  const [performers, setPerformers] = useState([])
  const [infoCards, setInfoCards] = useState([])
  const { choreId } = useParams()
  const [note, setNote] = useState(null)
  const queryClient = useQueryClient()

  const [searchParams] = useSearchParams()

  const [isPendingCompletion, setIsPendingCompletion] = useState(false)
  const [timeoutId, setTimeoutId] = useState(null)
  const [secondsLeftToCancel, setSecondsLeftToCancel] = useState(null)
  const [completedDate, setCompletedDate] = useState(null)
  const [confirmModelConfig, setConfirmModelConfig] = useState({})
  const [chorePriority, setChorePriority] = useState(null)
  const [isDescriptionOpen, setIsDescriptionOpen] = useState(false)
  const [timerActionConfig, setTimerActionConfig] = useState({})
  const { data: circleMembersData, isLoading: isCircleMembersLoading } =
    useCircleMembers()
  const { impersonatedUser } = useImpersonateUser()

  const { data: choreData, isLoading: isChoreLoading } =
    useChoreDetails(choreId)

  useEffect(() => {
    if (!choreData || !choreData.res || !circleMembersData) {
      return
    }
    setChore(choreData.res)
    setChorePriority(Priorities.find(p => p.value === choreData.res.priority))
    document.title = 'Donetick: ' + choreData.res.name

    setPerformers(circleMembersData.res)
    const auto_complete = searchParams.get('auto_complete')
    if (auto_complete === 'true') {
      handleTaskCompletion()
    }
  }, [choreData, circleMembersData])

  useEffect(() => {
    if (chore && performers?.length > 0) {
      generateInfoCards(chore)
    }
  }, [chore, performers])
  const handleUpdatePriority = priority => {
    UpdateChorePriority(choreId, priority.value).then(response => {
      if (response.ok) {
        response.json().then(() => {
          setChorePriority(priority)
          // Invalidate chores cache to refetch data
          queryClient.invalidateQueries(['chores'])
        })
      }
    })
  }
  const generateInfoCards = chore => {
    const cards = [
      {
        size: 6,
        icon: <PeopleAlt />,
        title: 'Assignment',
        text: `Assigned: ${
          performers.find(p => p.id === chore.assignedTo)?.displayName || 'N/A'
        }`,
        subtext: ` Last: ${
          chore.lastCompletedDate
            ? performers.find(p => p.id === chore.lastCompletedBy)?.displayName
            : '--'
        }`,
      },
      {
        size: 6,
        icon: <CalendarMonth />,
        title: 'Schedule',
        text: `Due: ${
          chore.nextDueDate ? moment(chore.nextDueDate).fromNow() : 'N/A'
        }`,
        subtext: `Last: ${
          chore.lastCompletedDate
            ? moment(chore.lastCompletedDate).fromNow()
            : 'N/A'
        }`,
      },
      {
        size: 6,
        icon: <Checklist />,
        title: 'Statistics',
        text: `Completed: ${chore.totalCompletedCount || 0} times`,
      },
      {
        size: 6,
        icon: <Person />,
        title: 'Details',
        subtext: `Created By: ${
          performers.find(p => p.id === chore.createdBy)?.displayName || 'N/A'
        }`,
      },
    ]
    setInfoCards(cards)
  }
  const handleTaskCompletion = () => {
    setIsPendingCompletion(true)
    let seconds = 3 // Starting countdown from 3 seconds
    setSecondsLeftToCancel(seconds)

    const countdownInterval = setInterval(() => {
      seconds -= 1
      setSecondsLeftToCancel(seconds)

      if (seconds <= 0) {
        clearInterval(countdownInterval) // Stop the countdown when it reaches 0
      }
    }, 1000)

    const id = setTimeout(() => {
      MarkChoreComplete(
        choreId,
        impersonatedUser
          ? { completedBy: impersonatedUser.userId, note }
          : { note },
        completedDate,
        null,
      )
        .then(resp => {
          if (resp.ok) {
            return resp.json().then(data => {
              setNote(null)
              setChore(data.res)
            })
          }
        })
        .then(() => {
          setIsPendingCompletion(false)
          clearTimeout(id)
          clearInterval(countdownInterval) // Ensure to clear this interval as well
          setTimeoutId(null)
          setSecondsLeftToCancel(null)
          // Invalidate chores cache to refetch data
          queryClient.invalidateQueries(['chores'])
        })
        .then(() => {
          // refetch the chore details
          GetChoreDetailById(choreId).then(resp => {
            if (resp.ok) {
              return resp.json().then(data => {
                setChore(data.res)
              })
            }
          })
        })
    }, 3000)

    setTimeoutId(id)
  }
  const handleSkippingTask = () => {
    SkipChore(choreId).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = data.res
          setChore(newChore)
          // Invalidate chores cache to refetch data
          queryClient.invalidateQueries(['chores'])
        })
      }
    })
  }
  const handleChoreStart = () => {
    StartChore(choreId).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = {
            ...chore,
            ...data.res,
          }
          setChore(newChore)
        })
      }
    })
  }

  const handleChorePause = () => {
    PauseChore(choreId).then(response => {
      if (response.ok) {
        response.json().then(data => {
          const newChore = {
            ...chore,
            ...data.res,
          }
          setChore(newChore)
        })
      }
    })
  }

  const handleResetTimer = () => {
    setTimerActionConfig({
      isOpen: true,
      title: 'Reset Timer',
      message:
        'Are you sure you want to reset the timer? This will clear all time records since you started the task.',
      confirmText: 'Reset Timer',
      cancelText: 'Cancel',
      onClose: confirmed => {
        if (confirmed) {
          ResetChoreTimer(choreId).then(response => {
            if (response.ok) {
              response.json().then(data => {
                const newChore = {
                  ...chore,
                  ...data.res,
                }
                setChore(newChore)
                queryClient.invalidateQueries(['chores'])
              })
            }
          })
        }
        setTimerActionConfig({})
      },
    })
  }

  const handleClearAllTime = () => {
    setTimerActionConfig({
      isOpen: true,
      title: 'Clear All Time Records',
      message:
        'This will permanently delete all timers for this task and set it back to "not started".',
      confirmText: 'Clear All Time',
      cancelText: 'Cancel',
      onClose: async confirmed => {
        if (confirmed) {
          const resp = await GetChoreTimer(choreId)
          if (resp.ok) {
            const data = await resp.json()
            const sessionId = data?.res?.id
            DeleteTimeSession(choreId, sessionId).then(response => {
              if (response.ok) {
                response.json().then(data => {
                  const newChore = {
                    ...chore,
                    ...data.res,
                  }
                  setChore(newChore)
                  queryClient.invalidateQueries(['chores'])
                })
              }
            })
          }
        }
        setTimerActionConfig({})
      },
    })
  }

  if (isChoreLoading || isCircleMembersLoading) {
    // while loading the chore or circle members, return a loading state
    return <LoadingComponent />
  }
  return (
    <Container
      maxWidth='sm'
      sx={{
        display: 'flex',
        flexDirection: 'column',
        // space between :
        justifyContent: 'space-between',
        // max height of the container:
        maxHeight: 'calc(100vh - 500px)',
      }}
    >
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          textAlign: 'center',
          mb: 1,
        }}
      >
        <Typography
          level='h3'
          // textAlign={'center'}
          sx={{
            mt: 1,
            mb: 0.5,
          }}
        >
          {chore.name}
        </Typography>
        <Chip startDecorator={<CalendarMonth />} size='md' sx={{ mb: 1 }}>
          {chore.nextDueDate
            ? `Due at ${moment(chore.nextDueDate).format('MM/DD/YYYY hh:mm A')}`
            : 'N/A'}
        </Chip>
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'row',
            justifyContent: 'center',
            alignItems: 'center',
            mb: 1,
          }}
        >
          {chore?.labelsV2?.map((label, index) => (
            <Chip
              key={index}
              sx={{
                position: 'relative',
                ml: index === 0 ? 0 : 0.5,
                top: 2,
                zIndex: 1,
                backgroundColor: label?.color,
                color: getTextColorFromBackgroundColor(label?.color),
              }}
            >
              {label?.name}
            </Chip>
          ))}
        </Box>
      </Box>

      <Box>
        <Grid
          container
          spacing={1}
          sx={{
            mb: 1,
          }}
        >
          {chore.status !== 0 && (
            <Grid item xs={12}>
              <TimePassedCard
                chore={chore}
                handleAction={action => {
                  if (action === 'pause') {
                    handleChorePause()
                  } else if (action === 'resume') {
                    handleChoreStart()
                  }
                }}
                onShowDetails={() => navigate(`/chores/${choreId}/timer`)}
              />
            </Grid>
          )}
          {infoCards.map((card, index) => (
            <Grid item xs={6} sm={6} key={index}>
              <Card
                variant='soft'
                sx={{
                  borderRadius: 'md',
                  boxShadow: 1,
                  px: 2,
                  py: 1,
                  minHeight: 90,
                  height: '100%',
                  // change from space-between to start:
                  justifyContent: 'start',
                }}
              >
                <CardContent>
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'start',
                      mb: 0.5,
                    }}
                  >
                    {card.icon}

                    <Typography
                      level='body-md'
                      sx={{
                        ml: 1,
                        fontWeight: '500',
                        color: 'text.primary',
                      }}
                    >
                      {card.title}
                    </Typography>
                  </Box>
                  <Box>
                    <Typography
                      level='body-sm'
                      sx={{ color: 'text.secondary', lineHeight: 1.5 }}
                    >
                      {card.text}
                    </Typography>
                    <Typography
                      level='body-sm'
                      sx={{ color: 'text.secondary', lineHeight: 1.5 }}
                    >
                      {card.subtext}
                    </Typography>
                  </Box>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'row',
            gap: 1,
            alignContent: 'center',
            justifyContent: 'center',
            mb: 1,
          }}
        >
          <Dropdown>
            <MenuButton
              color={
                chorePriority?.name === 'P1'
                  ? 'danger'
                  : chorePriority?.name === 'P2'
                    ? 'warning'
                    : 'neutral'
              }
              sx={{
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                p: 1,
              }}
              fullWidth
              variant='plain'
            >
              {chorePriority ? chorePriority.icon : <LowPriority />}
              {chorePriority ? chorePriority.name : 'No Priority'}
            </MenuButton>
            <Menu>
              {Priorities.map((priority, index) => (
                <MenuItem
                  sx={{
                    pr: 1,
                    py: 1,
                  }}
                  key={index}
                  onClick={() => {
                    handleUpdatePriority(priority)
                  }}
                  color={priority.color}
                >
                  {priority.icon}
                  {priority.name}
                </MenuItem>
              ))}
              <Divider />
              <MenuItem
                sx={{
                  pr: 1,
                  py: 1,
                }}
                onClick={() => {
                  handleUpdatePriority({
                    name: 'No Priority',
                    value: 0,
                  })
                  setChorePriority(null)
                }}
              >
                No Priority
              </MenuItem>
            </Menu>
          </Dropdown>

          <Button
            size='sm'
            color='neutral'
            variant='plain'
            fullWidth
            onClick={() => {
              navigate(`/chores/${choreId}/history`)
            }}
            sx={{
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              p: 1,
            }}
          >
            <History />
            History
          </Button>
          <Button
            size='sm'
            color='neutral'
            variant='plain'
            fullWidth
            sx={{
              // top right of the card:
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              p: 1,
            }}
            onClick={() => {
              navigate(`/chores/${choreId}/edit`)
            }}
          >
            <Edit />
            Edit
          </Button>
        </Box>

        {chore.description && (
          <>
            <Typography level='title-md' sx={{ mb: 1 }}>
              Description :
            </Typography>

            <Sheet
              variant='plain'
              sx={{
                p: 2,
                borderRadius: 'lg',
                mb: 1,
              }}
            >
              <IconButton
                variant='plain'
                onClick={() => {
                  setIsDescriptionOpen(!isDescriptionOpen)
                }}
                size='sm'
                sx={{
                  position: 'absolute',
                  bottom: 5,
                  right: 5,
                }}
              >
                {isDescriptionOpen ? <CloseFullscreen /> : <OpenInFull />}
              </IconButton>
              <Box
                sx={{
                  maxHeight: isDescriptionOpen ? 'none' : '100px',
                  overflowY: 'auto',
                }}
              >
                <RichTextEditor value={chore.description} isEditable={false} />
              </Box>
            </Sheet>
          </>
        )}

        {chore.notes && (
          <>
            <Typography level='title-md' sx={{ mb: 1 }}>
              Previous note:
            </Typography>
            <Sheet variant='plain' sx={{ p: 2, borderRadius: 'lg', mb: 1 }}>
              <Typography level='body-md' sx={{ mb: 1 }}>
                {chore.notes || '--'}
              </Typography>
            </Sheet>
          </>
        )}
        {chore.subTasks && chore.subTasks.length > 0 && (
          <Box sx={{ p: 0, m: 0, mb: 2 }}>
            <Typography level='title-md' sx={{ mb: 1 }}>
              Subtasks :
            </Typography>
            <Sheet
              variant='plain'
              sx={{
                borderRadius: 'lg',
                p: 1,
                overflow: 'auto',
                // maxHeight: '100px',
              }}
            >
              <SubTasks
                editMode={false}
                performers={performers}
                tasks={chore.subTasks}
                setTasks={tasks => {
                  setChore({
                    ...chore,
                    subTasks: tasks,
                  })
                }}
                choreId={choreId}
              />
            </Sheet>
          </Box>
        )}
      </Box>

      <Card
        sx={{
          p: 2,
          borderRadius: 'md',
          boxShadow: 'sm',
        }}
        variant='soft'
      >
        <Typography level='body-md' sx={{ mb: 1 }}>
          Completion options
        </Typography>

        <FormControl size='sm'>
          <Checkbox
            checked={note !== null}
            size='lg'
            onChange={e => {
              if (e.target.checked) {
                setNote('')
              } else {
                setNote(null)
              }
            }}
            overlay
            label={
              <Typography
                level='body-sm'
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                }}
              >
                Add a note
              </Typography>
            }
          />
        </FormControl>
        {note !== null && (
          <Input
            fullWidth
            multiline
            label='Additional Notes'
            placeholder='Add any additional notes here...'
            value={note || ''}
            onChange={e => {
              if (e.target.value.trim() === '') {
                setNote(null)
                return
              }
              setNote(e.target.value)
            }}
            sx={{
              mb: 1,
            }}
          />
        )}

        <FormControl size='sm'>
          <Checkbox
            checked={completedDate !== null}
            size='lg'
            onChange={e => {
              if (e.target.checked) {
                setCompletedDate(
                  moment(new Date()).format('YYYY-MM-DDTHH:00:00'),
                )
              } else {
                setCompletedDate(null)
              }
            }}
            overlay
            sx={
              {
                // my: 1,
              }
            }
            label={
              <Typography
                level='body-sm'
                sx={{
                  // center vertically
                  display: 'flex',
                  alignItems: 'center',
                }}
              >
                Set custom completion time
              </Typography>
            }
          />
        </FormControl>
        {completedDate !== null && (
          <Input
            sx={{ mt: 1, mb: 1.5, width: 300 }}
            type='datetime-local'
            value={completedDate}
            onChange={e => {
              setCompletedDate(e.target.value)
            }}
          />
        )}

        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            gap: 1,
            alignContent: 'center',
            justifyContent: 'center',
          }}
        >
          <Box
            sx={{
              display: 'flex',
              flexDirection: 'row',
              gap: 1,
              alignContent: 'center',
              justifyContent: 'center',
              mb: 1,
            }}
          >
            <Button
              fullWidth
              size='lg'
              onClick={handleTaskCompletion}
              disabled={
                isPendingCompletion ||
                notInCompletionWindow(chore) ||
                (chore.lastCompletedDate !== null &&
                  chore.frequencyType === 'once')
              }
              color={isPendingCompletion ? 'danger' : 'success'}
              startDecorator={<Check />}
              sx={{
                flex: 4,
              }}
            >
              <Box>Mark as done</Box>
            </Button>

            <Button
              fullWidth
              size='lg'
              onClick={() => {
                setConfirmModelConfig({
                  isOpen: true,
                  title: 'Skip Task',

                  message: 'Are you sure you want to skip this task?',

                  confirmText: 'Skip',
                  cancelText: 'Cancel',
                  onClose: confirmed => {
                    if (confirmed) {
                      handleSkippingTask()
                    }
                    setConfirmModelConfig({})
                  },
                })
              }}
              disabled={
                chore.lastCompletedDate !== null &&
                chore.frequencyType === 'once'
              }
              startDecorator={<SwitchAccessShortcut />}
              sx={{
                flex: 1,
              }}
            >
              <Box>Skip</Box>
            </Button>
          </Box>
          {/* Timer Button - Show split button when timer is active, regular button otherwise */}
          {chore.status !== 0 ? (
            <TimerSplitButton
              disabled={
                chore.lastCompletedDate !== null &&
                chore.frequencyType === 'once'
              }
              chore={chore}
              onAction={action => {
                if (action === 'pause') {
                  handleChorePause()
                } else if (action === 'resume') {
                  handleChoreStart()
                }
              }}
              onShowDetails={() => navigate(`/chores/${choreId}/timer`)}
              onResetTimer={handleResetTimer}
              onClearAllTime={handleClearAllTime}
              fullWidth
            />
          ) : (
            <Button
              size='lg'
              onClick={() => {
                handleChoreStart()
              }}
              variant='soft'
              color='success'
              disabled={
                chore.lastCompletedDate !== null &&
                chore.frequencyType === 'once'
              }
              startDecorator={<PlayArrow />}
              sx={{
                flex: 1,
              }}
            >
              Start
            </Button>
          )}
        </Box>

        <Snackbar
          open={isPendingCompletion}
          endDecorator={
            <Button
              onClick={() => {
                if (timeoutId) {
                  clearTimeout(timeoutId)
                  setIsPendingCompletion(false)
                  setTimeoutId(null)
                  setSecondsLeftToCancel(null) // Reset or adjust as needed
                }
              }}
              size='lg'
              variant='outlined'
              color='danger'
              startDecorator={<CancelScheduleSend />}
            >
              Cancel
            </Button>
          }
        >
          <Typography level='body-md' textAlign={'center'}>
            Task will be marked as completed in {secondsLeftToCancel} seconds
          </Typography>
        </Snackbar>
        <ConfirmationModal config={confirmModelConfig} />
        <ConfirmationModal config={timerActionConfig} />
      </Card>
    </Container>
  )
}

export default ChoreView
