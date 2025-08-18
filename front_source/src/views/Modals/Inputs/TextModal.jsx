import { Box, Button, Textarea, Typography } from '@mui/joy'
import { useState } from 'react'
import FadeModal from '../../../components/common/FadeModal'

function TextModal({
  isOpen,
  onClose,
  onSave,
  current,
  title,
  okText,
  cancelText,
}) {
  const [text, setText] = useState(current)

  const handleSave = () => {
    onSave(text)
    onClose()
  }

  return (
    <FadeModal open={isOpen} onClose={onClose}>
      <Typography variant='h4'>{title}</Typography>
      <Textarea
        placeholder='Type in here…'
        value={text}
        onChange={e => setText(e.target.value)}
        minRows={2}
        maxRows={4}
        sx={{ minWidth: 300 }}
      />

      <Box display={'flex'} justifyContent={'space-around'} mt={1}>
        <Button onClick={handleSave} fullWidth sx={{ mr: 1 }}>
          {okText ? okText : 'Save'}
        </Button>
        <Button onClick={onClose} variant='outlined'>
          {cancelText ? cancelText : 'Cancel'}
        </Button>
      </Box>
    </FadeModal>
  )
}
export default TextModal
