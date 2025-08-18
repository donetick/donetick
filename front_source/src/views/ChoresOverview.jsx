import {
  Adjust,
  CancelRounded,
  CheckBox,
  Edit,
  HelpOutline,
  History,
  QueryBuilder,
  SearchRounded,
  Warning,
} from '@mui/icons-material'
import {
  Avatar,
  Button,
  ButtonGroup,
  Chip,
  Container,
  Grid,
  IconButton,
  Input,
  Table,
  Tooltip,
  Typography,
} from '@mui/joy'

import moment from 'moment'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { GetAllUsers, GetChores, MarkChoreComplete } from '../utils/Fetcher'
import DateModal from './Modals/Inputs/DateModal'
// import moment from 'moment'

// enum for chore status:
const CHORE_STATUS = {
  NO_DUE_DATE: 'No due date',
  DUE_SOON: 'Soon',
  DUE_NOW: 'Due',
  OVER_DUE: 'Overdue',
}

const ChoresOverview = () => {
  const [chores, setChores] = useState([])
  const [filteredChores, setFilteredChores] = useState([])
  const [performers, setPerformers] = useState([])
  const [activeUserId, setActiveUserId] = useState(null)
  const [isDateModalOpen, setIsDateModalOpen] = useState(false)
  const [choreId, setChoreId] = useState(null)
  const [search, setSearch] = useState('')
  const Navigate = useNavigate()

  const getChoreStatus = chore => {
    if (chore.nextDueDate === null) {
      return CHORE_STATUS.NO_DUE_DATE
    }
    const dueDate = new Date(chore.nextDueDate)
    const now = new Date()
    const diff = dueDate - now
    if (diff < 0) {
      return CHORE_STATUS.OVER_DUE
    }
    if (diff > 1000 * 60 * 60 * 24) {
      return CHORE_STATUS.DUE_NOW
    }
    if (diff > 0) {
      return CHORE_STATUS.DUE_SOON
    }
    return CHORE_STATUS.NO_DUE_DATE
  }
  const getChoreStatusColor = chore => {
    switch (getChoreStatus(chore)) {
      case CHORE_STATUS.NO_DUE_DATE:
        return 'neutral'
      case CHORE_STATUS.DUE_SOON:
        return 'success'
      case CHORE_STATUS.DUE_NOW:
        return 'primary'
      case CHORE_STATUS.OVER_DUE:
        return 'warning'
      default:
        return 'neutral'
    }
  }
  const getChoreStatusIcon = chore => {
    switch (getChoreStatus(chore)) {
      case CHORE_STATUS.NO_DUE_DATE:
        return <HelpOutline />
      case CHORE_STATUS.DUE_SOON:
        return <QueryBuilder />
      case CHORE_STATUS.DUE_NOW:
        return <Adjust />
      case CHORE_STATUS.OVER_DUE:
        return <Warning />
      default:
        return <HelpOutline />
    }
  }
  useEffect(() => {
    // fetch chores:
    GetChores()
      .then(response => response.json())
      .then(data => {
        const filteredData = data.res.filter(
          chore => chore.assignedTo === activeUserId || chore.assignedTo === 0,
        )
        setChores(data.res)
        setFilteredChores(data.res)
      })
    GetAllUsers()
      .then(response => response.json())
      .then(data => {
        setPerformers(data.res)
      })
    const user = JSON.parse(localStorage.getItem('user'))
    if (user != null && user.id > 0) {
      setActiveUserId(user.id)
    }
  }, [])

  return (
    <Container>
      <Typography level='h4' mb={1.5}>
        Chores Overviews
      </Typography>
      {/* <SummaryCard /> */}
      <Grid container>
        <Grid
          item
          sm={6}
          alignSelf={'flex-start'}
          minWidth={100}
          display='flex'
          gap={2}
        >
          <Input
            placeholder='Search'
            value={search}
            onChange={e => {
              if (e.target.value === '') {
                setFilteredChores(chores)
              }
              setSearch(e.target.value)
              const newChores = chores.filter(chore => {
                return chore.name.includes(e.target.value)
              })
              setFilteredChores(newChores)
            }}
            endDecorator={
              search !== '' ? (
                <Button
                  variant='text'
                  onClick={() => {
                    setSearch('')
                    setFilteredChores(chores)
                  }}
                >
                  <CancelRounded />
                </Button>
              ) : (
                <Button variant='text'>
                  <SearchRounded />
                </Button>
              )
            }
          ></Input>
        </Grid>
        <Grid item sm={6} justifyContent={'flex-end'} display={'flex'} gap={2}>
          <Button
            onClick={() => {
              Navigate(`/chores/create`)
            }}
          >
            New Chore
          </Button>
        </Grid>
      </Grid>

      <Table>
        <thead>
          <tr>
            {/* first column has minium size because its icon */}
            <th style={{ width: 100 }}>Due</th>
            <th>Chore</th>
            <th>Assignee</th>
            <th>Due</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody>
          {filteredChores.map(chore => (
            <tr key={chore.id}>
              {/* cirular icon if the chore is due will be red else yellow: */}
              <td>
                <Chip color={getChoreStatusColor(chore)}>
                  {getChoreStatus(chore)}
                </Chip>
              </td>
              <td
                onClick={() => {
                  Navigate(`/chores/${chore.id}/edit`)
                }}
              >
                {chore.name || '--'}
              </td>
              <td>
                {chore.assignedTo > 0 ? (
                  <Tooltip
                    title={
                      performers.find(p => p.id === chore.assignedTo)
                        ?.displayName
                    }
                    size='sm'
                  >
                    <Chip
                      startDecorator={
                        <Avatar color='primary'>
                          {
                            performers.find(p => p.id === chore.assignedTo)
                              ?.displayName[0]
                          }
                        </Avatar>
                      }
                    >
                      {performers.find(p => p.id === chore.assignedTo)?.name}
                    </Chip>
                  </Tooltip>
                ) : (
                  <Chip
                    color='warning'
                    startDecorator={<Avatar color='primary'>?</Avatar>}
                  >
                    Unassigned
                  </Chip>
                )}
              </td>
              <td>
                <Tooltip
                  title={
                    chore.nextDueDate === null
                      ? 'no due date'
                      : moment(chore.nextDueDate).format('YYYY-MM-DD')
                  }
                  size='sm'
                >
                  <Typography>
                    {chore.nextDueDate === null
                      ? '--'
                      : moment(chore.nextDueDate).fromNow()}
                  </Typography>
                </Tooltip>
              </td>

              <td>
                <ButtonGroup
                // display='flex'
                // // justifyContent='space-around'
                // alignItems={'center'}
                // gap={0.5}
                >
                  <IconButton
                    variant='outlined'
                    size='sm'
                    // sx={{ borderRadius: '50%' }}
                    onClick={() => {
                      MarkChoreComplete(chore.id, null, null, null).then(
                        response => {
                          if (response.ok) {
                            response.json().then(data => {
                              const newChore = data.res
                              const newChores = [...chores]
                              const index = newChores.findIndex(
                                c => c.id === chore.id,
                              )
                              newChores[index] = newChore
                              setChores(newChores)
                              setFilteredChores(newChores)
                            })
                          }
                        },
                      )
                    }}
                    aria-setsize={2}
                  >
                    <CheckBox />
                  </IconButton>
                  <IconButton
                    variant='outlined'
                    size='sm'
                    // sx={{ borderRadius: '50%' }}
                    onClick={() => {
                      setChoreId(chore.id)
                      setIsDateModalOpen(true)
                    }}
                    aria-setsize={2}
                  >
                    <History />
                  </IconButton>
                  <IconButton
                    variant='outlined'
                    size='sm'
                    // sx={{
                    //   borderRadius: '50%',
                    // }}
                    onClick={() => {
                      Navigate(`/chores/${chore.id}/edit`)
                    }}
                  >
                    <Edit />
                  </IconButton>
                </ButtonGroup>
              </td>
            </tr>
          ))}
        </tbody>
      </Table>
      <DateModal
        isOpen={isDateModalOpen}
        key={choreId}
        title={`Change due date`}
        onClose={() => {
          setIsDateModalOpen(false)
        }}
        onSave={date => {
          if (activeUserId === null) {
            alert('Please select a performer')
            return
          }
          MarkChoreComplete(choreId, null, date, activeUserId).then(
            response => {
              if (response.ok) {
                response.json().then(data => {
                  const newChore = data.res
                  const newChores = [...chores]
                  const index = newChores.findIndex(c => c.id === chore.id)
                  newChores[index] = newChore
                  setChores(newChores)
                  setFilteredChores(newChores)
                })
              }
            },
          )
        }}
      />
    </Container>
  )
}

export default ChoresOverview
