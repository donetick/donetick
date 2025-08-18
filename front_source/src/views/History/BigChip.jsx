import Chip from '@mui/joy/Chip'
import * as React from 'react'

function BigChip(props) {
  return (
    <Chip
      variant='outlined'
      color='primary'
      size='lg' // Adjust to your desired size
      sx={{
        fontSize: '1rem', // Example: Increase font size
        padding: '1rem', // Example: Increase padding
        height: '1rem', // Adjust to your desired height
        // Add other custom styles as needed
      }}
      {...props}
    >
      {props.children}
    </Chip>
  )
}

export default BigChip
BigChip.propTypes = {
  ...Chip.propTypes,
}
