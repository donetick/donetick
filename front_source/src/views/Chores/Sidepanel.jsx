import { Box, Sheet } from '@mui/joy'
import { useMediaQuery } from '@mui/material'
import { useEffect, useState } from 'react'
import { useChoresHistory } from '../../queries/ChoreQueries'
import { ChoresGrouper } from '../../utils/Chores'
import CalendarView from '../components/CalendarView'
import ActivitiesCard from './ActivitesCard'
import WelcomeCard from './WelcomeCard'

const Sidepanel = ({ chores }) => {
  const isLargeScreen = useMediaQuery(theme => theme.breakpoints.up('md'))
  const [dueDatePieChartData, setDueDatePieChartData] = useState([])
  const {
    data: choresHistory,
    isChoresHistoryLoading,
    handleLimitChange: refetchHistory,
  } = useChoresHistory(7, true)

  useEffect(() => {
    setDueDatePieChartData(generateChoreDuePieChartData(chores))
  }, [])

  const generateChoreDuePieChartData = chores => {
    const groups = ChoresGrouper('due_date', chores, null)
    return groups
      .map(group => {
        return {
          label: group.name,
          value: group.content.length,
          color: group.color,
          id: group.name,
        }
      })
      .filter(item => item.value > 0)
  }

  if (!isLargeScreen) {
    return null
  }
  return (
    <Box>
      <WelcomeCard chores={chores} />
      <Sheet
        variant='plain'
        sx={{
          my: 1,
          p: 2,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          mr: 10,
          justifyContent: 'space-between',
          boxShadow: 'sm',
          borderRadius: 20,
          width: '315px',
        }}
      >
        <Box sx={{ width: '100%', overflowY: 'hidden', overflowX: 'hidden' }}>
          <CalendarView chores={chores} />
        </Box>
      </Sheet>
      <ActivitiesCard chores={chores} choreHistory={choresHistory} />
    </Box>
  )
}

export default Sidepanel
