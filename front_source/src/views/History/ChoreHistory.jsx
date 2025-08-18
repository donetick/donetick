import { Checklist, EventBusy, Group, Timelapse } from '@mui/icons-material'
import {
  Avatar,
  Button,
  Chip,
  Container,
  Grid,
  List,
  ListItem,
  ListItemContent,
  Sheet,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { LoadingScreen, SmoothCard } from '../../components/animations'
import {
  DeleteChoreHistory,
  GetAllCircleMembers,
  GetChoreHistory,
  UpdateChoreHistory,
} from '../../utils/Fetcher'
import EditHistoryModal from '../Modals/EditHistoryModal'
import HistoryCard from './HistoryCard'

const ChoreHistory = () => {
  const [choreHistory, setChoresHistory] = useState([])
  const [userHistory, setUserHistory] = useState([])
  const [performers, setPerformers] = useState([])
  const [historyInfo, setHistoryInfo] = useState([])

  const [isLoading, setIsLoading] = useState(true) // Add loading state
  const { choreId } = useParams()
  const [isEditModalOpen, setIsEditModalOpen] = useState(false)
  const [editHistory, setEditHistory] = useState({})

  useEffect(() => {
    setIsLoading(true) // Start loading

    Promise.all([
      GetChoreHistory(choreId).then(res => res.json()),
      GetAllCircleMembers(),
    ])
      .then(([historyData, usersData]) => {
        setChoresHistory(historyData.res)

        const newUserChoreHistory = {}
        historyData.res.forEach(choreHistory => {
          const userId = choreHistory.completedBy
          newUserChoreHistory[userId] = (newUserChoreHistory[userId] || 0) + 1
        })
        setUserHistory(newUserChoreHistory)

        setPerformers(usersData.res)
        updateHistoryInfo(historyData.res, newUserChoreHistory, usersData.res)
      })
      .catch(error => {
        console.error('Error fetching data:', error)
        // Handle errors, e.g., show an error message to the user
      })
      .finally(() => {
        setIsLoading(false) // Finish loading
      })
  }, [choreId])

  const updateHistoryInfo = (histories, userHistories, performers) => {
    // average delay for task completaion from due date:

    const averageDelay =
      histories.reduce((acc, chore) => {
        if (chore.dueDate && chore.performedAt) {
          // Only consider chores with a due date
          return acc + moment(chore.performedAt).diff(chore.dueDate, 'hours')
        }
        return acc
      }, 0) / histories.filter(chore => chore.dueDate).length
    const averageDelayMoment = moment.duration(averageDelay, 'hours')
    const maximumDelay = histories.reduce((acc, chore) => {
      if (chore.dueDate) {
        // Only consider chores with a due date
        const delay = moment(chore.performedAt).diff(chore.dueDate, 'hours')
        return delay > acc ? delay : acc
      }
      return acc
    }, 0)

    const maxDelayMoment = moment.duration(maximumDelay, 'hours')

    // find max value in userHistories:
    const userCompletedByMost = Object.keys(userHistories).reduce((a, b) =>
      userHistories[a] > userHistories[b] ? a : b,
    )
    const userCompletedByLeast = Object.keys(userHistories).reduce((a, b) =>
      userHistories[a] < userHistories[b] ? a : b,
    )

    const historyInfo = [
      {
        icon: <Checklist />,
        text: 'Total Completed',
        subtext: `${histories.length} times`,
      },
      {
        icon: <Timelapse />,
        text: 'Usually Within',
        subtext: moment.duration(averageDelayMoment).isValid()
          ? moment.duration(averageDelayMoment).humanize()
          : '--',
      },
      {
        icon: <Timelapse />,
        text: 'Maximum Delay',
        subtext: moment.duration(maxDelayMoment).isValid()
          ? moment.duration(maxDelayMoment).humanize()
          : '--',
      },
      {
        icon: <Avatar />,
        text: ' Completed Most',
        subtext: `${
          performers.find(p => p.userId === Number(userCompletedByMost))
            ?.displayName
        } `,
      },
      //  contributes:
      {
        icon: <Group />,
        text: 'Total Performers',
        subtext: `${Object.keys(userHistories).length} users`,
      },
      {
        icon: <Avatar />,
        text: 'Last Completed',
        subtext: `${
          performers.find(p => p.userId === Number(histories[0].completedBy))
            ?.displayName
        }`,
      },
    ]

    setHistoryInfo(historyInfo)
  }

  if (isLoading) {
    return <LoadingScreen message='Loading task history...' />
  }
  if (!choreHistory.length) {
    return (
      <Container
        maxWidth='md'
        sx={{
          textAlign: 'center',
          display: 'flex',
          // make sure the content is centered vertically:
          alignItems: 'center',
          justifyContent: 'center',
          flexDirection: 'column',
          height: '50vh',
        }}
      >
        <EventBusy
          sx={{
            fontSize: '6rem',
            // color: 'text.disabled',
            mb: 1,
          }}
        />

        <Typography level='h3' gutterBottom>
          No History Yet
        </Typography>
        <Typography level='body1'>
          You haven't completed any tasks. Once you start finishing tasks,
          they'll show up here.
        </Typography>
        <Button variant='soft' sx={{ mt: 2 }}>
          <Link to='/my/chores'>Go back to chores</Link>
        </Button>
      </Container>
    )
  }

  return (
    <Container maxWidth='md'>
      <Typography level='title-md' mb={1.5}>
        Summary:
      </Typography>
      <Sheet
        // sx={{
        //   mb: 1,
        //   borderRadius: 'lg',
        //   p: 2,
        // }}
        sx={{ borderRadius: 'sm', p: 2 }}
        variant='outlined'
      >
        <Grid container spacing={1}>
          {historyInfo.map((info, index) => (
            <Grid item xs={4} key={index}>
              {/* divider between the list items: */}

              <ListItem key={index}>
                <ListItemContent>
                  <Typography level='body-xs' sx={{ fontWeight: 'md' }}>
                    {info.text}
                  </Typography>
                  <Chip color='primary' size='md' startDecorator={info.icon}>
                    {info.subtext ? info.subtext : '--'}
                  </Chip>
                </ListItemContent>
              </ListItem>
            </Grid>
          ))}
        </Grid>
      </Sheet>

      {/* User History Cards */}
      <Typography level='title-md' my={1.5}>
        History:
      </Typography>
      <Sheet variant='plain' sx={{ borderRadius: 'sm', boxShadow: 'md' }}>
        {/* Chore History List (Updated Style) */}

        <List sx={{ p: 0 }}>
          {choreHistory.map((historyEntry, index) => (
            <HistoryCard
              onClick={() => {
                setIsEditModalOpen(true)
                setEditHistory(historyEntry)
              }}
              historyEntry={historyEntry}
              performers={performers}
              allHistory={choreHistory}
              key={index}
              index={index}
            />
          ))}
        </List>
      </Sheet>
      <EditHistoryModal
        config={{
          isOpen: isEditModalOpen,
          onClose: () => {
            setIsEditModalOpen(false)
          },
          onSave: updated => {
            UpdateChoreHistory(choreId, editHistory.id, {
              performedAt: updated.performedAt,
              dueDate: updated.dueDate,
              notes: updated.notes,
            }).then(res => {
              if (!res.ok) {
                console.error('Failed to update chore history:', res)
                return
              }

              const newRecord = res.json().then(data => {
                const newRecord = data.res
                const newHistory = choreHistory.map(record =>
                  record.id === newRecord.id ? newRecord : record,
                )
                setChoresHistory(newHistory)
                setEditHistory(newRecord)
                setIsEditModalOpen(false)
              })
            })
          },
          onDelete: () => {
            DeleteChoreHistory(choreId, editHistory.id).then(() => {
              const newHistory = choreHistory.filter(
                record => record.id !== editHistory.id,
              )
              setChoresHistory(newHistory)
              setIsEditModalOpen(false)
            })
          },
        }}
        historyRecord={editHistory}
      />
    </Container>
  )
}

export default ChoreHistory
