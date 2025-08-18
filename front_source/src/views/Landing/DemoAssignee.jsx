import {
  Box,
  Card,
  Checkbox,
  Grid,
  List,
  ListItem,
  Option,
  Select,
  Typography,
} from '@mui/joy'
import { useState } from 'react'
const ASSIGN_STRATEGIES = [
  'random',
  'least_assigned',
  'least_completed',
  'keep_last_assigned',
]
const DemoAssignee = () => {
  const [assignStrategy, setAssignStrategy] = useState('random')
  const [assignees, setAssignees] = useState([
    {
      userId: 3,
      id: 3,
      displayName: 'Ryan',
    },
  ])
  const [assignedTo, setAssignedTo] = useState(3)
  const performers = [
    {
      userId: 1,
      id: 1,
      displayName: 'Mo',
    },
    {
      userId: 2,
      id: 2,
      displayName: 'Jiji',
    },
    {
      userId: 3,
      id: 3,
      displayName: 'Ryan',
    },
  ]
  return (
    <>
      <Grid item xs={12} sm={6} data-aos-create-chore-assignee>
        <Box
          mt={2}
          data-aos-delay={200}
          data-aos-anchor='[data-aos-create-chore-assignee]'
          data-aos='fade-right'
        >
          <Typography level='h4'>Assignees :</Typography>
          <Typography level='h5'>Who can do this chore?</Typography>
          <Card>
            <List
              orientation='horizontal'
              wrap
              sx={{
                '--List-gap': '8px',
                '--ListItem-radius': '20px',
              }}
            >
              {performers?.map(item => (
                <ListItem key={item.id}>
                  <Checkbox
                    // disabled={index === 0}
                    checked={assignees.find(a => a.userId == item.id) != null}
                    onClick={() => {
                      if (assignees.find(a => a.userId == item.id)) {
                        setAssignees(
                          assignees.filter(i => i.userId !== item.id),
                        )
                      } else {
                        setAssignees([...assignees, { userId: item.id }])
                      }
                    }}
                    overlay
                    disableIcon
                    variant='soft'
                    label={item.displayName}
                  />
                </ListItem>
              ))}
            </List>
          </Card>
        </Box>
        <Box
          mt={2}
          data-aos-delay={300}
          data-aos-anchor='[data-aos-create-chore-assignee]'
          data-aos='fade-right'
        >
          <Typography level='h4'>Assigned :</Typography>
          <Typography level='h5'>
            Who is assigned the next due chore?
          </Typography>

          <Select
            placeholder={
              assignees.length === 0
                ? 'No Assignees yet can perform this chore'
                : 'Select an assignee for this chore'
            }
            disabled={assignees.length === 0}
            value={assignedTo > -1 ? assignedTo : null}
          >
            {performers
              ?.filter(p => assignees.find(a => a.userId == p.userId))
              .map((item, index) => (
                <Option
                  value={item.id}
                  key={item.displayName}
                  onClick={() => {}}
                >
                  {item.displayName}
                  {/* <Chip size='sm' color='neutral' variant='soft'>
                </Chip> */}
                </Option>
              ))}
          </Select>
        </Box>
        <Box
          mt={2}
          data-aos-delay={400}
          data-aos-anchor='[data-aos-create-chore-assignee]'
          data-aos='fade-right'
        >
          <Typography level='h4'>Picking Mode :</Typography>
          <Typography level='h5'>
            How to pick the next assignee for the following chore?
          </Typography>

          <Card>
            <List
              orientation='horizontal'
              wrap
              sx={{
                '--List-gap': '8px',
                '--ListItem-radius': '20px',
              }}
            >
              {ASSIGN_STRATEGIES.map((item, idx) => (
                <ListItem key={item}>
                  <Checkbox
                    // disabled={index === 0}
                    checked={assignStrategy === item}
                    onClick={() => setAssignStrategy(item)}
                    overlay
                    disableIcon
                    variant='soft'
                    label={item
                      .split('_')
                      .map(x => x.charAt(0).toUpperCase() + x.slice(1))
                      .join(' ')}
                  />
                </ListItem>
              ))}
            </List>
          </Card>
        </Box>
      </Grid>
      <Grid item xs={12} sm={6} data-aos-create-chore-section-assignee>
        <Card
          sx={{
            p: 4,
            py: 6,
          }}
          data-aos-delay={200}
          data-aos-anchor='[data-aos-create-chore-section-assignee]'
          data-aos='fade-left'
        >
          <Typography level='h3' textAlign='center' sx={{ mt: 2, mb: 4 }}>
            Flexible Task Assignment
          </Typography>
          <Typography level='body-lg' textAlign='center' sx={{ mb: 4 }}>
            Whether you’re a solo user managing personal tasks or coordinating
            chores with others, Donetick provides robust assignment options.
            Assign tasks to different people and choose specific rotation
            strategies, such as assigning tasks based on who completed the most
            or least, randomly rotating assignments, or sticking with the last
            assigned person.
          </Typography>
        </Card>
      </Grid>
    </>
  )
}

export default DemoAssignee
