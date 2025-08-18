import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  Input,
  Typography,
} from '@mui/joy'
import { useState } from 'react'
import FadeModal from '../../../components/common/FadeModal'

function EditThingStateModal({ isOpen, onClose, onSave, currentThing }) {
  const [state, setState] = useState(currentThing?.state || '')
  const [errors, setErrors] = useState({})

  const isValid = () => {
    const newErrors = {}

    if (state.trim() === '') {
      newErrors.state = 'State is required'
    }

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSave = () => {
    if (!isValid()) {
      return
    }
    onSave({
      name,
      type: currentThing?.type,
      id: currentThing?.id,
      state: state || null,
    })
    onClose()
  }

  return (
    <FadeModal open={isOpen} onClose={onClose}>
      <Typography level='h4'>Update state</Typography>

      <FormControl>
        <Typography>Value</Typography>
        <Input
          placeholder='Thing value'
          value={state || ''}
          onChange={e => setState(e.target.value)}
          sx={{ minWidth: 300 }}
        />
        <FormHelperText color='danger'>{errors.state}</FormHelperText>
      </FormControl>

      <Box display={'flex'} justifyContent={'space-around'} mt={1}>
        <Button onClick={handleSave} fullWidth sx={{ mr: 1 }}>
          {currentThing?.id ? 'Update' : 'Create'}
        </Button>
        <Button onClick={onClose} variant='outlined'>
          {currentThing?.id ? 'Cancel' : 'Close'}
        </Button>
      </Box>
    </FadeModal>
  )
}
export default EditThingStateModal
