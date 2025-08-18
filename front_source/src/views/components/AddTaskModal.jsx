import { Add, EditNotifications } from '@mui/icons-material'
import { Box, Button, Chip, Input, Option, Select, Typography } from '@mui/joy'
import { FormControl } from '@mui/material'
import * as chrono from 'chrono-node'
import moment from 'moment'
import { useCallback, useEffect, useRef, useState } from 'react'
import FadeModal from '../../components/common/FadeModal'
import { useCreateChore } from '../../queries/ChoreQueries'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries'
import { isPlusAccount } from '../../utils/Helpers'
import { useLabels } from '../Labels/LabelQueries'
import {
  parseDueDate,
  parseLabels,
  parsePriority,
  parseRepeatV2,
} from './CustomParsers'
import SmartTaskTitleInput from './SmartTaskTitleInput'

import KeyboardShortcutHint from '../../components/common/KeyboardShortcutHint'
import NotificationTemplate from '../../components/NotificationTemplate'
import LearnMoreButton from './LearnMore'
import RichTextEditor from './RichTextEditor'
import SubTasks from './SubTask'
const getDefaultNotification = () => {
  const storedDefault = localStorage.getItem('defaultNotificationTemplate')
  if (storedDefault) {
    return JSON.parse(storedDefault)
  }
  const defaultNotification = [
    { value: 1, unit: 'days', type: 'before' },
    { value: 0, unit: 'minutes', type: 'ondue' },
    { value: 1, unit: 'days', type: 'after' },
  ]

  localStorage.setItem(
    'defaultNotification',
    JSON.stringify(defaultNotification),
  )
  return defaultNotification
}

const TaskInput = ({ autoFocus, onChoreUpdate, isModalOpen, onClose }) => {
  const { data: userLabels, isLoading: userLabelsLoading } = useLabels()
  const { data: circleMembers, isLoading: isCircleMembersLoading } =
    useCircleMembers()
  const createChoreMutation = useCreateChore()

  const { data: userProfile } = useUserProfile()

  const [taskText, setTaskText] = useState('')
  const [taskTitle, setTaskTitle] = useState('')
  const [renderedParts, setRenderedParts] = useState([])

  const textareaRef = useRef(null)
  const mainInputRef = useRef(null)
  const richTextEditorRef = useRef(null)
  const [priority, setPriority] = useState(0)
  const [dueDate, setDueDate] = useState(null)
  const [description, setDescription] = useState(null)
  const [assignees, setAssignees] = useState([])
  const [labelsV2, setLabelsV2] = useState([])
  const [frequency, setFrequency] = useState(null)
  const [notificationMetadata, setNotificationMetadata] = useState({
    templates: getDefaultNotification(),
  })
  const [frequencyHumanReadable, setFrequencyHumanReadable] = useState(null)
  const [subTasks, setSubTasks] = useState(null)
  const [hasDescription, setHasDescription] = useState(false)
  const [hasSubTasks, setHasSubTasks] = useState(false)
  const [hasNotifications, setHasNotifications] = useState(false)
  const [showKeyboardShortcuts, setShowKeyboardShortcuts] = useState(true)

  // set showKeyboardShortcuts true as soon as the user hold ctrl or cmd key:
  useEffect(() => {
    if (hasDescription && richTextEditorRef.current) {
      // Small delay to ensure the component is fully rendered
      setTimeout(() => {
        richTextEditorRef.current.focus()
      }, 100)
    }
  }, [hasDescription])

  // set showKeyboardShortcuts true as soon as the user hold ctrl or cmd key:
  useEffect(() => {
    const handleKeyDown = event => {
      const isHoldingCmd = event.ctrlKey || event.metaKey
      if (isHoldingCmd) {
        // event.preventDefault()
        setShowKeyboardShortcuts(true)
      }
      if (
        isHoldingCmd &&
        event.key.toLowerCase() === 'e' &&
        isModalOpen &&
        !hasDescription
      ) {
        setHasDescription(true)
        setShowKeyboardShortcuts(false)
      }
      if (isHoldingCmd && event.key.toLowerCase() === 'j' && isModalOpen) {
        // add subtask:
        setHasSubTasks(true)
        setShowKeyboardShortcuts(false)
        // set focus on the first subtask input:
      }
      if (
        isHoldingCmd &&
        event.key.toLowerCase() === 'b' &&
        isModalOpen &&
        !dueDate
      ) {
        // add due date:
        setDueDate(moment().add(1, 'day').format('YYYY-MM-DDTHH:00:00'))
        setShowKeyboardShortcuts(false)
      }
      // Enter key to create task
      if (
        event.key === 'Enter' &&
        (event.ctrlKey || event.metaKey) &&
        isModalOpen
      ) {
        event.preventDefault()
        createChore()
        return
      }
      // Escape key to cancel/close modal
      if (event.key === 'Escape' && isModalOpen) {
        event.preventDefault()
        handleCloseModal()
        return
      }
    }

    const handleKeyUp = event => {
      if (event.key === 'Control' || event.key === 'Meta') {
        setShowKeyboardShortcuts(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keyup', handleKeyUp)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('keyup', handleKeyUp)
    }
  }, [])

  useEffect(() => {
    if (isModalOpen && textareaRef.current) {
      textareaRef.current.focus()
      textareaRef.current.selectionStart = textareaRef.current.value?.length
      textareaRef.current.selectionEnd = textareaRef.current.value?.length
    }
  }, [isModalOpen])

  useEffect(() => {
    if (autoFocus > 0 && mainInputRef.current) {
      mainInputRef.current.focus()
      mainInputRef.current.selectionStart = mainInputRef.current.value?.length
      mainInputRef.current.selectionEnd = mainInputRef.current.value?.length
    }
  }, [autoFocus])

  const renderHighlightedSentence = useCallback(
    (
      sentence,
      repeatHighlight,
      priorityHighlight,
      labelsHighlight,
      dueDateHighlight,
    ) => {
      const parts = []
      let lastIndex = 0
      let plainText = ''

      // Combine all highlight ranges and sort them by their start index
      const allHighlights = []
      if (repeatHighlight) {
        repeatHighlight.forEach(h =>
          allHighlights.push({ ...h, type: 'repeat', priority: 40 }),
        )
      }
      if (priorityHighlight) {
        priorityHighlight.forEach(h =>
          allHighlights.push({ ...h, type: 'priority', priority: 30 }),
        )
      }
      if (labelsHighlight) {
        labelsHighlight.forEach(h =>
          allHighlights.push({ ...h, type: 'label', priority: 20 }),
        )
      }
      if (dueDateHighlight) {
        allHighlights.push({
          ...dueDateHighlight,
          type: 'dueDate',
          priority: 10,
        })
      }

      allHighlights.sort((a, b) => a.start - b.start)
      const resolvedHighlights = []
      for (let i = 0; i < allHighlights.length; i++) {
        const current = allHighlights[i]
        const previous = resolvedHighlights[resolvedHighlights.length - 1]

        if (previous && current.start < previous.end) {
          if (current.priority > previous.priority) {
            resolvedHighlights.pop()
            resolvedHighlights.push(current)
          }
        } else {
          // No overlap, add the current highlight
          resolvedHighlights.push(current)
        }
      }

      for (const highlight of resolvedHighlights) {
        // Add the text before the highlight
        if (highlight.start > lastIndex) {
          const textBefore = sentence.substring(lastIndex, highlight.start)
          parts.push(textBefore)
          plainText += textBefore
        }

        // Determine the class name based on the highlight type
        let className = ''
        switch (highlight.type) {
          case 'repeat':
            className = 'highlight-repeat'
            break
          case 'priority':
            className = 'highlight-priority'
            break
          case 'label':
            className = 'highlight-label'
            break
          case 'dueDate':
            className = 'highlight-date'
            break
          default:
            break
        }

        // Add the highlighted span
        const highlightedText = sentence.substring(
          highlight.start,
          highlight.end,
        )
        parts.push(
          <span
            key={highlight.start}
            className={className}
            style={{
              // text underline:
              textDecoration: 'underline',
              // textDecorationColor: 'red',
              textDecorationThickness: '2px',
              textDecorationStyle: 'dashed',
            }}
          >
            {highlightedText}
          </span>,
        )

        // Update the last index to the end of the current highlight
        lastIndex = highlight.end
      }

      // Add any remaining text after the last highlight
      if (lastIndex < sentence.length) {
        const remainingText = sentence.substring(lastIndex)
        parts.push(remainingText)
        plainText += remainingText
      }

      return {
        parts,
        plainText,
      }
    },
    [],
  )

  const processText = useCallback(
    sentence => {
      let cleanedSentence = sentence
      const priority = parsePriority(sentence)
      if (priority.result) setPriority(priority.result)
      cleanedSentence = priority.cleanedSentence
      const labels = parseLabels(sentence, userLabels)
      if (labels.result) {
        cleanedSentence = labels.cleanedSentence
        setLabelsV2(labels.result)
      }

      const repeat = parseRepeatV2(sentence)
      if (repeat.result) {
        setFrequency(repeat.result)
        setFrequencyHumanReadable(repeat.name)
        cleanedSentence = repeat.cleanedSentence
      }
      // Parse assignees using circle members
      // const circleMembersList = circleMembers?.res || []
      // const assigneesForParsing = circleMembersList.map(member => ({
      //   userId: member.userId,
      //   username:
      //     member.username ||
      //     member.displayName?.toLowerCase().replace(/\s+/g, ''),
      //   displayName: member.displayName,
      //   name: member.displayName,
      //   id: member.userId,
      // }))

      // const assigneesResult = parseAssignees(sentence, assigneesForParsing)
      // if (assigneesResult.result) {
      //   cleanedSentence = assigneesResult.cleanedSentence
      //   setAssignees(
      //     assigneesResult.result.map(assignee => ({
      //       userId: assignee.userId,
      //     })),
      //   )
      // } else {
      //   setAssignees([
      //     {
      //       userId: userProfile.id,
      //     },
      //   ])
      // }
      // Parse due date
      const dueDateParsed = parseDueDate(sentence, chrono)
      let dueDateHighlight = null
      if (dueDateParsed.result) {
        setDueDate(moment(dueDateParsed.result).format('YYYY-MM-DDTHH:mm:ss'))
        cleanedSentence = dueDateParsed.cleanedSentence
        dueDateHighlight = dueDateParsed.highlight[0]
      }

      if (repeat.result) {
        // if repeat has result the cleaned sentence will remove the date related info which mean
        // we need to reparse the date again to get the correct due date:
        const dueDateParsedAgain = parseDueDate(sentence, chrono)
        if (dueDateParsedAgain.result) {
          setDueDate(
            moment(dueDateParsedAgain.result).format('YYYY-MM-DDTHH:mm:ss'),
          )
        }
      }

      setTaskText(sentence)
      setTaskTitle(cleanedSentence.trim())
      const { parts, plainText } = renderHighlightedSentence(
        sentence,
        repeat.highlight,
        priority.highlight,
        labels.highlight,
        dueDateHighlight,
      )

      setRenderedParts(parts)
      setTaskTitle(plainText)
    },
    [userLabels, renderHighlightedSentence],
  )

  useEffect(() => {
    if (!isModalOpen || userLabelsLoading || isCircleMembersLoading) {
      return
    }

    processText(taskText)
  }, [
    taskText,
    userLabelsLoading,
    isCircleMembersLoading,
    isModalOpen,
    processText,
  ])

  const handleEnterPressed = () => {
    createChore()
  }

  const handleCloseModal = forceRefetch => {
    onClose(forceRefetch)
    setTaskText('')
    setTaskTitle('')
    setDueDate(null)
    setFrequency(null)
    setFrequencyHumanReadable(null)
    setPriority(0)
    setHasDescription(false)
    setDescription(null)
    setSubTasks(null)
    setHasSubTasks(false)
    setLabelsV2([])
    setAssignees([])
  }

  const createChore = () => {
    const chore = {
      name: taskTitle,
      assignees:
        assignees.length > 0 ? assignees : [{ userId: userProfile.id }],
      dueDate: dueDate ? new Date(dueDate).toISOString() : null,
      assignedTo: assignees.length > 0 ? assignees[0].userId : userProfile.id,
      assignStrategy: 'random',
      isRolling: false,

      labelsV2: labelsV2,
      priority: priority ? Number(priority) : 0,
      status: 0,
      frequencyType: 'once',
      frequencyMetadata: {},
      notificationMetadata: {},
      subTasks: subTasks?.length > 0 ? subTasks : null,
    }

    if (frequency) {
      chore.frequencyType = frequency.frequencyType
      chore.frequencyMetadata = frequency.frequencyMetadata
      chore.frequency = frequency.frequency
      if (isPlusAccount(userProfile)) {
        chore.notification = true
        chore.notificationMetadata = notificationMetadata
      }
    }
    if (!frequency && dueDate) {
      // use dueDate converted to UTC:
      chore.nextDueDate = new Date(dueDate).toUTCString()
      chore.notificationMetadata = notificationMetadata
    }

    createChoreMutation
      .mutateAsync(chore)
      .then(resp => {
        resp.json().then(data => {
          if (resp.status !== 200) {
            console.error('Error creating chore:', data)
            return
          } else {
            onChoreUpdate({
              ...chore,
              id: data.res,
              nextDueDate: chore.dueDate,
            })

            handleCloseModal(false)
          }
          handleCloseModal()
          setTaskText('')
        })
      })
      .catch(error => {
        if (error?.queued) {
          handleCloseModal(true)
        }
      })
  }
  if (userLabelsLoading || isCircleMembersLoading) {
    return <></>
  }

  return (
    <FadeModal
      open={isModalOpen}
      onClose={handleCloseModal}
      size='lg'
      fullWidth={true}
    >
      <Typography level='h4'>Create new task</Typography>
      <Chip startDecorator='🚧' variant='soft' color='warning' size='sm'>
        Experimental Feature
      </Chip>
      <Box>
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'row',
            alignItems: 'center',
          }}
        >
          <Typography level='body-sm'>Task in a sentence:</Typography>
          <LearnMoreButton
            content={
              <>
                <Typography level='body-sm' sx={{ mb: 1 }}>
                  This feature lets you create a task simply by typing a
                  sentence. It attempt parses the sentence to identify the
                  task&apos;s due date, priority, and frequency.
                </Typography>

                <Typography level='body-sm' sx={{ fontWeight: 'bold', mt: 2 }}>
                  Examples:
                </Typography>

                <Typography
                  level='body-sm'
                  component='ul'
                  sx={{ pl: 2, mt: 1, listStyle: 'disc' }}
                >
                  <li>
                    <strong>Priority:</strong>For highest priority any of the
                    following keyword <em>P1</em>, <em>Urgent</em>,{' '}
                    <em>Important</em>, or <em>ASAP</em>. For lower priorities,
                    use <em>P2</em>, <em>P3</em>, or <em>P4</em>.
                  </li>
                  <li>
                    <strong>Due date:</strong> Specify dates with phrases like{' '}
                    <em>tomorrow</em>, <em>next week</em>, <em>Monday</em>, or{' '}
                    <em>August 1st at 12pm</em>.
                  </li>
                  <li>
                    <strong>Frequency:</strong> Set recurring tasks with terms
                    like <em>daily</em>, <em>weekly</em>, <em>monthly</em>,{' '}
                    <em>yearly</em>, or patterns such as{' '}
                    <em>every Tuesday and Thursday</em>.
                  </li>
                </Typography>
              </>
            }
          />
        </Box>

        <SmartTaskTitleInput
          autoFocus
          value={taskText}
          placeholder='Type your full text here...'
          onChange={text => {
            setTaskText(text)
          }}
          customRenderer={renderedParts}
          onEnterPressed={handleEnterPressed}
          suggestions={{
            '#': {
              value: 'id',
              display: 'name',
              options: userLabels ? userLabels : [],
            },
            '!': {
              value: 'id',
              display: 'name',
              options: [
                { id: '1', name: 'P1' },
                { id: '2', name: 'P2' },
                { id: '3', name: 'P3' },
                { id: '4', name: 'P4' },
              ],
            },
            '@': {
              value: 'userId',
              display: 'displayName',
              options: circleMembers?.res || [],
            },
          }}
        />
      </Box>
      {/* <Box>
              <Typography level='body-sm'>Title:</Typography>
              <Input
                value={taskTitle}
                onChange={e => setTaskTitle(e.target.value)}
                sx={{ width: '100%', fontSize: '16px' }}
              />
            </Box> */}

      <Box>
        {!hasDescription && (
          <Button
            startDecorator={<Add />}
            variant='plain'
            size='sm'
            onClick={() => {
              setHasDescription(true)
              // Focus will be handled by the useEffect hook
            }}
            endDecorator={
              showKeyboardShortcuts && <KeyboardShortcutHint shortcut='E' />
            }
          >
            Description
          </Button>
        )}

        {!hasSubTasks && (
          <Button
            startDecorator={<Add />}
            variant='plain'
            size='sm'
            onClick={() => {
              setHasSubTasks(true)
            }}
            endDecorator={
              showKeyboardShortcuts && <KeyboardShortcutHint shortcut='J' />
            }
          >
            Subtasks
          </Button>
        )}
        {!dueDate && (
          <Button
            startDecorator={<Add />}
            variant='plain'
            size='sm'
            onClick={() => {
              setDueDate(moment().add(1, 'day').format('YYYY-MM-DDTHH:00:00'))
            }}
            endDecorator={
              showKeyboardShortcuts && <KeyboardShortcutHint shortcut='B' />
            }
          >
            Due Date
          </Button>
        )}
        {!hasNotifications && dueDate && (
          <Button
            startDecorator={<EditNotifications />}
            variant='plain'
            size='sm'
            onClick={() => {
              setHasNotifications(true)
              setFrequencyHumanReadable('Once')
              setFrequency(null)
            }}
          >
            Edit Notifications
          </Button>
        )}
      </Box>

      {hasDescription && (
        <Box>
          <Typography level='body-sm'>Description:</Typography>
          <div>
            <RichTextEditor
              ref={richTextEditorRef}
              onChange={setDescription}
              entityType={'chore_description'}
            />
          </div>
        </Box>
      )}
      {hasSubTasks && (
        <Box>
          <Typography level='body-sm'>Subtasks:</Typography>
          <SubTasks
            editMode={true}
            tasks={subTasks ? subTasks : []}
            setTasks={setSubTasks}
            shouldFocus={true}
          />
        </Box>
      )}

      <Box
        sx={{
          marginTop: 2,
          display: 'flex',
          flexDirection: 'row',
          gap: 2,
        }}
      >
        {priority > 0 && (
          <FormControl>
            <Typography level='body-sm'>Priority</Typography>
            <Select
              defaultValue={0}
              value={priority}
              onChange={(e, value) => setPriority(value)}
            >
              <Option value='0'>No Priority</Option>
              <Option value='1'>P1</Option>
              <Option value='2'>P2</Option>
              <Option value='3'>P3</Option>
              <Option value='4'>P4</Option>
            </Select>
          </FormControl>
        )}
        {dueDate && (
          <FormControl>
            <Typography level='body-sm'>Due Date</Typography>
            <Input
              type='datetime-local'
              value={dueDate}
              onChange={e => setDueDate(e.target.value)}
              sx={{ width: '100%', fontSize: '16px' }}
            />
          </FormControl>
        )}
      </Box>
      <Box
        sx={{
          marginTop: 2,
          display: 'flex',
          flexDirection: 'row',
          justifyContent: 'start',
          gap: 2,
        }}
      >
        {/* <FormControl>
              <Typography level='body-sm'>Assignees</Typography>
              <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                {assignees.length > 0 ? (
                  assignees.map((assignee, index) => (
                    <Chip
                      key={assignee.userId || index}
                      variant='soft'
                      size='lg'
                      color='primary'
                    >
                      {assignee.displayName || assignee.username}
                    </Chip>
                  ))
                ) : (
                  <Chip variant='soft' size='sm' color='neutral'>
                    {userProfile.displayName}
                  </Chip>
                )}
              </Box>
            </FormControl> */}
        {hasNotifications && dueDate && (
          <Box
            sx={{
              flexDirection: 'column',
              alignItems: 'center',
            }}
          >
            <Typography level='body-sm'>Notification Schedule</Typography>
            <Box sx={{ p: 0.5 }}>
              <NotificationTemplate
                onChange={metadata => {
                  if (
                    metadata.notifications !== notificationMetadata.templates
                  ) {
                    const newNotificaitonMetadata = {
                      ...notificationMetadata,
                      templates: metadata.notifications,
                    }
                    setNotificationMetadata(newNotificaitonMetadata)
                  }
                }}
                value={notificationMetadata}
                showTimeline={false}
              />
            </Box>
          </Box>
        )}
      </Box>
      <Box
        sx={{
          marginTop: 2,
          display: 'flex',
          flexDirection: 'row',
          justifyContent: 'end',
          gap: 1,
        }}
      >
        <Button variant='outlined' color='neutral' onClick={handleCloseModal}>
          Cancel
          {showKeyboardShortcuts && (
            <KeyboardShortcutHint
              shortcut='Esc'
              sx={{ ml: 1 }}
              withCtrl={false}
            />
          )}
        </Button>
        <Button variant='solid' color='primary' onClick={createChore}>
          Create
          {showKeyboardShortcuts && (
            <KeyboardShortcutHint shortcut='Enter' sx={{ ml: 1 }} />
          )}
        </Button>
      </Box>
    </FadeModal>
  )
}

export default TaskInput
