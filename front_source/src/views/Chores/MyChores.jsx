import {
  Add,
  Archive,
  Bolt,
  CancelRounded,
  CheckBox,
  CheckBoxOutlineBlank,
  Close,
  Delete,
  Done,
  EditCalendar,
  ExpandCircleDown,
  Grain,
  PriorityHigh,
  SelectAll,
  SkipNext,
  Sort,
  Style,
  Unarchive,
  ViewAgenda,
  ViewModule,
} from '@mui/icons-material'
import {
  Accordion,
  AccordionDetails,
  AccordionGroup,
  Box,
  Button,
  Chip,
  Container,
  Divider,
  IconButton,
  Input,
  List,
  Menu,
  MenuItem,
  Typography,
} from '@mui/joy'
import Fuse from 'fuse.js'
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useChores } from '../../queries/ChoreQueries'
import { useNotification } from '../../service/NotificationProvider'
import { ArchiveChore, GetArchivedChores } from '../../utils/Fetcher'
import Priorities from '../../utils/Priorities'
import LoadingComponent from '../components/Loading'
import { useLabels } from '../Labels/LabelQueries'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import ChoreCard from './ChoreCard'
import CompactChoreCard from './CompactChoreCard'
import IconButtonWithMenu from './IconButtonWithMenu'
import MultiSelectHelp from './MultiSelectHelp'

import KeyboardShortcutHint from '../../components/common/KeyboardShortcutHint'
import { useImpersonateUser } from '../../contexts/ImpersonateUserContext.jsx'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries'
import { ChoreFilters, ChoresGrouper, ChoreSorter } from '../../utils/Chores'
import { DeleteChore, MarkChoreComplete, SkipChore } from '../../utils/Fetcher'
import TaskInput from '../components/AddTaskModal'
import {
  canScheduleNotification,
  scheduleChoreNotification,
} from './LocalNotificationScheduler'
import NotificationAccessSnackbar from './NotificationAccessSnackbar'
import Sidepanel from './Sidepanel'
import SortAndGrouping from './SortAndGrouping'

const MyChores = () => {
  const { data: userProfile, isLoading: isUserProfileLoading } =
    useUserProfile()
  const { showSuccess, showError, showWarning } = useNotification()
  const { impersonatedUser } = useImpersonateUser()
  const [chores, setChores] = useState([])
  const [archivedChores, setArchivedChores] = useState(null)
  const [filteredChores, setFilteredChores] = useState([])
  const [searchFilter, setSearchFilter] = useState('All')
  const [choreSections, setChoreSections] = useState([])

  const [showSearchFilter, setShowSearchFilter] = useState(false)
  const [addTaskModalOpen, setAddTaskModalOpen] = useState(false)
  const [taskInputFocus, setTaskInputFocus] = useState(0)
  const searchInputRef = useRef(null)
  const [searchInputFocus, setSearchInputFocus] = useState(0)
  const [selectedChoreSection, setSelectedChoreSection] = useState(
    localStorage.getItem('selectedChoreSection') || 'due_date',
  )
  const [openChoreSections, setOpenChoreSections] = useState(
    JSON.parse(localStorage.getItem('openChoreSections')) || {},
  )
  const [selectedChoreFilter, setSelectedChoreFilter] = useState(
    localStorage.getItem('selectedChoreFilter') || 'anyone',
  )
  const [searchTerm, setSearchTerm] = useState('')
  const [performers, setPerformers] = useState([])
  const [anchorEl, setAnchorEl] = useState(null)
  const [isCompactView, setIsCompactView] = useState(
    localStorage.getItem('choreCardViewMode') === 'compact',
  )
  const menuRef = useRef(null)
  const Navigate = useNavigate()
  const { data: userLabels, isLoading: userLabelsLoading } = useLabels()
  const {
    data: choresData,
    isLoading: choresLoading,
    refetch: refetchChores,
  } = useChores(false)
  const { data: membersData, isLoading: membersLoading } = useCircleMembers()

  // Multi-select state
  const [isMultiSelectMode, setIsMultiSelectMode] = useState(false)
  const [selectedChores, setSelectedChores] = useState(new Set())
  const [confirmModelConfig, setConfirmModelConfig] = useState({})
  const [showKeyboardShortcuts, setShowKeyboardShortcuts] = useState(false)
  useEffect(() => {
    ;(async () => {
      if (!choresLoading && !membersLoading && userProfile) {
        setPerformers(membersData.res)
        const sortedChores = choresData.res.sort(ChoreSorter)
        setChores(sortedChores)
        setFilteredChores(sortedChores)
        const sections = ChoresGrouper(
          selectedChoreSection,
          sortedChores,
          ChoreFilters(userProfile)[selectedChoreFilter],
        )
        setChoreSections(sections)
        if (localStorage.getItem('openChoreSections') === null) {
          setSelectedChoreSectionWithCache(selectedChoreSection)
          setOpenChoreSections(
            Object.keys(sections).reduce((acc, key) => {
              acc[key] = true
              return acc
            }, {}),
          )
        }

        if (await canScheduleNotification()) {
          console.log('Scheduling chore notifications...')
          scheduleChoreNotification(
            choresData.res,
            userProfile,
            membersData.res,
          )
        }
      }
    })()
  }, [
    membersLoading,
    choresLoading,
    isUserProfileLoading,
    choresData,
    membersData,
    userProfile,
  ])

  useEffect(() => {
    document.addEventListener('mousedown', handleMenuOutsideClick)
    return () => {
      document.removeEventListener('mousedown', handleMenuOutsideClick)
    }
  }, [anchorEl])

  useEffect(() => {
    if (searchInputFocus > 0 && searchInputRef.current) {
      searchInputRef.current.focus()
      searchInputRef.current.selectionStart =
        searchInputRef.current.value?.length
      searchInputRef.current.selectionEnd = searchInputRef.current.value?.length
    }
  }, [searchInputFocus])

  // Keyboard shortcuts for multi-select and other actions
  useEffect(() => {
    const handleKeyDown = event => {
      // if the modal open we don't want anything here to trigger
      if (addTaskModalOpen) return
      // if Ctrl/Cmd + / then show keyboard shortcuts modal
      if (event.ctrlKey || event.metaKey) {
        setShowKeyboardShortcuts(true)
      }
      const isHoldingCmdOrCtrl = event.ctrlKey || event.metaKey
      // Ctrl/Cmd + K to open task modal
      if (isHoldingCmdOrCtrl && event.key === 'k') {
        event.preventDefault()
        setAddTaskModalOpen(true)
        return
      }

      if (addTaskModalOpen) {
        // we want to ignore anything in here until the modal close
        return
      }

      // Ctrl/Cmd + J to navigate to create chore page
      if (isHoldingCmdOrCtrl && event.key === 'j') {
        event.preventDefault()
        Navigate(`/chores/create`)
        return
      }

      // Ctrl/Cmd + F to focus search input:
      else if (isHoldingCmdOrCtrl && event.key === 'f') {
        event.preventDefault()
        searchInputRef.current?.focus()
        return
        // Ctrl/Cmd + X to close search input
      } else if (isHoldingCmdOrCtrl && event.key === 'x') {
        event.preventDefault()
        if (searchTerm?.length > 0) {
          handleSearchClose()
        }
      }
      // Ctrl/Cmd + S Toggle Multi-select mode
      else if (isHoldingCmdOrCtrl && event.key === 's') {
        event.preventDefault()
        toggleMultiSelectMode()
        return
      }

      // Ctrl/Cmd + A to select all - works both in and out of multi-select mode
      else if (
        isHoldingCmdOrCtrl &&
        event.key === 'a' &&
        !['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)
      ) {
        event.preventDefault()
        if (!isMultiSelectMode) {
          // Enable multi-select mode and select all visible tasks
          setIsMultiSelectMode(true)
          setTimeout(() => {
            selectAllVisibleChores()
          }, 0)
          // showSuccess({
          //   title: '🎯 Multi-select Mode Active',
          //   message: 'Selected all visible tasks. Press Esc to exit.',
          // })
        } else {
          // Already in multi-select mode, check if all visible tasks are already selected
          let visibleChores = []

          if (searchTerm?.length > 0 || searchFilter !== 'All') {
            visibleChores = filteredChores
            const allVisibleSelected =
              visibleChores.length > 0 &&
              visibleChores.every(chore => selectedChores.has(chore.id))

            if (allVisibleSelected) {
              showSuccess({
                title: '✅ All Tasks Selected',
                message: `All ${visibleChores.length} filtered task${visibleChores.length !== 1 ? 's are' : ' is'} already selected.`,
              })
            } else {
              selectAllVisibleChores()
              showSuccess({
                title: '🎯 Tasks Selected',
                message: `Selected ${visibleChores.length} filtered task${visibleChores.length !== 1 ? 's' : ''}.`,
              })
            }
          } else {
            // Check expanded sections first
            const expandedChores = choreSections
              .filter((section, index) => openChoreSections[index])
              .flatMap(section => section.content || [])

            const allExpandedSelected =
              expandedChores.length > 0 &&
              expandedChores.every(chore => selectedChores.has(chore.id))

            // Get all chores (including collapsed sections)
            const allChores = choreSections.flatMap(
              section => section.content || [],
            )
            const allChoresSelected =
              allChores.length > 0 &&
              allChores.every(chore => selectedChores.has(chore.id))

            if (allChoresSelected) {
              // All chores (including collapsed) are already selected
              showSuccess({
                title: '✅ All Tasks Selected',
                message: `All ${allChores.length} task${allChores.length !== 1 ? 's are' : ' is'} already selected (including collapsed sections).`,
              })
            } else if (allExpandedSelected) {
              // All expanded are selected, now select ALL (including collapsed)
              selectAllVisibleChores() // This will now select all chores
              const collapsedCount = allChores.length - expandedChores.length
              showSuccess({
                title: '🎯 All Tasks Selected',
                message: `Selected all ${allChores.length} tasks (including ${collapsedCount} from collapsed sections).`,
              })
            } else {
              // Not all expanded are selected, select expanded only
              selectAllVisibleChores() // This will select expanded only
              showSuccess({
                title: '🎯 Tasks Selected',
                message: `Selected ${expandedChores.length} task${expandedChores.length !== 1 ? 's' : ''} from expanded sections.`,
              })
            }
          }
        }
      }

      // Multi-select keyboard shortcuts (only when in multi-select mode)
      if (isMultiSelectMode) {
        // Escape to clear selection or exit multi-select mode
        if (event.key === 'Escape') {
          event.preventDefault()
          if (selectedChores.size > 0) {
            clearSelection()
          } else {
            setIsMultiSelectMode(false)
          }
          return
        }

        // Enter key for bulk complete
        if (
          isHoldingCmdOrCtrl &&
          event.key === 'Enter' &&
          selectedChores.size > 0
        ) {
          event.preventDefault()
          handleBulkComplete()
          return
        }

        // "/" key for bulk skip
        if (
          isHoldingCmdOrCtrl &&
          event.key === '/' &&
          selectedChores.size > 0
        ) {
          event.preventDefault()
          handleBulkSkip()
          return
        }

        // "x" key for bulk archive (without shift or modifiers)
        if (
          isHoldingCmdOrCtrl &&
          (event.key === 'x' || event.key === 'X') &&
          selectedChores.size > 0 &&
          !['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)
        ) {
          event.preventDefault()
          handleBulkArchive()
          return
        }
      }

      if (
        isHoldingCmdOrCtrl &&
        (event.key === 'e' || event.key === 'E') &&
        selectedChores.size > 0 &&
        !['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)
      ) {
        event.preventDefault()
        if (isMultiSelectMode && selectedChores.size > 0) {
          handleBulkDelete()
        }
        return
      }
    }
    const handleKeyUp = event => {
      if (!event.ctrlKey && !event.metaKey) {
        setShowKeyboardShortcuts(false)
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    document.addEventListener('keyup', handleKeyUp)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.removeEventListener('keyup', handleKeyUp)
    }
  }, [isMultiSelectMode, selectedChores.size, addTaskModalOpen])
  const setSelectedChoreSectionWithCache = value => {
    setSelectedChoreSection(value)
    localStorage.setItem('selectedChoreSection', value)
  }
  const setOpenChoreSectionsWithCache = value => {
    setOpenChoreSections(value)
    localStorage.setItem('openChoreSections', JSON.stringify(value))
  }
  const setSelectedChoreFilterWithCache = value => {
    setSelectedChoreFilter(value)
    localStorage.setItem('selectedChoreFilter', value)
  }

  const toggleViewMode = () => {
    const newMode = !isCompactView
    setIsCompactView(newMode)
    localStorage.setItem('choreCardViewMode', newMode ? 'compact' : 'default')
  }

  // Helper function to render the appropriate card component
  const renderChoreCard = (chore, key) => {
    const CardComponent = isCompactView ? CompactChoreCard : ChoreCard
    return (
      <CardComponent
        key={key || chore.id}
        chore={chore}
        onChoreUpdate={handleChoreUpdated}
        onChoreRemove={handleChoreDeleted}
        performers={performers}
        userLabels={userLabels}
        onChipClick={handleLabelFiltering}
        // Multi-select props
        isMultiSelectMode={isMultiSelectMode}
        isSelected={selectedChores.has(chore.id)}
        onSelectionToggle={() => toggleChoreSelection(chore.id)}
      />
    )
  }

  const updateChores = newChore => {
    const newChores = chores
    newChores.push(newChore)
    setChores(newChores)
    setFilteredChores(newChores)
    setChoreSections(
      ChoresGrouper(
        selectedChoreSection,
        newChores,
        ChoreFilters(userProfile)[selectedChoreFilter],
      ),
    )
    setSearchFilter('All')
  }
  const handleMenuOutsideClick = event => {
    if (
      anchorEl &&
      !anchorEl.contains(event.target) &&
      !menuRef.current.contains(event.target)
    ) {
      handleFilterMenuClose()
    }
  }
  const handleFilterMenuOpen = event => {
    event.preventDefault()
    setAnchorEl(event.currentTarget)
  }

  const handleFilterMenuClose = () => {
    setAnchorEl(null)
  }

  const handleLabelFiltering = chipClicked => {
    if (chipClicked.label) {
      const label = chipClicked.label
      const labelFiltered = [...chores].filter(chore =>
        chore.labelsV2.some(
          l => l.id === label.id && l.created_by === label.created_by,
        ),
      )
      setFilteredChores(labelFiltered)
      setSearchFilter('Label: ' + label.name)
    } else if (chipClicked.priority) {
      const priority = chipClicked.priority
      const priorityFiltered = chores.filter(
        chore => chore.priority === priority,
      )
      setFilteredChores(priorityFiltered)
      setSearchFilter('Priority: ' + priority)
    }
  }

  const handleChoreUpdated = (updatedChore, event) => {
    var newChores = chores.map(chore => {
      if (chore.id === updatedChore.id) {
        return updatedChore
      }
      return chore
    })

    var newFilteredChores = filteredChores.map(chore => {
      if (chore.id === updatedChore.id) {
        return updatedChore
      }
      return chore
    })
    if (
      event === 'archive' ||
      (event === 'completed' && updatedChore.frequencyType === 'once')
    ) {
      newChores = newChores.filter(chore => chore.id !== updatedChore.id)
      newFilteredChores = newFilteredChores.filter(
        chore => chore.id !== updatedChore.id,
      )
      if (archivedChores !== null) {
        setArchivedChores([...archivedChores, updatedChore])
      }
    }
    if (event === 'unarchive') {
      newChores.push(updatedChore)
      newFilteredChores.push(updatedChore)
      setArchivedChores(
        archivedChores.filter(chore => chore.id !== updatedChore.id),
      )
    }
    setChores(newChores)
    setFilteredChores(newFilteredChores)
    setChoreSections(
      ChoresGrouper(
        selectedChoreSection,
        newChores,
        ChoreFilters(userProfile)[selectedChoreFilter],
      ),
    )

    switch (event) {
      case 'completed':
        showSuccess({
          title: 'Task Completed',
          message: 'Great job! The task has been marked as completed.',
        })
        break
      case 'skipped':
        showSuccess({
          title: 'Task Skipped',
          message: 'The task has been moved to the next due date.',
        })
        break
      case 'rescheduled':
        showSuccess({
          title: 'Task Rescheduled',
          message: 'The task due date has been updated successfully.',
        })
        break
      case 'unarchive':
        showSuccess({
          title: 'Task Restored',
          message: 'The task has been restored and is now active.',
        })
        break
      case 'archive':
        showSuccess({
          title: 'Task Archived',
          message:
            'The task has been archived and hidden from the active list.',
        })
        break
      case 'started':
        showSuccess({
          title: 'Task Started',
          message: 'The task has been marked as started.',
        })
        break
      case 'paused':
        showWarning({
          title: 'Task Paused',
          message: 'The task has been paused.',
        })
        break
      case 'deleted':
      default:
        showSuccess({
          title: 'Task Updated',
          message: 'Your changes have been saved successfully.',
        })
    }
  }

  const handleChoreDeleted = deletedChore => {
    const newChores = chores.filter(chore => chore.id !== deletedChore.id)
    const newFilteredChores = filteredChores.filter(
      chore => chore.id !== deletedChore.id,
    )
    setChores(newChores)
    setFilteredChores(newFilteredChores)
    setChoreSections(
      ChoresGrouper(
        selectedChoreSection,
        newChores,
        ChoreFilters(userProfile)[selectedChoreFilter],
      ),
    )
  }

  const searchOptions = {
    // keys to search in
    keys: ['name', 'raw_label'],
    includeScore: true, // Optional: if you want to see how well each result matched the search term
    isCaseSensitive: false,
    findAllMatches: true,
  }

  const fuse = new Fuse(
    chores.map(c => ({
      ...c,
      raw_label: c.labelsV2?.map(c => c.name).join(' '),
    })),
    searchOptions,
  )

  const handleSearchChange = e => {
    if (searchFilter !== 'All') {
      setSearchFilter('All')
    }
    const search = e.target.value
    if (search === '') {
      setFilteredChores(chores)
      setSearchTerm('')
      return
    }

    const term = search.toLowerCase()
    setSearchTerm(term)
    setFilteredChores(fuse.search(term).map(result => result.item))
  }
  const handleSearchClose = () => {
    setSearchTerm('')
    setFilteredChores(chores)
    // remove the focus from the search input:
    setSearchInputFocus(0)
  }

  // Multi-select helper functions
  const toggleMultiSelectMode = () => {
    const newMode = !isMultiSelectMode
    setIsMultiSelectMode(newMode)

    if (newMode) {
      setSelectedChores(new Set()) // Clear selection when exiting multi-select
    }
  }

  const toggleChoreSelection = choreId => {
    const newSelection = new Set(selectedChores)
    if (newSelection.has(choreId)) {
      newSelection.delete(choreId)
    } else {
      newSelection.add(choreId)
    }
    setSelectedChores(newSelection)
  }

  const selectAllVisibleChores = () => {
    let visibleChores = []

    if (searchTerm?.length > 0 || searchFilter !== 'All') {
      // If there's a search term or filter, all filtered chores are visible
      visibleChores = filteredChores
    } else {
      // First, get chores from expanded sections only
      const expandedChores = choreSections
        .filter((section, index) => openChoreSections[index]) // Only expanded sections
        .flatMap(section => section.content || []) // Get all chores from expanded sections

      // Check if all expanded chores are already selected
      const allExpandedSelected =
        expandedChores.length > 0 &&
        expandedChores.every(chore => selectedChores.has(chore.id))

      if (allExpandedSelected) {
        // If all expanded chores are already selected, select ALL chores (including collapsed sections)
        visibleChores = choreSections.flatMap(section => section.content || [])
      } else {
        // Otherwise, just select expanded chores
        visibleChores = expandedChores
      }
    }

    if (visibleChores.length > 0) {
      const allIds = new Set(visibleChores.map(chore => chore.id))
      setSelectedChores(allIds)
    }
  }

  const clearSelection = () => {
    // if already empty, just exit multi-select mode:
    if (selectedChores.size === 0) {
      setIsMultiSelectMode(false)
      return
    }
    setSelectedChores(new Set())
  }

  const getSelectedChoresData = () => {
    const allChores = [...chores, ...(archivedChores || [])]
    return Array.from(selectedChores)
      .map(id => allChores.find(chore => chore.id === id))
      .filter(Boolean)
  }

  // Bulk operations with improved UX and confirmation modal
  const handleBulkComplete = async () => {
    const selectedData = getSelectedChoresData()
    if (selectedData.length === 0) return

    setConfirmModelConfig({
      isOpen: true,
      title: 'Complete Tasks',
      confirmText: 'Complete',
      cancelText: 'Cancel',
      message: `Mark ${selectedData.length} task${selectedData.length > 1 ? 's' : ''} as completed?`,
      onClose: async isConfirmed => {
        if (isConfirmed === true) {
          try {
            const completedTasks = []
            const failedTasks = []

            for (const chore of selectedData) {
              try {
                await MarkChoreComplete(
                  chore.id,
                  impersonatedUser
                    ? { completedBy: impersonatedUser.userId }
                    : null,
                  null,
                  null,
                )
                completedTasks.push(chore)
              } catch (error) {
                failedTasks.push(chore)
              }
            }

            if (completedTasks.length > 0) {
              showSuccess({
                title: '✅ Tasks Completed',
                message: `Successfully completed ${completedTasks.length} task${completedTasks.length > 1 ? 's' : ''}.`,
              })
            }

            if (failedTasks.length > 0) {
              showError({
                title: 'Some Tasks Failed',
                message: `${failedTasks.length} task${failedTasks.length > 1 ? 's' : ''} could not be completed.`,
              })
            }

            refetchChores()
            clearSelection()
          } catch (error) {
            showError({
              title: 'Bulk Complete Failed',
              message: 'An unexpected error occurred. Please try again.',
            })
          }
        }
        setConfirmModelConfig({})
      },
    })
  }
  const handleBulkArchive = async () => {
    const selectedData = getSelectedChoresData()
    if (selectedData.length === 0) return
    setConfirmModelConfig({
      isOpen: true,
      title: 'Archive Tasks',
      confirmText: 'Archive',
      cancelText: 'Cancel',
      message: `Archive ${selectedData.length} task${selectedData.length > 1 ? 's' : ''}?`,
      onClose: async isConfirmed => {
        if (isConfirmed === true) {
          try {
            const archivedTasks = []
            const failedTasks = []
            for (const chore of selectedData) {
              try {
                const archivedChore = await ArchiveChore(chore.id)
                archivedTasks.push(archivedChore)
                // Remove from chores and filteredChores
                setChores(chores.filter(c => c.id !== chore.id))
                setFilteredChores(filteredChores.filter(c => c.id !== chore.id))
              } catch (error) {
                failedTasks.push(chore)
              }
            }
            if (archivedTasks.length > 0) {
              showSuccess({
                title: '📦 Tasks Archived',
                message: `Successfully archived ${archivedTasks.length} task${archivedTasks.length > 1 ? 's' : ''}.`,
              })
              // Update archived chores state
              setArchivedChores([
                ...(archivedChores || []),
                ...archivedTasks.map(c => ({
                  ...c,
                  archived: true,
                })),
              ])
            }
            if (failedTasks.length > 0) {
              showError({
                title: 'Some Tasks Failed',
                message: `${failedTasks.length} task${failedTasks.length > 1 ? 's' : ''} could not be archived.`,
              })
            }
            clearSelection()
          } catch (error) {
            showError({
              title: 'Bulk Archive Failed',
              message: 'An unexpected error occurred. Please try again.',
            })
          }
        }
        setConfirmModelConfig({})
      },
    })
  }
  const handleBulkDelete = async () => {
    const selectedData = getSelectedChoresData()
    if (selectedData.length === 0) return

    setConfirmModelConfig({
      isOpen: true,
      title: 'Delete Tasks',
      confirmText: 'Delete',
      cancelText: 'Cancel',
      message: `Delete ${selectedData.length} task${selectedData.length > 1 ? 's' : ''}?\n\nThis action cannot be undone.`,
      onClose: async isConfirmed => {
        if (isConfirmed === true) {
          try {
            const deletedTasks = []
            const failedTasks = []

            for (const chore of selectedData) {
              try {
                await DeleteChore(chore.id)
                deletedTasks.push(chore)
              } catch (error) {
                failedTasks.push(chore)
              }
            }

            if (deletedTasks.length > 0) {
              showSuccess({
                title: '🗑️ Tasks Deleted',
                message: `Successfully deleted ${deletedTasks.length} task${deletedTasks.length > 1 ? 's' : ''}.`,
              })

              const deletedIds = new Set(deletedTasks.map(c => c.id))
              const newChores = chores.filter(c => !deletedIds.has(c.id))
              const newFilteredChores = filteredChores.filter(
                c => !deletedIds.has(c.id),
              )
              setChores(newChores)
              setFilteredChores(newFilteredChores)
              setChoreSections(
                ChoresGrouper(
                  selectedChoreSection,
                  newChores,
                  ChoreFilters(userProfile)[selectedChoreFilter],
                ),
              )
            }

            if (failedTasks.length > 0) {
              showError({
                title: 'Some Tasks Failed',
                message: `${failedTasks.length} task${failedTasks.length > 1 ? 's' : ''} could not be deleted.`,
              })
            }

            clearSelection()
          } catch (error) {
            showError({
              title: 'Bulk Delete Failed',
              message: 'An unexpected error occurred. Please try again.',
            })
          }
        }
        setConfirmModelConfig({})
      },
    })
  }

  const handleBulkSkip = async () => {
    const selectedData = getSelectedChoresData()
    if (selectedData.length === 0) return

    setConfirmModelConfig({
      isOpen: true,
      title: 'Skip Tasks',
      confirmText: 'Skip',
      cancelText: 'Cancel',
      message: `Skip ${selectedData.length} task${selectedData.length > 1 ? 's' : ''} to next due date?`,
      onClose: async isConfirmed => {
        if (isConfirmed === true) {
          try {
            const skippedTasks = []
            const failedTasks = []

            for (const chore of selectedData) {
              try {
                await SkipChore(chore.id)
                skippedTasks.push(chore)
              } catch (error) {
                failedTasks.push(chore)
              }
            }

            if (skippedTasks.length > 0) {
              showSuccess({
                title: '⏭️ Tasks Skipped',
                message: `Successfully skipped ${skippedTasks.length} task${skippedTasks.length > 1 ? 's' : ''}.`,
              })
            }

            if (failedTasks.length > 0) {
              showError({
                title: 'Some Tasks Failed',
                message: `${failedTasks.length} task${failedTasks.length > 1 ? 's' : ''} could not be skipped.`,
              })
            }

            refetchChores()
            clearSelection()
          } catch (error) {
            showError({
              title: 'Bulk Skip Failed',
              message: 'An unexpected error occurred. Please try again.',
            })
          }
        }
        setConfirmModelConfig({})
      },
    })
  }

  if (
    isUserProfileLoading ||
    userLabelsLoading ||
    performers.length === 0 ||
    choresLoading
  ) {
    return (
      <>
        <LoadingComponent />
      </>
    )
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'row',
      }}
    >
      <Container maxWidth='md'>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignContent: 'center',
            alignItems: 'center',
            gap: 0.5,
          }}
        >
          <Input
            slotProps={{ input: { ref: searchInputRef } }}
            placeholder='Search'
            value={searchTerm}
            onFocus={() => {
              setShowSearchFilter(true)
            }}
            fullWidth
            sx={{
              mt: 1,
              mb: 1,
              borderRadius: 24,
              height: 24,
              borderColor: 'text.disabled',
              padding: 1,
            }}
            onChange={handleSearchChange}
            startDecorator={
              <KeyboardShortcutHint shortcut='F' show={showKeyboardShortcuts} />
            }
            endDecorator={
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                {searchTerm && (
                  <>
                    <KeyboardShortcutHint
                      shortcut='X'
                      show={showKeyboardShortcuts}
                    />
                    <CancelRounded onClick={handleSearchClose} />
                  </>
                )}
              </Box>
            }
          />

          {/* {activeTextField != 'search' && (
            <IconButton
              variant='outlined'
              color='neutral'
              size='sm'
              sx={{
                height: 24,
                borderRadius: 24,
              }}
              onClick={() => {
                setActiveTextFieldWithCache('search')
                setSearchInputFocus(searchInputFocus + 1)

                searchInputRef?.current?.focus()
              }}
            >
              <Search />
            </IconButton>
          )} */}
          <SortAndGrouping
            title='Group by'
            k={'icon-menu-group-by'}
            icon={<Sort />}
            selectedItem={selectedChoreSection}
            selectedFilter={selectedChoreFilter}
            setFilter={filter => {
              setSelectedChoreFilterWithCache(filter)
              const section = ChoresGrouper(
                selectedChoreSection,
                chores,
                ChoreFilters(userProfile)[filter],
              )
              setChoreSections(section)
              setOpenChoreSectionsWithCache(
                // open all sections by default
                Object.keys(section).reduce((acc, key) => {
                  acc[key] = true
                  return acc
                }, {}),
              )
            }}
            onItemSelect={selected => {
              const section = ChoresGrouper(
                selected.value,
                chores,
                ChoreFilters(userProfile)[selectedChoreFilter],
              )
              setChoreSections(section)
              setSelectedChoreSectionWithCache(selected.value)
              setOpenChoreSectionsWithCache(
                // open all sections by default
                Object.keys(section).reduce((acc, key) => {
                  acc[key] = true
                  return acc
                }, {}),
              )
              setFilteredChores(chores)
              setSearchFilter('All')
            }}
            mouseClickHandler={handleMenuOutsideClick}
          />

          {/* View Mode Toggle Button */}
          <IconButton
            variant='outlined'
            color='neutral'
            size='sm'
            sx={{
              height: 32,
              width: 32,
              borderRadius: '50%',
            }}
            onClick={toggleViewMode}
            title={
              isCompactView ? 'Switch to Card View' : 'Switch to Compact View'
            }
          >
            {isCompactView ? <ViewModule /> : <ViewAgenda />}
          </IconButton>

          {/* Multi-select Toggle Button */}
          <Box sx={{ position: 'relative', display: 'inline-flex' }}>
            <IconButton
              variant={isMultiSelectMode ? 'solid' : 'outlined'}
              color={isMultiSelectMode ? 'primary' : 'neutral'}
              size='sm'
              sx={{
                height: 32,
                width: 32,
                borderRadius: '50%',
              }}
              onClick={toggleMultiSelectMode}
              title={
                isMultiSelectMode
                  ? 'Exit Multi-select Mode (Ctrl+S)'
                  : 'Enable Multi-select Mode (Ctrl+S)'
              }
            >
              {isMultiSelectMode ? <CheckBox /> : <CheckBoxOutlineBlank />}
            </IconButton>
            <KeyboardShortcutHint
              shortcut='S'
              show={showKeyboardShortcuts}
              sx={{
                position: 'absolute',
                top: -8,
                right: -8,
                zIndex: 1000,
              }}
            />
          </Box>
        </Box>

        {/* Search Filter with animation */}
        <Box
          sx={{
            overflow: 'hidden',
            transition: 'all 0.3s ease-in-out',
            maxHeight: showSearchFilter ? '150px' : '0',
            opacity: showSearchFilter ? 1 : 0,
            transform: showSearchFilter ? 'translateY(0)' : 'translateY(-10px)',
            marginBottom: showSearchFilter ? 1 : 0,
          }}
        >
          <div className='flex gap-4'>
            <div className='grid flex-1 grid-cols-3 gap-4'>
              <IconButtonWithMenu
                label={' Priority'}
                k={'icon-menu-priority-filter'}
                icon={<PriorityHigh />}
                options={Priorities}
                selectedItem={searchFilter}
                onItemSelect={selected => {
                  handleLabelFiltering({ priority: selected.value })
                }}
                mouseClickHandler={handleMenuOutsideClick}
                isActive={searchFilter.startsWith('Priority: ')}
              />

              <IconButtonWithMenu
                k={'icon-menu-labels-filter'}
                label={' Labels'}
                icon={<Style />}
                options={userLabels}
                selectedItem={searchFilter}
                onItemSelect={selected => {
                  handleLabelFiltering({ label: selected })
                }}
                isActive={searchFilter.startsWith('Label: ')}
                mouseClickHandler={handleMenuOutsideClick}
                useChips
              />

              <Button
                onClick={handleFilterMenuOpen}
                variant='outlined'
                startDecorator={<Grain />}
                color={
                  searchFilter && FILTERS[searchFilter] && searchFilter != 'All'
                    ? 'primary'
                    : 'neutral'
                }
                size='sm'
                sx={{
                  height: 24,
                  borderRadius: 24,
                }}
              >
                {' Other'}
              </Button>

              <List
                orientation='horizontal'
                wrap
                sx={{
                  mt: 0.2,
                }}
              >
                <Menu
                  ref={menuRef}
                  anchorEl={anchorEl}
                  open={Boolean(anchorEl)}
                  onClose={handleFilterMenuClose}
                >
                  {Object.keys(FILTERS).map((filter, index) => (
                    <MenuItem
                      key={`filter-list-${filter}-${index}`}
                      onClick={() => {
                        const filterFunction = FILTERS[filter]
                        const filteredChores =
                          filterFunction.length === 2
                            ? filterFunction(chores, userProfile.id)
                            : filterFunction(chores)
                        setFilteredChores(filteredChores)
                        setSearchFilter(filter)
                        handleFilterMenuClose()
                      }}
                    >
                      {filter}
                      <Chip
                        color={searchFilter === filter ? 'primary' : 'neutral'}
                      >
                        {FILTERS[filter].length === 2
                          ? FILTERS[filter](chores, userProfile.id).length
                          : FILTERS[filter](chores).length}
                      </Chip>
                    </MenuItem>
                  ))}

                  {searchFilter.startsWith('Label: ') ||
                    (searchFilter.startsWith('Priority: ') && (
                      <MenuItem
                        key={`filter-list-cancel-all-filters`}
                        onClick={() => {
                          setFilteredChores(chores)
                          setSearchFilter('All')
                        }}
                      >
                        Cancel All Filters
                      </MenuItem>
                    ))}
                </Menu>
              </List>
            </div>
            <IconButton
              variant='outlined'
              color='neutral'
              size='sm'
              sx={{
                height: 24,
                borderRadius: 24,
              }}
              onClick={() => {
                setShowSearchFilter(false)
                setSearchTerm('')
                setFilteredChores(chores)
                setSearchFilter('All')
              }}
            >
              <CancelRounded />
            </IconButton>
          </div>
        </Box>

        {/* Multi-select Toolbar with animation */}
        <Box
          sx={{
            position: 'sticky',
            top: 0,
            zIndex: 1000,
            overflow: 'hidden',
            transition: 'all 0.3s ease-in-out',
            maxHeight: isMultiSelectMode ? '200px' : '0',
            opacity: isMultiSelectMode ? 1 : 0,
            transform: isMultiSelectMode
              ? 'translateY(0)'
              : 'translateY(-20px)',
            marginBottom: isMultiSelectMode ? 2 : 0,
          }}
        >
          <Box
            sx={{
              backgroundColor: 'background.surface',
              backdropFilter: 'blur(8px)',
              borderRadius: 'lg',
              p: 2,
              border: '1px solid',
              borderColor: 'divider',
              boxShadow: 'm',
              gap: 2,
              display: 'flex',
              flexDirection: {
                sm: 'column', // Stack vertically on mobile
                md: 'row', // Horizontal on tablet and larger
              },
              alignItems: {
                xs: 'stretch', // Full width on mobile
                sm: 'center', // Center aligned on larger screens
              },
              justifyContent: {
                xs: 'center',
                sm: 'space-between',
              },
            }}
          >
            {/* Selection Info and Controls */}
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 2,
                flexWrap: {
                  xs: 'wrap', // Allow wrapping on mobile if needed
                  sm: 'nowrap',
                },
                justifyContent: {
                  xs: 'center', // Center on mobile
                  sm: 'flex-start',
                },
              }}
            >
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <CheckBox sx={{ color: 'primary.500' }} />
                <Typography level='body-sm' fontWeight='md'>
                  {selectedChores.size} task
                  {selectedChores.size !== 1 ? 's' : ''} selected
                </Typography>
              </Box>

              <Divider
                orientation='vertical'
                sx={{
                  display: { xs: 'none', sm: 'block' }, // Hide vertical divider on mobile
                }}
              />

              <Box sx={{ display: 'flex', gap: 1 }}>
                <Button
                  size='sm'
                  variant='outlined'
                  onClick={selectAllVisibleChores}
                  startDecorator={<SelectAll />}
                  disabled={
                    searchTerm?.length > 0 || searchFilter !== 'All'
                      ? selectedChores.size === filteredChores.length
                      : selectedChores.size ===
                        choreSections.flatMap(s => s.content || []).length
                  }
                  sx={{
                    minWidth: 'auto',
                    '--Button-paddingInline': '0.75rem',
                    position: 'relative',
                  }}
                  title='Select all visible tasks (Ctrl+A)'
                >
                  All
                  {showKeyboardShortcuts && (
                    <KeyboardShortcutHint
                      shortcut='A'
                      sx={{
                        position: 'absolute',
                        top: -8,
                        right: -8,
                        zIndex: 1000,
                      }}
                    />
                  )}
                </Button>
                <Button
                  size='sm'
                  variant='outlined'
                  onClick={clearSelection}
                  startDecorator={
                    selectedChores.size === 0 ? (
                      <Close />
                    ) : (
                      <CheckBoxOutlineBlank />
                    )
                  }
                  sx={{
                    minWidth: 'auto',
                    '--Button-paddingInline': '0.75rem',
                    position: 'relative',
                  }}
                  title={`${selectedChores.size === 0 ? 'Close' : 'Clear'} multi-select (Esc)`}
                >
                  {selectedChores.size === 0 ? 'Close' : 'Clear'}
                  {showKeyboardShortcuts && (
                    <KeyboardShortcutHint
                      withCtrl={false}
                      shortcut='Esc'
                      sx={{
                        position: 'absolute',
                        top: -8,
                        right: -8,
                        zIndex: 1000,
                      }}
                    />
                  )}
                </Button>
              </Box>
            </Box>

            {/* Action Buttons */}
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                flexWrap: {
                  xs: 'wrap', // Allow wrapping on mobile
                  sm: 'nowrap',
                },
                justifyContent: {
                  xs: 'center', // Center on mobile
                  sm: 'flex-end',
                },
              }}
            >
              <Button
                size='sm'
                variant='solid'
                color='success'
                onClick={handleBulkComplete}
                startDecorator={<Done />}
                disabled={selectedChores.size === 0}
                sx={{
                  '--Button-paddingInline': { xs: '0.75rem', sm: '1rem' },
                  position: 'relative',
                }}
                title='Complete selected tasks (Enter)'
              >
                Complete
                {showKeyboardShortcuts && selectedChores.size > 0 && (
                  <KeyboardShortcutHint
                    shortcut='Enter'
                    sx={{
                      position: 'absolute',
                      top: -8,
                      right: -8,
                      zIndex: 1000,
                    }}
                  />
                )}
              </Button>
              <Button
                size='sm'
                variant='soft'
                color='warning'
                onClick={handleBulkSkip}
                startDecorator={<SkipNext />}
                disabled={selectedChores.size === 0}
                sx={{
                  '--Button-paddingInline': { xs: '0.75rem', sm: '1rem' },
                  position: 'relative',
                }}
                title='Skip selected tasks (/)'
              >
                Skip
                {showKeyboardShortcuts && selectedChores.size > 0 && (
                  <KeyboardShortcutHint
                    shortcut='/'
                    sx={{
                      position: 'absolute',
                      top: -8,
                      right: -8,
                      zIndex: 1000,
                    }}
                  />
                )}
              </Button>
              <Button
                size='sm'
                variant='soft'
                color='danger'
                onClick={handleBulkArchive}
                startDecorator={<Archive />}
                disabled={selectedChores.size === 0}
                sx={{
                  '--Button-paddingInline': { xs: '0.75rem', sm: '1rem' },
                  position: 'relative',
                }}
                title='Archive selected tasks (X)'
              >
                Archive
                {showKeyboardShortcuts && selectedChores.size > 0 && (
                  <KeyboardShortcutHint
                    shortcut='X'
                    sx={{
                      position: 'absolute',
                      top: -8,
                      right: -8,
                      zIndex: 1000,
                    }}
                  />
                )}
              </Button>

              <Button
                size='sm'
                variant='soft'
                color='danger'
                onClick={handleBulkDelete}
                startDecorator={<Delete />}
                disabled={selectedChores.size === 0}
                sx={{
                  '--Button-paddingInline': { xs: '0.75rem', sm: '1rem' },
                  position: 'relative',
                }}
                title='Delete selected tasks (Shift+X)'
              >
                Delete
                {showKeyboardShortcuts && selectedChores.size > 0 && (
                  <KeyboardShortcutHint
                    shortcut='E'
                    sx={{
                      position: 'absolute',
                      top: -8,
                      right: -8,
                      zIndex: 1000,
                    }}
                  />
                )}
              </Button>

              {/* 
              <Divider
                orientation='vertical'
                sx={{
                  display: { xs: 'none', sm: 'block' }, // Hide vertical divider on mobile
                }}
              />

              <IconButton
                size='sm'
                variant='plain'
                onClick={toggleMultiSelectMode}
                color='neutral'
                title='Exit multi-select mode (Esc)'
                sx={{
                  '&:hover': {
                    bgcolor: 'danger.softBg',
                    color: 'danger.softColor',
                  },
                }}
              >
                <CancelRounded />
              </IconButton> */}
            </Box>
          </Box>
        </Box>

        {searchFilter !== 'All' && (
          <Chip
            level='title-md'
            gutterBottom
            color='warning'
            label={searchFilter}
            onDelete={() => {
              setFilteredChores(chores)
              setSearchFilter('All')
            }}
            endDecorator={<CancelRounded />}
            onClick={() => {
              setFilteredChores(chores)
              setSearchFilter('All')
            }}
          >
            Current Filter: {searchFilter}
          </Chip>
        )}
        {filteredChores.length === 0 && archivedChores == null && (
          <Box
            sx={{
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              flexDirection: 'column',
              height: '50vh',
            }}
          >
            <EditCalendar
              sx={{
                fontSize: '4rem',
                // color: 'text.disabled',
                mb: 1,
              }}
            />
            <Typography level='title-md' gutterBottom>
              Nothing scheduled
            </Typography>
            {chores.length > 0 && (
              <>
                <Button
                  onClick={() => {
                    setFilteredChores(chores)
                    setSearchTerm('')
                  }}
                  variant='outlined'
                  color='neutral'
                >
                  Reset filters
                </Button>
              </>
            )}
          </Box>
        )}
        {(searchTerm?.length > 0 || searchFilter !== 'All') &&
          filteredChores.map(chore =>
            renderChoreCard(chore, `filtered-${chore.id}`),
          )}
        {searchTerm.length === 0 && searchFilter === 'All' && (
          <AccordionGroup transition='0.2s ease' disableDivider>
            {choreSections.map((section, index) => {
              if (section.content.length === 0) return null
              return (
                <Accordion
                  key={section.name + index}
                  sx={{
                    my: 0,
                    px: 0,
                  }}
                  expanded={Boolean(openChoreSections[index])}
                >
                  <Divider orientation='horizontal'>
                    <Chip
                      variant='soft'
                      color='neutral'
                      size='md'
                      onClick={() => {
                        if (openChoreSections[index]) {
                          const newOpenChoreSections = {
                            ...openChoreSections,
                          }
                          delete newOpenChoreSections[index]
                          setOpenChoreSectionsWithCache(newOpenChoreSections)
                        } else {
                          setOpenChoreSectionsWithCache({
                            ...openChoreSections,
                            [index]: true,
                          })
                        }
                      }}
                      endDecorator={
                        openChoreSections[index] ? (
                          <ExpandCircleDown
                            color='primary'
                            sx={{ transform: 'rotate(180deg)' }}
                          />
                        ) : (
                          <ExpandCircleDown color='primary' />
                        )
                      }
                      startDecorator={
                        <>
                          <Chip color='primary' size='sm' variant='soft'>
                            {section?.content?.length}
                          </Chip>
                        </>
                      }
                    >
                      {section.name}
                    </Chip>
                  </Divider>
                  <AccordionDetails
                    sx={{
                      flexDirection: 'column',
                      ['& > *']: {
                        // px: 0.5,
                        px: 0.5,
                        // pr: 0,
                      },
                    }}
                  >
                    {section.content?.map(chore => renderChoreCard(chore))}
                  </AccordionDetails>
                </Accordion>
              )
            })}
          </AccordionGroup>
        )}
        <Box
          sx={{
            // center the button
            justifyContent: 'center',
            mt: 2,
          }}
        >
          {archivedChores === null && (
            <Box sx={{ display: 'flex', justifyContent: 'center' }}>
              <Button
                sx={{}}
                onClick={() => {
                  GetArchivedChores()
                    .then(response => response.json())
                    .then(data => {
                      setArchivedChores(data.res)
                    })
                }}
                variant='outlined'
                color='neutral'
                startDecorator={<Unarchive />}
                endDecorator={
                  <KeyboardShortcutHint
                    shortcut='O'
                    show={showKeyboardShortcuts}
                  />
                }
              >
                Show Archived
              </Button>
            </Box>
          )}
          {archivedChores !== null && (
            <>
              <Divider orientation='horizontal'>
                <Chip
                  variant='soft'
                  color='danger'
                  size='md'
                  startDecorator={
                    <>
                      <Chip color='danger' size='sm' variant='plain'>
                        {archivedChores?.length}
                      </Chip>
                    </>
                  }
                >
                  Archived
                </Chip>
              </Divider>

              {archivedChores?.map(chore => renderChoreCard(chore))}
            </>
          )}
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
            'z-index': 100,
          }}
        >
          <IconButton
            color='primary'
            variant='solid'
            sx={{
              borderRadius: '50%',
              width: 50,
              height: 50,
              zIndex: 101,
              position: 'relative',
            }}
            onClick={() => {
              Navigate(`/chores/create`)
            }}
            title='Create new chore (Cmd+C)'
          >
            <Add />
            <KeyboardShortcutHint
              sx={{
                position: 'absolute',
                top: -8,
                right: -8,
                zIndex: 1000,
              }}
              show={showKeyboardShortcuts}
              shortcut='J'
            />
          </IconButton>
          <IconButton
            color='primary'
            variant='soft'
            sx={{
              borderRadius: '50%',
              width: 25,
              height: 25,
              position: 'relative',
              left: -25,
              top: 22,
            }}
            onClick={() => {
              setAddTaskModalOpen(true)
            }}
          >
            <Bolt
              style={{
                rotate: '20deg',
              }}
            />
          </IconButton>

          <KeyboardShortcutHint
            sx={{ position: 'relative', left: -40, top: 30 }}
            show={showKeyboardShortcuts}
            shortcut='K'
          />
        </Box>
        <NotificationAccessSnackbar />
        {addTaskModalOpen && (
          <TaskInput
            autoFocus={taskInputFocus}
            onChoreUpdate={updateChores}
            isModalOpen={addTaskModalOpen}
            onClose={forceRefresh => {
              setAddTaskModalOpen(false)
              if (forceRefresh) {
                refetchChores()
              }
            }}
          />
        )}
      </Container>

      <Sidepanel chores={chores} performers={performers} />

      {/* Multi-select Help - only show when in multi-select mode */}
      <MultiSelectHelp isVisible={isMultiSelectMode} />

      {/* Confirmation Modal for bulk operations */}
      {confirmModelConfig?.isOpen && (
        <ConfirmationModal config={confirmModelConfig} />
      )}
    </div>
  )
}

const FILTERS = {
  All: function (chores) {
    return chores
  },
  Overdue: function (chores) {
    return chores.filter(chore => {
      if (chore.nextDueDate === null) return false
      return new Date(chore.nextDueDate) < new Date()
    })
  },
  'Due today': function (chores) {
    return chores.filter(chore => {
      return (
        new Date(chore.nextDueDate).toDateString() === new Date().toDateString()
      )
    })
  },
  'Due in week': function (chores) {
    return chores.filter(chore => {
      return (
        new Date(chore.nextDueDate) <
          new Date(Date.now() + 7 * 24 * 60 * 60 * 1000) &&
        new Date(chore.nextDueDate) > new Date()
      )
    })
  },
  'Due Later': function (chores) {
    return chores.filter(chore => {
      return (
        new Date(chore.nextDueDate) > new Date(Date.now() + 24 * 60 * 60 * 1000)
      )
    })
  },
  'Created By Me': function (chores, userID) {
    return chores.filter(chore => {
      return chore.createdBy === userID
    })
  },
  'Assigned To Me': function (chores, userID) {
    return chores.filter(chore => {
      return chore.assignedTo === userID
    })
  },
  'No Due Date': function (chores) {
    return chores.filter(chore => {
      return chore.nextDueDate === null
    })
  },
}

export default MyChores
