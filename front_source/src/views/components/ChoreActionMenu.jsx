import {
  Archive,
  CopyAll,
  Delete,
  Edit,
  ManageSearch,
  MoreTime,
  MoreVert,
  Nfc,
  NoteAdd,
  RecordVoiceOver,
  SwitchAccessShortcut,
  Unarchive,
  Update,
  ViewCarousel,
} from '@mui/icons-material'
import { Divider, IconButton, Menu, MenuItem } from '@mui/joy'
import React, { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useNotification } from '../../service/NotificationProvider'
import {
  ArchiveChore,
  DeleteChore,
  SkipChore,
  UnArchiveChore,
} from '../../utils/Fetcher'

const ChoreActionMenu = ({
  chore,
  onChoreUpdate,
  onChoreRemove,
  onCompleteWithNote,
  onCompleteWithPastDate,
  onChangeAssignee,
  onChangeDueDate,
  onWriteNFC,
  onDelete,
  onOpen,
  onMouseEnter,
  onMouseLeave,
  sx = {},
  variant = 'soft',
}) => {
  const [anchorEl, setAnchorEl] = React.useState(null)
  const menuRef = React.useRef(null)
  const navigate = useNavigate()
  const { showError } = useNotification()

  useEffect(() => {
    const handleMenuOutsideClick = event => {
      if (
        anchorEl &&
        !anchorEl.contains(event.target) &&
        !menuRef.current.contains(event.target)
      ) {
        handleMenuClose()
      }
    }

    document.addEventListener('mousedown', handleMenuOutsideClick)
    if (anchorEl) {
      onOpen()
    }
    return () => {
      document.removeEventListener('mousedown', handleMenuOutsideClick)
    }
  }, [anchorEl])

  const handleMenuOpen = event => {
    event.stopPropagation()
    setAnchorEl(event.currentTarget)
  }

  const handleMenuClose = () => {
    setAnchorEl(null)
  }

  const handleEdit = () => {
    navigate(`/chores/${chore.id}/edit`)
    handleMenuClose()
  }

  const handleClone = () => {
    navigate(`/chores/${chore.id}/edit?clone=true`)
    handleMenuClose()
  }

  const handleView = () => {
    navigate(`/chores/${chore.id}`)
    handleMenuClose()
  }

  const handleDelete = () => {
    if (onDelete) {
      onDelete()
    } else {
      // Default delete behavior
      DeleteChore(chore.id).then(response => {
        if (response.ok) {
          onChoreRemove?.(chore)
        }
      })
    }
    handleMenuClose()
  }

  const handleArchive = () => {
    if (chore.isActive) {
      ArchiveChore(chore.id).then(response => {
        if (response.ok) {
          response.json().then(() => {
            const newChore = { ...chore, isActive: false }
            onChoreUpdate?.(newChore, 'archive')
          })
        }
      })
    } else {
      UnArchiveChore(chore.id).then(response => {
        if (response.ok) {
          response.json().then(() => {
            const newChore = { ...chore, isActive: true }
            onChoreUpdate?.(newChore, 'unarchive')
          })
        }
      })
    }
    handleMenuClose()
  }

  const handleSkip = () => {
    SkipChore(chore.id)
      .then(response => {
        if (response.ok) {
          response.json().then(data => {
            const newChore = data.res
            onChoreUpdate?.(newChore, 'skipped')
            handleMenuClose()
          })
        }
      })
      .catch(error => {
        if (error?.queued) {
          showError({
            title: 'Failed to update',
            message: 'Request will be processed when you are online',
          })
        } else {
          showError({
            title: 'Failed to update',
            message: error,
          })
        }
      })
  }

  const handleHistory = () => {
    navigate(`/chores/${chore.id}/history`)
    handleMenuClose()
  }

  return (
    <>
      <IconButton
        variant={variant}
        color='success'
        onClick={handleMenuOpen}
        onMouseEnter={onMouseEnter}
        onMouseLeave={onMouseLeave}
        sx={{
          borderRadius: '50%',
          width: 25,
          height: 25,
          position: 'relative',
          left: -10,
          ...sx,
        }}
      >
        <MoreVert />
      </IconButton>

      <Menu
        size='md'
        ref={menuRef}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleMenuClose}
        sx={{
          position: 'absolute',
          top: '100%',
          left: '50%',
        }}
      >
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            onCompleteWithNote?.()
            handleMenuClose()
          }}
        >
          <NoteAdd />
          Complete with note
        </MenuItem>
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            onCompleteWithPastDate?.()
            handleMenuClose()
          }}
        >
          <Update />
          Complete in past
        </MenuItem>
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            handleSkip()
          }}
        >
          <SwitchAccessShortcut />
          Skip to next due date
        </MenuItem>
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            onChangeAssignee?.()
            handleMenuClose()
          }}
        >
          <RecordVoiceOver />
          Delegate to someone else
        </MenuItem>
        <Divider />
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            handleHistory()
          }}
        >
          <ManageSearch />
          History
        </MenuItem>
        <Divider />
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            onChangeDueDate?.()
            handleMenuClose()
          }}
        >
          <MoreTime />
          Change due date
        </MenuItem>
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            onWriteNFC?.()
            handleMenuClose()
          }}
        >
          <Nfc />
          Write to NFC
        </MenuItem>
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            handleEdit()
          }}
        >
          <Edit />
          Edit
        </MenuItem>
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            handleClone()
          }}
        >
          <CopyAll />
          Clone
        </MenuItem>
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            handleView()
          }}
        >
          <ViewCarousel />
          View
        </MenuItem>
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            handleArchive()
          }}
          color='neutral'
        >
          {chore.isActive ? <Archive /> : <Unarchive />}
          {chore.isActive ? 'Archive' : 'Unarchive'}
        </MenuItem>
        <Divider />
        <MenuItem
          onClick={e => {
            e.stopPropagation()
            handleDelete()
          }}
          color='danger'
        >
          <Delete />
          Delete
        </MenuItem>
      </Menu>
    </>
  )
}

export default ChoreActionMenu
