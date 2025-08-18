import {
  Add,
  Delete,
  Edit,
  Flip,
  PlusOne,
  ToggleOff,
  ToggleOn,
  Widgets,
} from '@mui/icons-material'
import { Avatar, Box, Chip, Container, IconButton, Typography } from '@mui/joy'
import React, { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useNotification } from '../../service/NotificationProvider'
import {
  CreateThing,
  DeleteThing,
  GetThings,
  SaveThing,
  UpdateThingState,
} from '../../utils/Fetcher'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import CreateThingModal from '../Modals/Inputs/CreateThingModal'
import EditThingStateModal from '../Modals/Inputs/EditThingState'
const ThingCard = ({
  thing,
  onEditClick,
  onStateChangeRequest,
  onDeleteClick,
}) => {
  const [isDisabled, setIsDisabled] = useState(false)
  const Navigate = useNavigate()

  // Swipe functionality state
  const [swipeTranslateX, setSwipeTranslateX] = useState(0)
  const [isDragging, setIsDragging] = useState(false)
  const [isSwipeRevealed, setIsSwipeRevealed] = useState(false)
  const [hoverTimer, setHoverTimer] = useState(null)
  const swipeThreshold = 80
  const maxSwipeDistance = 200
  const dragStartX = useRef(0)
  const cardRef = useRef(null)

  const getThingIcon = type => {
    if (type === 'text') {
      return <Flip />
    } else if (type === 'number') {
      return <PlusOne />
    } else if (type === 'boolean') {
      if (thing.state === 'true') {
        return <ToggleOn />
      } else {
        return <ToggleOff />
      }
    } else {
      return <ToggleOff />
    }
  }

  const getThingAvatar = () => {
    const typeConfig = {
      text: { color: 'primary', icon: <Flip /> },
      number: { color: 'success', icon: <PlusOne /> },
      boolean: {
        color: thing.state === 'true' ? 'success' : 'neutral',
        icon: thing.state === 'true' ? <ToggleOn /> : <ToggleOff />,
      },
    }

    const config = typeConfig[thing?.type] || typeConfig.boolean
    return (
      <Avatar
        size='sm'
        color={config.color}
        variant='soft'
        sx={{
          width: 32,
          height: 32,
          '& svg': { fontSize: '16px' },
        }}
      >
        {config.icon}
      </Avatar>
    )
  }

  const handleRequestChange = thing => {
    setIsDisabled(true)
    resetSwipe()
    onStateChangeRequest(thing)
    setTimeout(() => {
      setIsDisabled(false)
    }, 2000)
  }

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
  React.useEffect(() => {
    return () => {
      if (hoverTimer) {
        clearTimeout(hoverTimer)
      }
    }
  }, [hoverTimer])

  return (
    <Box key={thing.id + '-compact-box'}>
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
            variant='soft'
            color='success'
            size='sm'
            onClick={e => {
              e.stopPropagation()
              if (thing?.type === 'text') {
                onEditClick(thing)
              } else {
                handleRequestChange(thing)
              }
            }}
            disabled={isDisabled}
            sx={{
              width: 40,
              height: 40,
              mx: 1,
            }}
          >
            {getThingIcon(thing?.type)}
          </IconButton>

          <IconButton
            variant='soft'
            color='neutral'
            size='sm'
            onClick={e => {
              e.stopPropagation()
              resetSwipe()
              onEditClick(thing)
            }}
            sx={{
              width: 40,
              height: 40,
              mx: 1,
            }}
          >
            <Edit sx={{ fontSize: 16 }} />
          </IconButton>

          <IconButton
            variant='soft'
            color='danger'
            size='sm'
            onClick={e => {
              e.stopPropagation()
              resetSwipe()
              onDeleteClick(thing)
            }}
            sx={{
              width: 40,
              height: 40,
              mx: 1,
            }}
          >
            <Delete sx={{ fontSize: 16 }} />
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
            Navigate(`/things/${thing?.id}`)
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
          {/* Avatar and Primary Action */}
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              mr: 2,
              flexShrink: 0,
            }}
          >
            {getThingAvatar()}
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
            {/* Line 1: Name + State */}
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                mb: 0.5,
              }}
            >
              <Typography
                level='title-sm'
                sx={{
                  fontWeight: 600,
                  fontSize: 14,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  mr: 1,
                  flex: 1,
                  minWidth: 0,
                }}
              >
                {thing?.name}
              </Typography>

              <Chip
                size='sm'
                variant='solid'
                color={
                  thing?.type === 'boolean' && thing?.state === 'true'
                    ? 'success'
                    : 'primary'
                }
                sx={{
                  fontSize: 11,
                  height: 20,
                  px: 1,
                  fontWeight: 'md',
                  flexShrink: 0,
                  ml: 1,
                }}
              >
                {thing?.state}
              </Chip>
            </Box>

            {/* Line 2: Type */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Chip
                size='sm'
                variant='soft'
                color='neutral'
                sx={{
                  fontSize: 10,
                  height: 18,
                  px: 0.75,
                }}
              >
                {thing?.type}
              </Chip>
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  )
}

const ThingsView = () => {
  const [things, setThings] = useState([])
  const [isShowCreateThingModal, setIsShowCreateThingModal] = useState(false)
  const [isShowEditThingStateModal, setIsShowEditStateModal] = useState(false)
  const [createModalThing, setCreateModalThing] = useState(null)
  const [confirmModelConfig, setConfirmModelConfig] = useState({})
  const { showError, showNotification } = useNotification()

  useEffect(() => {
    // fetch things
    GetThings().then(result => {
      result.json().then(data => {
        setThings(data.res)
      })
    })
  }, [])

  const handleSaveThing = thing => {
    let saveFunc = CreateThing
    if (thing?.id) {
      saveFunc = SaveThing
    }
    saveFunc(thing)
      .then(result => {
        result.json().then(data => {
          if (thing?.id) {
            const currentThings = [...things]
            const thingIndex = currentThings.findIndex(
              currentThing => currentThing.id === thing.id,
            )
            currentThings[thingIndex] = data.res
            setThings(currentThings)
          } else {
            const currentThings = [...things]
            currentThings.push(data.res)
            setThings(currentThings)
          }
          showNotification({
            type: 'success',
            title: 'Saved',
            message: 'Thing saved successfully',
          })
        })
      })
      .catch(error => {
        if (error?.queued) {
          showError({
            title: 'Unable to save thing',
            message: 'You are offline and the request has been queued',
          })
        } else {
          showError({
            title: 'Unable to save thing',
            message: 'An error occurred while saving the thing',
          })
        }
      })
  }
  const handleEditClick = thing => {
    setIsShowEditStateModal(true)
    setCreateModalThing(thing)
  }
  const handleDeleteClick = thing => {
    setConfirmModelConfig({
      isOpen: true,
      title: 'Delete Things',
      confirmText: 'Delete',
      cancelText: 'Cancel',
      message: 'Are you sure you want to delete this Thing?',
      onClose: isConfirmed => {
        if (isConfirmed === true) {
          DeleteThing(thing.id)
            .then(response => {
              if (response.ok) {
                const currentThings = [...things]
                const thingIndex = currentThings.findIndex(
                  currentThing => currentThing.id === thing.id,
                )
                currentThings.splice(thingIndex, 1)
                setThings(currentThings)
              } else if (response.status === 405) {
                showError({
                  title: 'Unable to Delete Thing',
                  message: 'Unable to delete thing with associated tasks',
                })
              }
              // if method not allwo show snackbar:
            })
            .catch(error => {
              if (error?.queued) {
                showError({
                  title: 'Unable to delete thing',
                  message: 'You are offline and the request has been queued',
                })
              } else {
                showError({
                  title: 'Unable to delete thing',
                  message: 'An error occurred while deleting the thing',
                })
              }
            })
        }
        setConfirmModelConfig({})
      },
    })
  }

  const handleStateChangeRequest = thing => {
    if (thing?.type === 'number') {
      thing.state = Number(thing.state) + 1
    } else if (thing?.type === 'boolean') {
      if (thing.state === 'true') {
        thing.state = 'false'
      } else {
        thing.state = 'true'
      }
    }

    UpdateThingState(thing)
      .then(result => {
        result.json().then(data => {
          const currentThings = [...things]
          const thingIndex = currentThings.findIndex(
            currentThing => currentThing.id === thing.id,
          )
          currentThings[thingIndex] = data.res
          setThings(currentThings)
          showNotification({
            type: 'success',
            title: 'Updated',
            message: 'Thing state updated successfully',
          })
        })
      })
      .catch(error => {
        if (error?.queued) {
          showError({
            title: 'Unable to update thing state',
            message: 'You are offline and the request has been queued',
          })
        } else {
          showError({
            title: 'Unable to update thing state',
            message: 'An error occurred while updating the thing state',
          })
        }
      })
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
        {things.length === 0 && (
          <Box
            sx={{
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              flexDirection: 'column',
              height: '50vh',
            }}
          >
            <Widgets
              sx={{
                fontSize: '4rem',
                mb: 1,
              }}
            />
            <Typography level='title-md' gutterBottom>
              No things has been created/found
            </Typography>
          </Box>
        )}
        {things.map(thing => (
          <ThingCard
            key={thing?.id}
            thing={thing}
            onEditClick={handleEditClick}
            onDeleteClick={handleDeleteClick}
            onStateChangeRequest={handleStateChangeRequest}
          />
        ))}
      </Box>
      <Box
        // variant='outlined'
        sx={{
          position: 'fixed',
          bottom: 0,
          left: 10,
          p: 2, // padding
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
          //   startDecorator={<Add />}
          onClick={() => {
            setIsShowCreateThingModal(true)
          }}
        >
          <Add />
        </IconButton>
        {isShowCreateThingModal && (
          <CreateThingModal
            isOpen={isShowCreateThingModal}
            onClose={() => {
              setIsShowCreateThingModal(false)
              setCreateModalThing(null)
            }}
            onSave={handleSaveThing}
            currentThing={createModalThing}
          />
        )}
        {isShowEditThingStateModal && (
          <EditThingStateModal
            isOpen={isShowEditThingStateModal}
            onClose={() => {
              setIsShowEditStateModal(false)
              setCreateModalThing(null)
            }}
            onSave={handleStateChangeRequest}
            currentThing={createModalThing}
          />
        )}

        <ConfirmationModal config={confirmModelConfig} />
      </Box>
    </Container>
  )
}

export default ThingsView
