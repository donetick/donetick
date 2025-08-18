import { Box, Button, Option, Select, Typography } from '@mui/joy'
import React from 'react'
import FadeModal from '../../../components/common/FadeModal'

function SelectModal({
  isOpen,
  onClose,
  onSave,
  options,
  title,
  displayKey,
  placeholder,
}) {
  const [selected, setSelected] = React.useState(null)
  const handleSave = () => {
    onSave(options.find(item => item.id === selected))
    onClose()
  }

  return (
    <FadeModal open={isOpen} onClose={onClose}>
      <Typography variant='h4'>{title}</Typography>
      <Select placeholder={placeholder}>
        {options.map((item, index) => (
          <Option
            value={item.id}
            key={item[displayKey]}
            onClick={() => {
              setSelected(item.id)
            }}
          >
            {item[displayKey]}
          </Option>
        ))}
      </Select>

      <Box display={'flex'} justifyContent={'space-around'} mt={1}>
        <Button onClick={handleSave} fullWidth sx={{ mr: 1 }}>
          Save
        </Button>
        <Button onClick={onClose} variant='outlined'>
          Cancel
        </Button>
      </Box>
    </FadeModal>
  )
}
export default SelectModal
