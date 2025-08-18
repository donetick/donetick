import DeleteIcon from '@mui/icons-material/Delete'
import EditIcon from '@mui/icons-material/Edit'
import {
  Avatar,
  Box,
  Chip,
  CircularProgress,
  Container,
  IconButton,
  Typography,
} from '@mui/joy'
import { useEffect, useRef, useState } from 'react'
import LabelModal from '../Modals/Inputs/LabelModal'

// import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Add, ColorLens } from '@mui/icons-material'
import { useQueryClient } from '@tanstack/react-query'
import { useUserProfile } from '../../queries/UserQueries'
import LABEL_COLORS, {
  getTextColorFromBackgroundColor,
} from '../../utils/Colors'
import { DeleteLabel } from '../../utils/Fetcher'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import { useLabels } from './LabelQueries'

const LabelCard = ({ label, onEditClick, onDeleteClick, currentUserId }) => {
  // Helper function to get color name from hex value
  const getColorName = hexValue => {
    const colorObj = LABEL_COLORS.find(
      color => color.value.toLowerCase() === hexValue.toLowerCase(),
    )
    return colorObj ? colorObj.name : hexValue
  }

  // Check if current user owns this label
  const isOwnedByCurrentUser = label.created_by === currentUserId

  // Swipe functionality state
  const [swipeTranslateX, setSwipeTranslateX] = useState(0)
  const [isDragging, setIsDragging] = useState(false)
  const [isSwipeRevealed, setIsSwipeRevealed] = useState(false)
  const [hoverTimer, setHoverTimer] = useState(null)
  const swipeThreshold = 80
  const maxSwipeDistance = 160
  const dragStartX = useRef(0)
  const cardRef = useRef(null)

  // Swipe gesture handlers
  const handleTouchStart = e => {
    dragStartX.current = e.touches[0].clientX
    setIsDragging(true)
  }

  const handleTouchMove = e => {
    if (!isDragging) return

    const currentX = e.touches[0].clientX
    const deltaX = currentX - dragStartX.current

    if (isSwipeRevealed) {
      if (deltaX > 0) {
        const clampedDelta = Math.min(deltaX - maxSwipeDistance, 0)
        setSwipeTranslateX(clampedDelta)
      }
    } else {
      if (deltaX < 0) {
        const clampedDelta = Math.max(deltaX, -maxSwipeDistance)
        setSwipeTranslateX(clampedDelta)
      }
    }
  }

  const handleTouchEnd = () => {
    if (!isDragging) return
    setIsDragging(false)

    if (isSwipeRevealed) {
      if (swipeTranslateX > -swipeThreshold) {
        setSwipeTranslateX(0)
        setIsSwipeRevealed(false)
      } else {
        setSwipeTranslateX(-maxSwipeDistance)
      }
    } else {
      if (Math.abs(swipeTranslateX) > swipeThreshold) {
        setSwipeTranslateX(-maxSwipeDistance)
        setIsSwipeRevealed(true)
      } else {
        setSwipeTranslateX(0)
        setIsSwipeRevealed(false)
      }
    }
  }

  const handleMouseDown = e => {
    dragStartX.current = e.clientX
    setIsDragging(true)
  }

  const handleMouseMove = e => {
    if (!isDragging) return

    const currentX = e.clientX
    const deltaX = currentX - dragStartX.current

    if (isSwipeRevealed) {
      if (deltaX > 0) {
        const clampedDelta = Math.min(deltaX - maxSwipeDistance, 0)
        setSwipeTranslateX(clampedDelta)
      }
    } else {
      if (deltaX < 0) {
        const clampedDelta = Math.max(deltaX, -maxSwipeDistance)
        setSwipeTranslateX(clampedDelta)
      }
    }
  }

  const handleMouseUp = () => {
    if (!isDragging) return
    setIsDragging(false)

    if (isSwipeRevealed) {
      if (swipeTranslateX > -swipeThreshold) {
        setSwipeTranslateX(0)
        setIsSwipeRevealed(false)
      } else {
        setSwipeTranslateX(-maxSwipeDistance)
      }
    } else {
      if (Math.abs(swipeTranslateX) > swipeThreshold) {
        setSwipeTranslateX(-maxSwipeDistance)
        setIsSwipeRevealed(true)
      } else {
        setSwipeTranslateX(0)
        setIsSwipeRevealed(false)
      }
    }
  }

  const resetSwipe = () => {
    setSwipeTranslateX(0)
    setIsSwipeRevealed(false)
  }

  // Hover functionality for desktop - only trigger from drag area
  const handleMouseEnter = () => {
    if (isSwipeRevealed) return
    const timer = setTimeout(() => {
      setSwipeTranslateX(-maxSwipeDistance)
      setIsSwipeRevealed(true)
      setHoverTimer(null)
    }, 800) // Shorter delay for drag area
    setHoverTimer(timer)
  }

  const handleMouseLeave = () => {
    if (hoverTimer) {
      clearTimeout(hoverTimer)
      setHoverTimer(null)
    }
    // Only add hide timer if we're leaving the drag area and actions are NOT revealed
    // If actions are revealed, let the action area handle the hiding
    if (!isSwipeRevealed) {
      // Actions are not revealed, so we can safely hide after delay
      const hideTimer = setTimeout(() => {
        resetSwipe()
      }, 300)
      setHoverTimer(hideTimer)
    }
  }

  const handleActionAreaMouseEnter = () => {
    // Clear any pending timer when entering action area
    if (hoverTimer) {
      clearTimeout(hoverTimer)
      setHoverTimer(null)
    }
  }

  const handleActionAreaMouseLeave = () => {
    // Hide immediately when leaving action area
    if (isSwipeRevealed) {
      resetSwipe()
    }
  }

  // Clean up timer on unmount
  useEffect(() => {
    return () => {
      if (hoverTimer) {
        clearTimeout(hoverTimer)
      }
    }
  }, [hoverTimer])

  return (
    <Box key={label.id + '-compact-box'}>
      <Box
        sx={{
          position: 'relative',
          overflow: 'hidden',
          borderBottom: '1px solid',
          borderColor: 'divider',
          '&:last-child': {
            borderBottom: 'none',
          },
        }}
        onMouseLeave={() => {
          // Only clear timers, don't auto-hide
          if (hoverTimer) {
            clearTimeout(hoverTimer)
            setHoverTimer(null)
          }
        }}
      >
        {/* Action buttons underneath (revealed on swipe) */}
        <Box
          sx={{
            position: 'absolute',
            right: 0,
            top: 0,
            bottom: 0,
            width: maxSwipeDistance,
            display: 'flex',
            alignItems: 'center',
            boxShadow: 'inset 2px 0 4px rgba(0,0,0,0.06)',
            zIndex: 0,
          }}
          onMouseEnter={handleActionAreaMouseEnter}
          onMouseLeave={handleActionAreaMouseLeave}
        >
          <IconButton
            variant='plain'
            color='neutral'
            size='sm'
            onClick={e => {
              e.stopPropagation()
              resetSwipe()
              onEditClick(label)
            }}
            sx={{
              width: 40,
              height: 40,
              mx: 1,
              bgcolor: 'primary.100',
              color: 'primary.600',
              '&:hover': {
                bgcolor: 'primary.200',
              },
            }}
          >
            <EditIcon sx={{ fontSize: 16 }} />
          </IconButton>

          <IconButton
            variant='plain'
            color='danger'
            size='sm'
            onClick={e => {
              e.stopPropagation()
              resetSwipe()
              onDeleteClick(label.id)
            }}
            sx={{
              width: 40,
              height: 40,
              mx: 1,
              bgcolor: 'danger.100',
              color: 'danger.600',
              '&:hover': {
                bgcolor: 'danger.200',
              },
            }}
          >
            <DeleteIcon sx={{ fontSize: 16 }} />
          </IconButton>
        </Box>

        {/* Main card content */}
        <Box
          ref={cardRef}
          sx={{
            display: 'flex',
            alignItems: 'center',
            minHeight: 64,
            cursor: 'pointer',
            position: 'relative',
            px: 2,
            py: 1.5,
            bgcolor: 'background.body',
            transform: `translateX(${swipeTranslateX}px)`,
            transition: isDragging ? 'none' : 'transform 0.3s ease-out',
            zIndex: 1,
            '&:hover': {
              bgcolor: isSwipeRevealed
                ? 'background.surface'
                : 'background.level1',
              boxShadow: isSwipeRevealed ? 'none' : 'sm',
            },
          }}
          onClick={() => {
            if (isSwipeRevealed) {
              resetSwipe()
              return
            }
            // Optional: Navigate to label details or edit directly
            onEditClick(label)
          }}
          onTouchStart={handleTouchStart}
          onTouchMove={handleTouchMove}
          onTouchEnd={handleTouchEnd}
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
        >
          {/* Right drag area - only triggers reveal on hover */}
          <Box
            sx={{
              position: 'absolute',
              right: 0,
              top: 0,
              bottom: 0,
              width: '20px',
              cursor: 'grab',
              zIndex: 2,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              opacity: isSwipeRevealed ? 0 : 0.3, // Hide when action area is revealed
              transition: 'opacity 0.2s ease',
              pointerEvents: isSwipeRevealed ? 'none' : 'auto', // Disable pointer events when revealed
              '&:hover': {
                opacity: isSwipeRevealed ? 0 : 0.7,
              },
              '&:active': {
                cursor: 'grabbing',
              },
            }}
            onMouseEnter={handleMouseEnter}
            onMouseLeave={handleMouseLeave}
          >
            {/* Drag indicator dots */}
            <Box
              sx={{
                display: 'flex',
                flexDirection: 'column',
                gap: 0.25,
              }}
            >
              {[...Array(3)].map((_, i) => (
                <Box
                  key={i}
                  sx={{
                    width: 3,
                    height: 3,
                    borderRadius: '50%',
                    bgcolor: 'text.tertiary',
                  }}
                />
              ))}
            </Box>
          </Box>
          {/* Color Avatar */}
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              mr: 2,
              flexShrink: 0,
            }}
          >
            <Avatar
              size='sm'
              sx={{
                width: 32,
                height: 32,
                bgcolor: label.color,
                border: '2px solid',
                borderColor: isOwnedByCurrentUser
                  ? 'background.surface'
                  : 'warning.300',
                boxShadow: isOwnedByCurrentUser
                  ? 'sm'
                  : '0 0 0 1px var(--joy-palette-warning-300)',
              }}
            >
              <Typography
                level='body-xs'
                sx={{
                  color: getTextColorFromBackgroundColor(label.color),
                  fontWeight: 'bold',
                  fontSize: 10,
                }}
              >
                {label.name.charAt(0).toUpperCase()}
              </Typography>
            </Avatar>
          </Box>

          {/* Content - Center */}
          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            {/* Label Name */}
            <Typography
              level='title-sm'
              sx={{
                fontWeight: 600,
                fontSize: 14,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                mb: 0.25,
              }}
            >
              {label.name}
            </Typography>

            {/* Color Info */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              {label.color && (
                <Chip
                  size='sm'
                  variant='soft'
                  startDecorator={<ColorLens />}
                  sx={{
                    fontSize: 10,
                    height: 18,
                    px: 0.75,
                    bgcolor: `${label.color}20`,
                    color: label.color,
                    border: `1px solid ${label.color}30`,
                  }}
                >
                  {getColorName(label.color)}
                </Chip>
              )}
              {!isOwnedByCurrentUser && (
                <Chip
                  size='sm'
                  variant='soft'
                  color='warning'
                  sx={{
                    fontSize: 9,
                    height: 16,
                    px: 0.5,
                    fontWeight: 'md',
                  }}
                >
                  Shared
                </Chip>
              )}
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  )
}

const LabelView = () => {
  const { data: labels, isLabelsLoading, isError } = useLabels()
  const { data: userProfile } = useUserProfile()

  const [userLabels, setUserLabels] = useState([])
  const [modalOpen, setModalOpen] = useState(false)

  const [currentLabel, setCurrentLabel] = useState(null)
  const queryClient = useQueryClient()
  const [confirmationModel, setConfirmationModel] = useState({})

  const handleAddLabel = () => {
    setCurrentLabel(null)
    setModalOpen(true)
  }

  const handleEditLabel = label => {
    setCurrentLabel(label)
    setModalOpen(true)
  }

  const handleDeleteClicked = id => {
    setConfirmationModel({
      isOpen: true,
      title: 'Delete Label',

      message:
        'Are you sure you want to delete this label? This will remove the label from all tasks.',

      confirmText: 'Delete',
      color: 'danger',
      cancelText: 'Cancel',
      onClose: confirmed => {
        if (confirmed === true) {
          handleDeleteLabel(id)
        }
        setConfirmationModel({})
      },
    })
  }

  const handleDeleteLabel = id => {
    DeleteLabel(id).then(() => {
      const updatedLabels = userLabels.filter(label => label.id !== id)
      setUserLabels(updatedLabels)

      queryClient.invalidateQueries('labels')
    })
  }

  const handleSaveLabel = newOrUpdatedLabel => {
    queryClient.invalidateQueries('labels')
    setModalOpen(false)
    const updatedLabels = userLabels.map(label =>
      label.id === newOrUpdatedLabel.id ? newOrUpdatedLabel : label,
    )
    setUserLabels(updatedLabels)
  }

  useEffect(() => {
    if (labels) {
      setUserLabels(labels)
    }
  }, [labels])

  if (isLabelsLoading) {
    return (
      <Box
        display='flex'
        justifyContent='center'
        alignItems='center'
        height='100vh'
      >
        <CircularProgress />
      </Box>
    )
  }

  if (isError) {
    return (
      <Typography color='danger' textAlign='center'>
        Failed to load labels. Please try again.
      </Typography>
    )
  }

  return (
    <Container maxWidth='md' sx={{ px: 0 }}>
      <Box
        sx={{
          // bgcolor: 'background.body',
          // border: '1px solid',
          // borderColor: 'divider',
          // borderRadius: 'md',
          overflow: 'hidden',
        }}
      >
        {userLabels.length === 0 && (
          <Box
            sx={{
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              flexDirection: 'column',
              height: '50vh',
            }}
          >
            <Typography level='title-md' gutterBottom>
              No labels available. Add a new label to get started.
            </Typography>
          </Box>
        )}
        {userLabels.map(label => (
          <LabelCard
            key={label.id}
            label={label}
            onEditClick={handleEditLabel}
            onDeleteClick={handleDeleteClicked}
            currentUserId={userProfile?.id}
          />
        ))}
      </Box>

      {modalOpen && (
        <LabelModal
          isOpen={modalOpen}
          onClose={() => setModalOpen(false)}
          onSave={handleSaveLabel}
          label={currentLabel}
        />
      )}

      <Box
        sx={{
          position: 'fixed',
          bottom: 0,
          left: 10,
          p: 2,
          display: 'flex',
          justifyContent: 'flex-end',
          gap: 2,
          'z-index': 1000,
        }}
      >
        <IconButton
          color='primary'
          variant='solid'
          sx={{
            borderRadius: '50%',
            width: 50,
            height: 50,
          }}
          onClick={handleAddLabel}
        >
          <Add />
        </IconButton>
      </Box>
      <ConfirmationModal config={confirmationModel} />
    </Container>
  )
}

export default LabelView
