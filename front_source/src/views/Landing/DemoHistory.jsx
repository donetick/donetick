import { Box, Card, Grid, List, Typography } from '@mui/joy'
import moment from 'moment'
import HistoryCard from '../History/HistoryCard'

const DemoHistory = () => {
  const allHistory = [
    {
      id: 32,
      choreId: 12,
      completedAt: moment().hour(4).format(),
      completedBy: 1,
      assignedTo: 1,
      notes: null,
      dueDate: moment().format(),
    },
    {
      id: 31,
      choreId: 12,
      completedAt: moment().day(-1).format(),
      completedBy: 1,
      assignedTo: 1,
      notes: 'Need to be replaced with a new one',
      dueDate: moment().day(-2).format(),
    },
    {
      id: 31,
      choreId: 12,
      completedAt: moment().day(-10).hour(1).format(),
      completedBy: 2,
      assignedTo: 1,
      notes: null,
      dueDate: moment().day(-10).format(),
    },
  ]
  const performers = [
    {
      userId: 1,
      displayName: 'Ryan',
    },
    {
      userId: 2,
      displayName: 'Sarah',
    },
  ]

  return (
    <>
      <Grid item xs={12} sm={6} data-aos-history-list>
        <Box sx={{ borderRadius: 'sm', p: 2, boxShadow: 'md' }}>
          <List sx={{ p: 0 }}>
            {allHistory.map((historyEntry, index) => (
              <div
                data-aos-delay={100 * index + 200}
                data-aos-anchor='[data-aos-history-list]'
                data-aos='fade-right'
                key={index}
              >
                <HistoryCard
                  allHistory={allHistory}
                  historyEntry={historyEntry}
                  key={index}
                  index={index}
                  performers={performers}
                />
              </div>
            ))}
          </List>
        </Box>
      </Grid>
      <Grid item xs={12} sm={6} data-aos-history-demo-section>
        <Card
          sx={{
            p: 4,
            py: 6,
          }}
          data-aos-delay={200}
          data-aos-anchor='[data-aos-history-demo-section]'
          data-aos='fade-left'
        >
          <Typography level='h3' textAlign='center' sx={{ mt: 2, mb: 4 }}>
            History with a Purpose
          </Typography>
          <Typography level='body-lg' textAlign='center' sx={{ mb: 4 }}>
            Keep track of all your chores and tasks with ease. Donetick records
            due dates, completion dates, and who completed each task. Any notes
            added to tasks are also tracked, providing a complete history for
            your reference. Stay organized and informed with detailed task
            tracking.
          </Typography>
        </Card>
      </Grid>
    </>
  )
}
export default DemoHistory
