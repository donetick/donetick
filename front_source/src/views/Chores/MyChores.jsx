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
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import KeyboardShortcutHint from '../../components/common/KeyboardShortcutHint'
import { useImpersonateUser } from '../../contexts/ImpersonateUserContext.jsx'
import { useChores } from '../../queries/ChoreQueries'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries'
import { useNotification } from '../../service/NotificationProvider'
import { ChoreFilters, ChoresGrouper, ChoreSorter } from '../../utils/Choores'
import {
  ArchiveChore,
  DeleteChore,
  GetArchivedChores,
  MarkChoreComplete,
  SkipChore,
} from '../../utils/Fetcher'
import Priorities from '../../utils/Priorities'
import LoadingComponent from '../components/Loading'
import TaskInput from '../components/AddTaskModal'
import { useLabels } from '../Labels/LabelQueries'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import ChoreCard from './ChoreCard'
import CompactChoreCard from './CompactChoreCard'
import IconButtonWithMenu from './IconButtonWithMenu'
import {
  canScheduleNotification,
  scheduleChoreNotification,
} from './LocalNotificationScheduler'
import MultiSelectHelp from './MultiSelectHelp'
import NotificationAccessSnackbar from './NotificationAccessSnackbar'
import Sidepanel from './Sidepanel'
import SortAndGrouping from './SortAndGrouping'

const MyChores = () => {
  const { t } = useTranslation()
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

  useEffect(() => {
    const handleKeyDown = event => {
      if (addTaskModalOpen) return
      if (event.ctrlKey || event.metaKey) {
        setShowKeyboardShortcuts(true)
      }
      const isHoldingCmdOrCtrl = event.ctrlKey || event.metaKey
      if (isHoldingCmdOrCtrl && event.key === 'k') {
        event.preventDefault()
        setAddTaskModalOpen(true)
        return
      }

      if (addTaskModalOpen) {
        return
      }

      if (isHoldingCmdOrCtrl && event.key === 'j') {
        event.preventDefault()
        Navigate(`/chores/create`)
        return
      } else if (isHoldingCmdOrCtrl && event.key === 'f') {
        event.preventDefault()
        searchInputRef.current?.focus()
        return
      } else if (isHoldingCmdOrCtrl && event.key === 'x') {
        event.preventDefault()
        if (searchTerm?.length > 0) {
          handleSearchClose()
        }
      } else if (isHoldingCmdOrCtrl && event.key === 's') {
        event.preventDefault()
        toggleMultiSelectMode()
        return
      } else if (
        isHoldingCmdOrCtrl &&
        event.key === 'a' &&
        !['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)
      ) {
        event.preventDefault()
        if (!isMultiSelectMode) {
          setIsMultiSelectMode(true)
          setTimeout(() => {
            selectAllVisibleChores()
          }, 0)
        } else {
          let visibleChores = []

          if (searchTerm?.length > 0 || searchFilter !== 'All') {
            visibleChores = filteredChores
            const allVisibleSelected =
              visibleChores.length > 0 &&
              visibleChores.every(chore => selectedChores.has(chore.id))

            if (allVisibleSelected) {
              showSuccess({
                title: t('myChores.allTasksSelected'),
                message: t('myChores.allTasksSelectedSuccess', {
                  count: visibleChores.length,
                }),
              })
            } else {
              selectAllVisibleChores()
              showSuccess({
                title: t('myChores.tasksSelected'),
                message: t('myChores.tasksSelectedSuccess', {
                  count: visibleChores.length,
                }),
              })
            }
          } else {
            const expandedChores = choreSections
              .filter((section, index) => openChoreSections[index])
              .flatMap(section => section.content || [])

            const allExpandedSelected =
              expandedChores.length > 0 &&
              expandedChores.every(chore => selectedChores.has(chore.id))

            const allChores = choreSections.flatMap(
              section => section.content || [],
            )
            const allChoresSelected =
              allChores.length > 0 &&
              allChores.every(chore => selectedChores.has(chore.id))

            if (allChoresSelected) {
              showSuccess({
                title: t('myChores.allTasksSelected'),
                message: t('myChores.allTasksSelectedWithCollapsed', {
                  count: allChores.length,
                }),
              })
            } else if (allExpandedSelected) {
              selectAllVisibleChores()
              const collapsedCount = allChores.length - expandedChores.length
              showSuccess({
                title: t('myChores.allTasksSelected'),
                message: t(
                  'myChores.allTasksSelectedWithCollapsedSuccess',
                  {
                    count: allChores.length,
                    collapsedCount: collapsedCount,
                  },
                ),
              })
            } else {
              selectAllVisibleChores()
              showSuccess({
                title: t('myChores.tasksSelected'),
                message: t('myChores.tasksSelectedFromExpanded', {
                  count: expandedChores.length,
                }),
              })
            }
          }
        }
      }

      if (isMultiSelectMode) {
        if (event.key === 'Escape') {
          event.preventDefault()
          if (selectedChores.size > 0) {
            clearSelection()
          } else {
            setIsMultiSelectMode(false)
          }
          return
        }

        if (
          isHoldingCmdOrCtrl &&
          event.key === 'Enter' &&
          selectedChores.size > 0
        ) {
          event.preventDefault()
          handleBulkComplete()
          return
        }

        if (
          isHoldingCmdOrCtrl &&
          event.key === '/' &&
          selectedChores.size > 0
        ) {
          event.preventDefault()
          handleBulkSkip()
          return
        }

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
          title: t('myChores.taskCompleted'),
          message: t('myChores.taskCompletedSuccess'),
        })
        break
      case 'skipped':
        showSuccess({
          title: t('myChores.taskSkipped'),
          message: t('myChores.taskSkippedSuccess'),
        })
        break
      case 'rescheduled':
        showSuccess({
          title: t('myChores.taskRescheduled'),
          message: t('myChores.taskRescheduledSuccess'),
        })
        break
      case 'unarchive':
        showSuccess({
          title: t('myChores.taskRestored'),
          message: t('myChores.taskRestoredSuccess'),
        })
        break
      case 'archive':
        showSuccess({
          title: t('myChores.taskArchived'),
          message: t('myChores.taskArchivedSuccess'),
        })
        break
      case 'started':
        showSuccess({
          title: t('myChores.taskStarted'),
          message: t('myChores.taskStartedSuccess'),
        })
        break
      case 'paused':
        showWarning({
          title: t('myChores.taskPaused'),
          message: t('myChores.taskPausedSuccess'),
        })
        break
      case 'deleted':
      default:
        showSuccess({
          title: t('myChores.taskUpdated'),
          message: t('myChores.taskUpdatedSuccess'),
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
    keys: ['name', 'raw_label'],
    includeScore: true,
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
    setSearchInputFocus(0)
  }

  const toggleMultiSelectMode = () => {
    const newMode = !isMultiSelectMode
    setIsMultiSelectMode(newMode)

    if (newMode) {
      setSelectedChores(new Set())
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
      visibleChores = filteredChores
    } else {
      const expandedChores = choreSections
        .filter((section, index) => openChoreSections[index])
        .flatMap(section => section.content || [])

      const allExpandedSelected =
        expandedChores.length > 0 &&
        expandedChores.every(chore => selectedChores.has(chore.id))

      if (allExpandedSelected) {
        visibleChores = choreSections.flatMap(section => section.content || [])
      } else {
        visibleChores = expandedChores
      }
    }

    if (visibleChores.length > 0) {
      const allIds = new Set(visibleChores.map(chore => chore.id))
      setSelectedChores(allIds)
    }
  }

  const clearSelection = () => {
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

  const handleBulkComplete = async () => {
    const selectedData = getSelectedChoresData()
    if (selectedData.length === 0) return

    setConfirmModelConfig({
      isOpen: true,
      title: t('myChores.completeTasks'),
      confirmText: t('myChores.complete'),
      cancelText: t('myChores.cancel'),
      message: t('myChores.completeTasksConfirmation', {
        count: selectedData.length,
      }),
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
                title: t('myChores.tasksCompleted'),
                message: t('myChores.tasksCompletedSuccess', {
                  count: completedTasks.length,
                }),
              })
            }

            if (failedTasks.length > 0) {
              showError({
                title: t('myChores.someTasksFailed'),
                message: t('myChores.someTasksFailedToComplete', {
                  count: failedTasks.length,
                }),
              })
            }

            refetchChores()
            clearSelection()
          } catch (error) {
            showError({
              title: t('myChores.bulkCompleteFailed'),
              message: t('myChores.bulkCompleteFailedError'),
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
      title: t('myChores.archiveTasks'),
      confirmText: t('myChores.archive'),
      cancelText: t('myChores.cancel'),
      message: t('myChores.archiveTasksConfirmation', {
        count: selectedData.length,
      }),
      onClose: async isConfirmed => {
        if (isConfirmed === true) {
          try {
            const archivedTasks = []
            const failedTasks = []
            for (const chore of selectedData) {
              try {
                const archivedChore = await ArchiveChore(chore.id)
                archivedTasks.push(archivedChore)
                setChores(chores.filter(c => c.id !== chore.id))
                setFilteredChores(filteredChores.filter(c => c.id !== chore.id))
              } catch (error) {
                failedTasks.push(chore)
              }
            }
            if (archivedTasks.length > 0) {
              showSuccess({
                title: t('myChores.tasksArchived'),
                message: t('myChores.tasksArchivedSuccess', {
                  count: archivedTasks.length,
                }),
              })
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
                title: t('myChores.someTasksFailed'),
                message: t('myChores.someTasksFailedToArchive', {
                  count: failedTasks.length,
                }),
              })
            }
            clearSelection()
          } catch (error) {
            showError({
              title: t('myChores.bulkArchiveFailed'),
              message: t('myChores.bulkArchiveFailedError'),
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
      title: t('myChores.deleteTasks'),
      confirmText: t('myChores.delete'),
      cancelText: t('myChores.cancel'),
      message: t('myChores.deleteTasksConfirmation', {
        count: selectedData.length,
      }),
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
                title: t('myChores.tasksDeleted'),
                message: t('myChores.tasksDeletedSuccess', {
                  count: deletedTasks.length,
                }),
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
                title: t('myChores.someTasksFailed'),
                message: t('myChores.someTasksFailedToDelete', {
                  count: failedTasks.length,
                }),
              })
            }

            clearSelection()
          } catch (error) {
            showError({
              title: t('myChores.bulkDeleteFailed'),
              message: t('myChores.bulkDeleteFailedError'),
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
      title: t('myChores.skipTasks'),
      confirmText: t('myChores.skip'),
      cancelText: t('myChores.cancel'),
      message: t('myChores.skipTasksConfirmation', {
        count: selectedData.length,
      }),
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
                title: t('myChores.tasksSkipped'),
                message: t('myChores.tasksSkippedSuccess', {
                  count: skippedTasks.length,
                }),
              })
            }

            if (failedTasks.length > 0) {
              showError({
                title: t('myChores.someTasksFailed'),
                message: t('myChores.someTasksFailedToSkip', {
                  count: failedTasks.length,
                }),
              })
            }

            refetchChores()
            clearSelection()
          } catch (error) {
            showError({
              title: t('myChores.bulkSkipFailed'),
              message: t('myChores.bulkSkipFailedError'),
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
            placeholder={t('myChores.search')}
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
          <SortAndGrouping
            title={t('myChores.groupBy')}
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
              isCompactView
                ? t('myChores.switchToCardView')
                : t('myChores.switchToCompactView')
            }
          >
            {isCompactView ? <ViewModule /> : <ViewAgenda />}
          </IconButton>
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
                  ? t('myChores.exitMultiSelectMode')
                  : t('myChores.enableMultiSelectMode')
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
                label={t('myChores.priority')}
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
                label={t('myChores.labels')}
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
                {t('myChores.other')}
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

                  {(searchFilter.startsWith('Label: ') ||
                    searchFilter.startsWith('Priority: ')) && (
                    <MenuItem
                      key={`filter-list-cancel-all-filters`}
                      onClick={() => {
                        setFilteredChores(chores)
                        setSearchFilter('All')
                      }}
                    >
                      {t('myChores.cancelAllFilters')}
                    </MenuItem>
                  )}
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
                sm: 'column',
                md: 'row',
              },
              alignItems: {
                xs: 'stretch',
                sm: 'center',
              },
              justifyContent: {
                xs: 'center',
                sm: 'space-between',
              },
            }}
          >
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 2,
                flexWrap: {
                  xs: 'wrap',
                  sm: 'nowrap',
                },
                justifyContent: {
                  xs: 'center',
                  sm: 'flex-start',
                },
              }}
            >
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <CheckBox sx={{ color: 'primary.500' }} />
                <Typography level='body-sm' fontWeight='md'>
                  {t('myChores.tasksSelectedMessage', {
                    count: selectedChores.size,
                  })}
                </Typography>
              </Box>

              <Divider
                orientation='vertical'
                sx={{
                  display: { xs: 'none', sm: 'block' },
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
                  {t('myChores.selectAll')}
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
                  title={`${
                    selectedChores.size === 0
                      ? t('myChores.close')
                      : t('myChores.clear')
                  } multi-select (Esc)`}
                >
                  {selectedChores.size === 0
                    ? t('myChores.close')
                    : t('myChores.clear')}
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
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                flexWrap: {
                  xs: 'wrap',
                  sm: 'nowrap',
                },
                justifyContent: {
                  xs: 'center',
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
                {t('myChores.complete')}
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
                {t('myChores.skip')}
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
                {t('myChores.archive')}
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
                {t('myChores.delete')}
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
            {t('myChores.currentFilter', { filter: searchFilter })}
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
                mb: 1,
              }}
            />
            <Typography level='title-md' gutterBottom>
              {t('myChores.nothingScheduled')}
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
                  {t('myChores.resetFilters')}
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
                        px: 0.5,
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
                {t('myChores.showArchived')}
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
                  {t('myChores.archived')}
                </Chip>
              </Divider>

              {archivedChores?.map(chore => renderChoreCard(chore))}
            </>
          )}
        </Box>
        <Box
          sx={{
            position: 'fixed',
            bottom: 0,
            left: 10,
            p: 2,
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
            title={t('myChores.createNewChore')}
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
      <MultiSelectHelp isVisible={isMultiSelectMode} />
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
