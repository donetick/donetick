import { Box, Button, FormLabel, Input, Typography } from '@mui/joy'
import moment from 'moment'
import { useEffect, useState } from 'react'
import FadeModal from '../../components/common/FadeModal'
import ConfirmationModal from './Inputs/ConfirmationModal'

function EditHistoryModal({ config, historyRecord }) {
  useEffect(() => {
    setCompletedDate(
      moment(historyRecord.performedAt).format('YYYY-MM-DDTHH:mm'),
    )
    setDueDate(moment(historyRecord.dueDate).format('YYYY-MM-DDTHH:mm'))
    setNotes(historyRecord.notes)
  }, [historyRecord])

  const [completedDate, setCompletedDate] = useState(
    moment(historyRecord.completedDate).format('YYYY-MM-DDTHH:mm'),
  )
  const [dueDate, setDueDate] = useState(
    moment(historyRecord.dueDate).format('YYYY-MM-DDTHH:mm'),
  )
  const [notes, setNotes] = useState(historyRecord.notes)
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false)
  return (
    <FadeModal open={config?.isOpen} onClose={config?.onClose}>
      <Typography level='h4' mb={1}>
        Edit History
      </Typography>
      <FormLabel>Due Date</FormLabel>
      <Input
        type='datetime-local'
        value={dueDate}
        onChange={e => {
          setDueDate(e.target.value)
        }}
      />
      <FormLabel>Completed Date</FormLabel>
      <Input
        type='datetime-local'
        value={completedDate}
        onChange={e => {
          setCompletedDate(e.target.value)
        }}
      />
      <FormLabel>Note</FormLabel>
      <Input
        fullWidth
        multiline
        label='Additional Notes'
        placeholder='Additional Notes'
        value={notes}
        onChange={e => {
          if (e.target.value.trim() === '') {
            setNotes(null)
            return
          }
          setNotes(e.target.value)
        }}
        size='md'
        sx={{
          mb: 1,
        }}
      />

      {/* 3 button save , cancel and delete */}
      <Box display={'flex'} justifyContent={'space-around'} mt={1}>
        <Button
          onClick={() =>
            config.onSave({
              id: historyRecord.id,
              performedAt: moment(completedDate).toISOString(),
              dueDate: moment(dueDate).toISOString(),
              notes,
            })
          }
          fullWidth
          sx={{ mr: 1 }}
        >
          Save
        </Button>
        <Button onClick={config.onClose} variant='outlined'>
          Cancel
        </Button>
        <Button
          onClick={() => {
            setIsDeleteModalOpen(true)
          }}
          variant='outlined'
          color='danger'
        >
          Delete
        </Button>
      </Box>
      <ConfirmationModal
        config={{
          isOpen: isDeleteModalOpen,
          onClose: isConfirm => {
            if (isConfirm) {
              config.onDelete(historyRecord.id)
            }
            setIsDeleteModalOpen(false)
          },
          title: 'Delete History',
          message: 'Are you sure you want to delete this history?',
          confirmText: 'Delete',
          cancelText: 'Cancel',
        }}
      />
    </FadeModal>
  )
}
export default EditHistoryModal
