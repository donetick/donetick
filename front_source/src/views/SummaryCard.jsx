import { Card, IconButton, Typography } from '@mui/joy'

const SummaryCard = () => {
  return (
    <Card>
      <div className='flex justify-between'>
        <div>
          <Typography level='h2'>Summary</Typography>
          <Typography level='body-xs'>
            This is a summary of your chores
          </Typography>
        </div>
        <IconButton>
          <MoreVert />
        </IconButton>
      </div>
      <div className='flex justify-between'>
        <div>
          <Typography level='h3'>Due Today</Typography>
          <Typography level='h1'>3</Typography>
        </div>
        <div>
          <Typography level='h3'>Overdue</Typography>
          <Typography level='h1'>1</Typography>
        </div>
      </div>
    </Card>
  )
}

export default SummaryCard
