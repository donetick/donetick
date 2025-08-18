import { Card, Grid, Typography } from '@mui/joy'
import moment from 'moment'
import ChoreCard from '../Chores/ChoreCard'

const DemoMyChore = () => {
  const cards = [
    {
      id: 12,
      name: '♻️ Take out recycle ',
      frequencyType: 'days_of_the_week',
      frequency: 1,
      frequencyMetadata:
        '{"days":["thursday"],"time":"2024-07-07T22:00:00-04:00"}',
      nextDueDate: moment().add(1, 'days').hour(8).minute(0).toISOString(),
      isRolling: false,
      assignedTo: 1,
    },
    {
      id: 9,
      name: '🐜 Spray Pesticide',
      frequencyType: 'interval',
      frequency: 3,
      frequencyMetadata: '{"unit":"months"}',
      nextDueDate: moment().subtract(7, 'day').toISOString(),
      isRolling: false,
      assignedTo: 1,
    },
    {
      id: 6,
      name: '🍂 Gutter Cleaning',
      frequencyType: 'day_of_the_month',
      frequency: 1,
      frequencyMetadata: '{"months":["may"]}',
      nextDueDate: moment()
        .month('may')
        .year(moment().year() + 1)
        .date(1)
        .hour(17)
        .minute(0)
        .toISOString(),
      isRolling: false,
      assignedTo: 1,
    },
    // {
    //   id: 10,
    //   name: '💨 Air dust Synology NAS and',
    //   frequencyType: 'interval',
    //   frequency: 12,
    //   frequencyMetadata: '{"unit":"weeks"}',
    //   nextDueDate: '2024-07-24T17:18:00Z',
    //   isRolling: false,
    //   assignedTo: 1,
    // },
    // {
    //   id: 8,
    //   name: '🛁 Deep Cleaning Bathroom',
    //   frequencyType: 'monthly',
    //   frequency: 1,
    //   frequencyMetadata: '{}',
    //   nextDueDate: '2024-08-04T17:15:00Z',
    //   isRolling: false,
    //   assignedTo: 1,
    // },
    // {
    //   id: 11,
    //   name: '☴ Replace AC Air filter',
    //   frequencyType: 'adaptive',
    //   frequency: 1,
    //   frequencyMetadata: '{"unit":"days"}',
    //   nextDueDate: moment().add(120, 'days').toISOString(),
    //   isRolling: false,
    //   assignedTo: 1,
    // },
    // {
    //   id: 6,
    //   name: '🍂 Gutter Cleaning ',
    //   frequencyType: 'day_of_the_month',
    //   frequency: 1,
    //   frequencyMetadata: '{"months":["may"]}',
    //   nextDueDate: '2025-05-01T17:00:00Z',
    //   isRolling: false,
    //   assignedTo: 1,
    // },
    // {
    //   id: 13,
    //   name: '🚰 Replace Water Filter',
    //   frequencyType: 'yearly',
    //   frequency: 1,
    //   frequencyMetadata: '{}',
    //   nextDueDate: '2025-07-08T01:00:00Z',
    //   isRolling: false,
    //   assignedTo: 1,
    // },
  ]

  const users = [{ displayName: 'Me', id: 1, userId: 1 }]
  return (
    <>
      <Grid item xs={12} sm={5} data-aos-first-tasks-list>
        {cards.map((card, index) => (
          <div
            key={index}
            data-aos-delay={100 * index + 200}
            data-aos-anchor='[data-aos-first-tasks-list]'
            data-aos='fade-up'
          >
            <ChoreCard chore={card} performers={users} viewOnly={true} />
          </div>
        ))}
      </Grid>
      <Grid item xs={12} sm={7} data-aos-my-chore-demo-section>
        <Card
          sx={{
            p: 4,
            py: 6,
          }}
          data-aos-delay={200}
          data-aos-anchor='[data-aos-my-chore-demo-section]'
          data-aos='fade-left'
        >
          <Typography level='h3' textAlign='center' sx={{ mt: 2, mb: 4 }}>
            Glance at Your Task and Chores
          </Typography>
          <Typography level='body-lg' textAlign='center' sx={{ mb: 4 }}>
            Main view prioritize tasks due today, followed by overdue ones, and
            finally, future tasks or those without due dates. With Donetick, you
            can view all the tasks you've created (whether assigned to you or
            not) as well as tasks assigned to you by others. Quickly mark them
            as done with just one click, ensuring a smooth and efficient task
            management experience.
          </Typography>
        </Card>
      </Grid>
    </>
  )
}

export default DemoMyChore
