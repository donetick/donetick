import { Box, Card, Grid, Typography } from '@mui/joy'
import { useState } from 'react'
import RepeatSection from '../ChoreEdit/RepeatSection'

const DemoScheduler = () => {
  const [assignees, setAssignees] = useState([])
  const [frequency, setFrequency] = useState(2)
  const [frequencyType, setFrequencyType] = useState('weekly')
  const [frequencyMetadata, setFrequencyMetadata] = useState({
    months: ['may', 'june', 'july'],
  })

  return (
    <>
      <Grid item xs={12} sm={5} data-aos-create-chore-scheduler>
        <Box
          data-aos-delay={300}
          data-aos-anchor='[data-aos-create-chore-scheduler]'
          data-aos='fade-right'
        >
          <RepeatSection
            frequency={frequency}
            onFrequencyUpdate={setFrequency}
            frequencyType={frequencyType}
            onFrequencyTypeUpdate={setFrequencyType}
            frequencyMetadata={frequencyMetadata}
            onFrequencyMetadataUpdate={setFrequencyMetadata}
            onFrequencyTimeUpdate={t => {}}
            frequencyError={null}
            allUserThings={[]}
            onTriggerUpdate={thingUpdate => {}}
            OnTriggerValidate={() => {}}
            isAttemptToSave={false}
            selectedThing={null}
          />
        </Box>
      </Grid>
      <Grid item xs={12} sm={7} data-aos-create-chore-section-scheduler>
        <Card
          sx={{
            p: 4,
            py: 6,
          }}
          data-aos-delay={200}
          data-aos-anchor='[data-aos-create-chore-section-scheduler]'
          data-aos='fade-left'
        >
          <Typography level='h3' textAlign='center' sx={{ mt: 2, mb: 4 }}>
            Advanced Scheduling and Automation
          </Typography>
          <Typography level='body-lg' textAlign='center' sx={{ mb: 4 }}>
            Scheduling is a crucial aspect of managing tasks and chores.
            Donetick offers basic scheduling options, such as recurring tasks
            daily, weekly, or yearly, as well as more customizable schedules
            like specific days of the week or month. For those unsure of exact
            frequencies, the adaptive scheduling feature averages based on how
            often you mark a task as completed. Additionally, Donetick supports
            automation by linking tasks with triggers via API. When specific
            conditions are met, Donetick’s Things feature will automatically
            initiate the task, streamlining your workflow.
          </Typography>
        </Card>
      </Grid>
    </>
  )
}

export default DemoScheduler
