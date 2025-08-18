import {
  Box,
  Button,
  FormControl,
  Input,
  Option,
  Select,
  Typography,
} from '@mui/joy'
import { useEffect, useState } from 'react'
import FadeModal from '../../../components/common/FadeModal'

import { useQueryClient } from '@tanstack/react-query'
import { useNotification } from '../../../service/NotificationProvider.jsx'
import LABEL_COLORS from '../../../utils/Colors.jsx'
import { CreateLabel, UpdateLabel } from '../../../utils/Fetcher'
import { useLabels } from '../../Labels/LabelQueries'

function LabelModal({ isOpen, onClose, label }) {
  const [labelName, setLabelName] = useState('')
  const [color, setColor] = useState('')
  const [error, setError] = useState('')
  const { data: userLabels = [] } = useLabels()
  const queryClient = useQueryClient()
  const { showError } = useNotification()

  // Populate the form fields when editing
  useEffect(() => {
    if (label) {
      setLabelName(label.name)
      setColor(label.color)
    } else {
      setLabelName('')
      setColor('')
    }
    setError('')
  }, [label])

  // Validation logic
  const validateLabel = () => {
    if (!labelName.trim()) {
      setError('Name cannot be empty')
      return false
    }
    if (
      userLabels.some(
        userLabel => userLabel.name === labelName && userLabel.id !== label?.id,
      )
    ) {
      setError('Label with this name already exists')
      return false
    }
    if (!color) {
      setError('Please select a color')
      return false
    }
    return true
  }

  const handleSave = () => {
    if (!validateLabel()) return
    const saveLabel = label?.id && label.id !== -1 ? UpdateLabel : CreateLabel
    saveLabel({
      id: label?.id,
      name: labelName,
      color,
    })
      .then(res => {
        if (res?.error) {
          setError(res.error)
        } else {
          queryClient.invalidateQueries('labels')
          onClose()
        }
      })
      .catch(err => {
        if (err.queued) {
          showError({
            title: 'Failed to save label',
            message: 'Unable to save label. Please try again.',
          })
        } else {
          showError({
            title: 'Failed to save label',
            message: 'Unable to save label. Please try again.',
          })
        }
      })
  }

  return (
    <FadeModal open={isOpen} onClose={onClose}>
      <Typography level='title-md' mb={1}>
        {label ? 'Edit Label' : 'Add Label'}
      </Typography>

      <FormControl>
        <Typography gutterBottom level='body-sm' alignSelf='start'>
          Name
        </Typography>
        <Input
          fullWidth
          id='labelName'
          value={labelName}
          onChange={e => setLabelName(e.target.value)}
        />
      </FormControl>

      <FormControl>
        <Typography gutterBottom level='body-sm' alignSelf='start'>
          Color
        </Typography>
        <Select
          value={color}
          onChange={(e, value) => value && setColor(value)}
          renderValue={selected => (
            <Typography
              startDecorator={
                <Box
                  className='size-4'
                  borderRadius={10}
                  sx={{ background: selected.value }}
                />
              }
            >
              {selected.label}
            </Typography>
          )}
        >
          {LABEL_COLORS.map(val => (
            <Option key={val.value} value={val.value}>
              <Box className='flex items-center justify-between'>
                <Box
                  width={20}
                  height={20}
                  borderRadius={10}
                  sx={{ background: val.value }}
                />
                <Typography sx={{ ml: 1 }} variant='caption'>
                  {val.name}
                </Typography>
              </Box>
            </Option>
          ))}
        </Select>
      </FormControl>

      {error && (
        <Typography color='warning' level='body-sm'>
          {error}
        </Typography>
      )}

      <Box display='flex' justifyContent='space-around' mt={1}>
        <Button onClick={handleSave} fullWidth sx={{ mr: 1 }}>
          {label ? 'Save Changes' : 'Add Label'}
        </Button>
        <Button onClick={onClose} variant='outlined'>
          Cancel
        </Button>
      </Box>
    </FadeModal>
  )
}

export default LabelModal
