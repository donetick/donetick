import { Add } from '@mui/icons-material'
import {
  Box,
  Button,
  Card,
  Checkbox,
  Chip,
  Container,
  Divider,
  FormControl,
  FormHelperText,
  Input,
  List,
  ListItem,
  MenuItem,
  Option,
  Radio,
  RadioGroup,
  Select,
  Sheet,
  Switch,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import NotificationTemplate from '../../components/NotificationTemplate.jsx'
import {
  useChore,
  useCreateChore,
  useUpdateChore,
} from '../../queries/ChoreQueries.jsx'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries.jsx'
import { useNotification } from '../../service/NotificationProvider'
import { getTextColorFromBackgroundColor } from '../../utils/Colors.jsx'
import {
  DeleteChore,
  GetAllCircleMembers,
  GetThings,
} from '../../utils/Fetcher'
import { isPlusAccount } from '../../utils/Helpers'
import Priorities from '../../utils/Priorities.jsx'
import LoadingComponent from '../components/Loading.jsx'
import RichTextEditor from '../components/RichTextEditor.jsx'
import SubTasks from '../components/SubTask.jsx'
import { useLabels } from '../Labels/LabelQueries'
import ConfirmationModal from '../Modals/Inputs/ConfirmationModal'
import LabelModal from '../Modals/Inputs/LabelModal'
import RepeatSection from './RepeatSection'

const ASSIGN_STRATEGIES = [
  'random',
  'least_assigned',
  'least_completed',
  'keep_last_assigned',
  'random_except_last_assigned',
  'round_robin',
]
const REPEAT_ON_TYPE = ['interval', 'days_of_the_week', 'day_of_the_month']

const NO_DUE_DATE_REQUIRED_TYPE = ['no_repeat', 'once']
const NO_DUE_DATE_ALLOWED_TYPE = ['trigger']
const ChoreEdit = () => {
  const { t } = useTranslation()
  const { data: userProfile, isLoading: isUserProfileLoading } =
    useUserProfile()

  const [chore, setChore] = useState([])
  const [choresHistory, setChoresHistory] = useState([])
  const [userHistory, setUserHistory] = useState({})
  const { choreId } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [confirmModelConfig, setConfirmModelConfig] = useState({})
  const [assignees, setAssignees] = useState([])
  const [performers, setPerformers] = useState([])
  const [assignStrategy, setAssignStrategy] = useState(ASSIGN_STRATEGIES[2])
  const [dueDate, setDueDate] = useState(null)
  const [assignedTo, setAssignedTo] = useState(-1)
  const [frequencyType, setFrequencyType] = useState('once')
  const [frequency, setFrequency] = useState(1)
  const [frequencyMetadata, setFrequencyMetadata] = useState({})
  const [labels, setLabels] = useState([])
  const [labelsV2, setLabelsV2] = useState([])
  const [priority, setPriority] = useState(0)
  const [points, setPoints] = useState(-1)
  const [subTasks, setSubTasks] = useState(null)
  const [completionWindow, setCompletionWindow] = useState(-1)
  const [allUserThings, setAllUserThings] = useState([])
  const [thingTrigger, setThingTrigger] = useState(null)
  const [isThingValid, setIsThingValid] = useState(false)

  const [notificationMetadata, setNotificationMetadata] = useState({})

  const [isRolling, setIsRolling] = useState(false)
  const [isNotificable, setIsNotificable] = useState(false)
  const [isActive, setIsActive] = useState(true)
  const [updatedBy, setUpdatedBy] = useState(0)
  const [createdBy, setCreatedBy] = useState(0)
  const [errors, setErrors] = useState({})
  const [attemptToSave, setAttemptToSave] = useState(false)
  const [addLabelModalOpen, setAddLabelModalOpen] = useState(false)
  const { data: userLabelsRaw, isLoading: isUserLabelsLoading } = useLabels()
  const updateChoreMutation = useUpdateChore()
  const createChoreMutation = useCreateChore()
  const {
    data: choreData,
    isLoading: isChoreLoading,
    refetch: refetchChore,
  } = useChore(choreId)
  const { data: membersData, isLoading: isMemberDataLoading } =
    useCircleMembers()
  const { showSuccess, showError } = useNotification()

  const [userLabels, setUserLabels] = useState([])

  useEffect(() => {
    if (userLabelsRaw) {
      setUserLabels(userLabelsRaw)
    }
  }, [userLabelsRaw])

  const Navigate = useNavigate()

  const HandleValidateChore = () => {
    const errors = {}

    if (name.trim() === '') {
      errors.name = t('choreEdit.nameIsRequired')
    }
    if (assignees.length === 0) {
      errors.assignees = t('choreEdit.assigneesIsRequired')
    }
    if (assignedTo < 0) {
      errors.assignedTo = t('choreEdit.assignedToIsRequired')
    }
    if (frequencyType === 'interval' && !frequency > 0) {
      errors.frequency = t('choreEdit.invalidFrequency', {
        unit: frequencyMetadata.unit,
      })
    }
    if (
      frequencyType === 'days_of_the_week' &&
      frequencyMetadata['days']?.length === 0
    ) {
      errors.frequency = t('choreEdit.atLeastOneDayIsRequired')
    }
    if (
      frequencyType === 'day_of_the_month' &&
      frequencyMetadata['months']?.length === 0
    ) {
      errors.frequency = t('choreEdit.atLeastOneMonthIsRequired')
    }
    if (
      dueDate === null &&
      !NO_DUE_DATE_REQUIRED_TYPE.includes(frequencyType) &&
      !NO_DUE_DATE_ALLOWED_TYPE.includes(frequencyType)
    ) {
      if (REPEAT_ON_TYPE.includes(frequencyType)) {
        errors.dueDate = t('choreEdit.startDateIsRequired')
      } else {
        errors.dueDate = t('choreEdit.dueDateIsRequired')
      }
    }
    if (frequencyType === 'trigger') {
      if (!isThingValid) {
        errors.thingTrigger = t('choreEdit.thingTriggerIsInvalid')
      }
    }

    setErrors(errors)
    if (Object.keys(errors).length > 0) {
      const errorList = Object.keys(errors).map(key => (
        <ListItem key={key}>{errors[key]}</ListItem>
      ))
      showError({
        title: t('choreEdit.resolveErrors'),
        message: <List>{errorList}</List>,
      })
      return false
    }

    return true
  }

  const handleDueDateChange = e => {
    setDueDate(e.target.value)
  }
  const HandleSaveChore = () => {
    setAttemptToSave(true)
    if (!HandleValidateChore()) {
      return
    }
    let newChoreId = choreId
    if (searchParams.get('clone') === 'true') {
      newChoreId = null
    }
    const chore = {
      id: Number(newChoreId),
      name: name,
      description: description,
      assignees: assignees,
      dueDate: dueDate ? new Date(dueDate).toISOString() : null,
      frequencyType: frequencyType,
      frequency: Number(frequency),
      frequencyMetadata: frequencyMetadata,
      assignedTo: assignedTo,
      assignStrategy: assignStrategy,
      isRolling: isRolling,
      isActive: isActive,
      notification: isNotificable,
      labels: labels.map(l => l.name),
      labelsV2: labelsV2,
      subTasks: subTasks,
      notificationMetadata: notificationMetadata,
      thingTrigger: thingTrigger,
      points: points < 0 ? null : points,
      completionWindow:
        completionWindow < 0 || dueDate === null ? null : completionWindow,
      priority: priority,
    }
    let SaveFunction = createChoreMutation.mutateAsync
    if (newChoreId > 0) {
      SaveFunction = updateChoreMutation.mutateAsync
    }

    SaveFunction(chore)
      .then(() => {
        showSuccess({
          title: t('choreEdit.choreSaved'),
          message: t('choreEdit.choreSavedSuccess'),
        })
        Navigate('/my/chores/')
      })
      .catch(error => {
        console.error('Failed to save chore:', error)
        showError({
          title: t('choreEdit.saveFailed'),
          message: t('choreEdit.saveFailedError'),
        })
      })
  }
  useEffect(() => {
    GetAllCircleMembers().then(data => {
      setPerformers(data.res)
    })
    GetThings().then(response => {
      response.json().then(data => {
        setAllUserThings(data.res)
      })
    })
  }, [])
  useEffect(() => {
    if (isChoreLoading === false && choreData && choreId) {
      const data = choreData
      const isCloneMode = searchParams.get('clone') === 'true'

      setChore(data.res)
      setName(data.res.name ? data.res.name : '')
      setDescription(data.res.description ? data.res.description : '')
      setAssignees(data.res.assignees ? data.res.assignees : [])
      setAssignedTo(data.res.assignedTo)
      setFrequencyType(data.res.frequencyType ? data.res.frequencyType : 'once')

      setFrequencyMetadata(data.res.frequencyMetadata)
      setFrequency(data.res.frequency)

      setNotificationMetadata(data.res.notificationMetadata)
      setPoints(data.res.points && data.res.points > -1 ? data.res.points : -1)
      setCompletionWindow(
        data.res.completionWindow && data.res.completionWindow > -1
          ? data.res.completionWindow
          : -1,
      )

      setLabelsV2(data.res.labelsV2)

      setPriority(data.res.priority)
      setAssignStrategy(
        data.res.assignStrategy
          ? data.res.assignStrategy
          : ASSIGN_STRATEGIES[2],
      )
      setIsRolling(data.res.isRolling)
      setIsActive(data.res.isActive)
      setSubTasks(data.res.subTasks ? data.res.subTasks : [])

      if (isCloneMode) {
        if (data.res.subTasks) {
          const clonedSubTasks = data.res.subTasks.map(subTask => ({
            ...subTask,
            id: -subTask.id,
            parentId: subTask.parentId ? -subTask.parentId : null,
            completed: false,
            completedAt: null,
          }))
          setSubTasks(clonedSubTasks)
        }
        if (data.res.name) {
          setName(`${t('choreEdit.copyPrefix')}${data.res.name}`)
        }
      }

      setIsNotificable(data.res.notification)
      setThingTrigger(data.res.thingChore)
      setDueDate(
        data.res.nextDueDate
          ? moment(data.res.nextDueDate).format('YYYY-MM-DDTHH:mm:00')
          : null,
      )
      setCreatedBy(data.res.createdBy)
      setUpdatedBy(data.res.updatedBy)
    }
  }, [choreData, isChoreLoading, searchParams, t])

  useEffect(() => {
    if (!NO_DUE_DATE_REQUIRED_TYPE.includes(frequencyType) && !dueDate) {
      setDueDate(moment(new Date()).format('YYYY-MM-DDTHH:mm:00'))
    }
    if (NO_DUE_DATE_ALLOWED_TYPE.includes(frequencyType)) {
      setDueDate(null)
    }
  }, [frequencyType])

  useEffect(() => {
    if (assignees.length === 1) {
      setAssignedTo(assignees[0].userId)
    }
  }, [assignees])

  useEffect(() => {
    if (performers.length > 0 && assignees.length === 0 && userProfile) {
      setAssignees([
        {
          userId: userProfile?.id,
        },
      ])
    }
  }, [performers, userProfile])

  useEffect(() => {
    if (attemptToSave) {
      HandleValidateChore()
    }
  }, [assignees, name, frequencyMetadata, attemptToSave, dueDate])

  const handleDelete = () => {
    setConfirmModelConfig({
      isOpen: true,
      title: t('choreEdit.deleteChore'),
      confirmText: t('choreEdit.delete'),
      cancelText: t('choreEdit.cancel'),
      message: t('choreEdit.confirmDelete'),
      onClose: isConfirmed => {
        if (isConfirmed === true) {
          DeleteChore(choreId).then(response => {
            if (response.status === 200) {
              Navigate('/my/chores')
            } else {
              alert(t('choreEdit.deleteFailed'))
            }
          })
        }
        setConfirmModelConfig({})
      },
    })
  }
  if (
    (isChoreLoading && choreId) ||
    isUserLabelsLoading ||
    isUserProfileLoading ||
    isMemberDataLoading
  ) {
    return <LoadingComponent />
  }
  return (
    <Container maxWidth='md'>
      <Box>
        <FormControl error={errors.name}>
          <Typography level='h4'>{t('choreEdit.name')}</Typography>
          <Typography level='h5'>{t('choreEdit.nameHint')}</Typography>
          <Input value={name} onChange={e => setName(e.target.value)} />
          <FormHelperText error>{errors.name}</FormHelperText>
        </FormControl>
      </Box>
      <Box mt={2}>
        <FormControl error={errors.description}>
          <Typography level='h4'>{t('choreEdit.additionalDetails')}</Typography>
          <Typography level='h5'>{t('choreEdit.additionalDetailsHint')}</Typography>
          <RichTextEditor
            value={description}
            onChange={setDescription}
            entityId={choreId}
            entityType={'chore_description'}
          />
          <FormHelperText error>{errors.name}</FormHelperText>
        </FormControl>
      </Box>
      <Box mt={2}>
        <Typography level='h4'>{t('choreEdit.assignees')}</Typography>
        <Typography level='h5'>{t('choreEdit.assigneesHint')}</Typography>
        <Card>
          <List
            orientation='horizontal'
            wrap
            sx={{
              '--List-gap': '8px',
              '--ListItem-radius': '20px',
            }}
          >
            {performers?.map((item, index) => (
              <ListItem key={item.id}>
                <Checkbox
                  checked={assignees.find(a => a.userId == item.userId) != null}
                  onClick={() => {
                    if (assignees.some(a => a.userId === item.userId)) {
                      const newAssignees = assignees.filter(
                        a => a.userId !== item.userId,
                      )
                      setAssignees(newAssignees)
                    } else {
                      setAssignees([...assignees, { userId: item.userId }])
                    }
                  }}
                  overlay
                  disableIcon
                  variant='soft'
                  label={item.displayName}
                />
              </ListItem>
            ))}
          </List>
        </Card>
        <FormControl error={Boolean(errors.assignee)}>
          <FormHelperText error>{Boolean(errors.assignee)}</FormHelperText>
        </FormControl>
      </Box>
      {assignees.length > 1 && (
        <>
          <Box mt={2}>
            <Typography level='h4'>{t('choreEdit.assigned')}</Typography>
            <Typography level='h5'>{t('choreEdit.assignedHint')}</Typography>
            <Select
              placeholder='Select an assignee'
              disabled={assignees.length === 0}
              value={assignedTo > -1 ? assignedTo : null}
            >
              {performers
                ?.filter(p => assignees.find(a => a.userId == p.userId))
                .map((item, index) => (
                  <Option
                    value={item.userId}
                    key={item.displayName}
                    onClick={() => {
                      setAssignedTo(item.userId)
                    }}
                  >
                    {item.displayName}
                  </Option>
                ))}
            </Select>
          </Box>
          <Box mt={2}>
            <Typography level='h4'>{t('choreEdit.pickingMode')}</Typography>
            <Typography level='h5'>{t('choreEdit.pickingModeHint')}</Typography>
            <Card>
              <List
                orientation='horizontal'
                wrap
                sx={{
                  '--List-gap': '8px',
                  '--ListItem-radius': '20px',
                }}
              >
                {ASSIGN_STRATEGIES.map((item, idx) => (
                  <ListItem key={item}>
                    <Checkbox
                      checked={assignStrategy === item}
                      onClick={() => setAssignStrategy(item)}
                      overlay
                      disableIcon
                      variant='soft'
                      label={item
                        .split('_')
                        .map(x => x.charAt(0).toUpperCase() + x.slice(1))
                        .join(' ')}
                    />
                  </ListItem>
                ))}
              </List>
            </Card>
          </Box>
        </>
      )}
      <RepeatSection
        frequency={frequency}
        onFrequencyUpdate={setFrequency}
        frequencyType={frequencyType}
        onFrequencyTypeUpdate={setFrequencyType}
        frequencyMetadata={frequencyMetadata}
        onFrequencyMetadataUpdate={setFrequencyMetadata}
        frequencyError={errors?.frequency}
        allUserThings={allUserThings}
        onTriggerUpdate={thingUpdate => {
          if (thingUpdate === null) {
            setThingTrigger(null)
            return
          }
          setThingTrigger({
            triggerState: thingUpdate.triggerState,
            condition: thingUpdate.condition,
            thingID: thingUpdate.thing.id,
          })
        }}
        OnTriggerValidate={setIsThingValid}
        isAttemptToSave={attemptToSave}
        selectedThing={thingTrigger}
      />

      <Box mt={2}>
        <Typography level='h4'>
          {REPEAT_ON_TYPE.includes(frequencyType)
            ? t('choreEdit.startDate')
            : t('choreEdit.dueDate')}{' '}
          :
        </Typography>
        {frequencyType === 'trigger' && !dueDate && (
          <Typography level='body-sm'>
            {t('choreEdit.dueDateHint')}
          </Typography>
        )}

        {NO_DUE_DATE_REQUIRED_TYPE.includes(frequencyType) && (
          <FormControl sx={{ mt: 1 }}>
            <Checkbox
              onChange={e => {
                if (e.target.checked) {
                  setDueDate(moment(new Date()).format('YYYY-MM-DDTHH:mm:00'))
                } else {
                  setDueDate(null)
                }
              }}
              defaultChecked={dueDate !== null}
              checked={dueDate !== null}
              overlay
              label={t('choreEdit.giveTaskDueDate')}
            />
            <FormHelperText>{t('choreEdit.dueDateHelper')}</FormHelperText>
          </FormControl>
        )}
        {dueDate && (
          <FormControl error={Boolean(errors.dueDate)}>
            <Typography level='h5'>
              {REPEAT_ON_TYPE.includes(frequencyType)
                ? t('choreEdit.whenDoesChoreStart')
                : t('choreEdit.whenIsNextDueDate')}
            </Typography>
            <Input
              type='datetime-local'
              value={dueDate}
              onChange={handleDueDateChange}
            />
            <FormHelperText>{errors.dueDate}</FormHelperText>
          </FormControl>
        )}
        {dueDate && (
          <>
            <FormControl orientation='horizontal'>
              <Switch
                checked={completionWindow != -1}
                onClick={event => {
                  event.preventDefault()
                  if (completionWindow != -1) {
                    setCompletionWindow(-1)
                  } else {
                    setCompletionWindow(1)
                  }
                }}
                color={completionWindow !== -1 ? 'success' : 'neutral'}
                variant={completionWindow !== -1 ? 'solid' : 'outlined'}
                sx={{
                  mr: 2,
                }}
              />
              <div>
                <Typography level='h5'>
                  {t('choreEdit.completionWindow')}
                </Typography>
                <FormHelperText sx={{ mt: 0 }}>
                  {t('choreEdit.completionWindowHint')}
                </FormHelperText>
              </div>
            </FormControl>
            {completionWindow != -1 && (
              <Card variant='outlined'>
                <Box
                  sx={{
                    mt: 0,
                    ml: 4,
                  }}
                >
                  <Typography level='body-sm'>{t('choreEdit.hours')}</Typography>
                  <Input
                    type='number'
                    value={completionWindow}
                    sx={{ maxWidth: 100 }}
                    slotProps={{
                      input: {
                        min: 0,
                        max: 24 * 7,
                      },
                    }}
                    placeholder={t('choreEdit.hours')}
                    onChange={e => {
                      setCompletionWindow(parseInt(e.target.value))
                    }}
                  />
                </Box>
              </Card>
            )}
          </>
        )}
      </Box>
      {!['once', 'no_repeat'].includes(frequencyType) && (
        <Box mt={2}>
          <Typography level='h4'>
            {t('choreEdit.schedulingPreferences')}
          </Typography>
          <Typography level='h5'>
            {t('choreEdit.schedulingPreferencesHint')}
          </Typography>

          <RadioGroup name='tiers' sx={{ gap: 1, '& > div': { p: 1 } }}>
            <FormControl>
              <Radio
                overlay
                checked={!isRolling}
                onClick={() => setIsRolling(false)}
                label={t('choreEdit.rescheduleFromDueDate')}
              />
              <FormHelperText>
                {t('choreEdit.rescheduleFromDueDateHint')}
              </FormHelperText>
            </FormControl>
            <FormControl>
              <Radio
                overlay
                checked={isRolling}
                onClick={() => setIsRolling(true)}
                label={t('choreEdit.rescheduleFromCompletionDate')}
              />
              <FormHelperText>
                {t('choreEdit.rescheduleFromCompletionDateHint')}
              </FormHelperText>
            </FormControl>
          </RadioGroup>
        </Box>
      )}
      <Box mt={2}>
        <Typography level='h4'>{t('choreEdit.notifications')}</Typography>
        <Typography level='h5'>
          {t('choreEdit.notificationsHint')}
          {!isPlusAccount(userProfile) && (
            <Chip variant='soft' color='warning'>
              {t('choreEdit.plusFeature')}
            </Chip>
          )}
        </Typography>
        {!isPlusAccount(userProfile) && (
          <Typography level='body-sm' color='warning' sx={{ mb: 1 }}>
            {t('choreEdit.notificationsUnavailable')}
          </Typography>
        )}

        <FormControl sx={{ mt: 1 }}>
          <Checkbox
            onChange={e => {
              setIsNotificable(e.target.checked)
              if (!e.target.checked) {
                setNotificationMetadata({})
              }
            }}
            defaultChecked={isNotificable}
            checked={isNotificable}
            disabled={!isPlusAccount(userProfile)}
            overlay
            label={t('choreEdit.notifyForThisTask')}
          />
          <FormHelperText
            sx={{
              opacity: !isPlusAccount(userProfile) ? 0.5 : 1,
            }}
          >
            {t('choreEdit.notifyForThisTaskHint')}
          </FormHelperText>
        </FormControl>
      </Box>
      {isNotificable && (
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
            '& > div': { p: 2, borderRadius: 'md', display: 'flex' },
          }}
        >
          <Card variant='outlined'>
            <Typography level='body-md'>
              {t('choreEdit.notificationSchedule')}
            </Typography>
            <Box sx={{ p: 0.5 }}>
              <NotificationTemplate
                onChange={metadata => {
                  const newTemplates = metadata.notifications
                  if (notificationMetadata.templates !== newTemplates) {
                    setNotificationMetadata({
                      ...notificationMetadata,
                      templates: newTemplates,
                    })
                  }
                }}
                value={notificationMetadata}
              />
            </Box>
            <Typography level='h5'>{t('choreEdit.chooseWhoToNotify')}</Typography>
            <FormControl>
              <Checkbox
                overlay
                disabled={true}
                checked={true}
                label={t('choreEdit.allAssignees')}
              />
              <FormHelperText>{t('choreEdit.notifyAllAssignees')}</FormHelperText>
            </FormControl>

            <FormControl>
              <Checkbox
                overlay
                onClick={() => {
                  if (notificationMetadata['circleGroup']) {
                    delete notificationMetadata['circleGroupID']
                  }

                  setNotificationMetadata({
                    ...notificationMetadata,
                    ['circleGroup']: !notificationMetadata['circleGroup'],
                  })
                }}
                checked={
                  notificationMetadata
                    ? notificationMetadata['circleGroup']
                    : false
                }
                label={t('choreEdit.specificGroup')}
              />
              <FormHelperText>{t('choreEdit.notifySpecificGroup')}</FormHelperText>
            </FormControl>

            {notificationMetadata['circleGroup'] && (
              <Box
                sx={{
                  mt: 0,
                  ml: 4,
                }}
              >
                <Typography level='body-sm'>
                  {t('choreEdit.telegramGroupId')}
                </Typography>

                <Input
                  type='number'
                  value={notificationMetadata['circleGroupID']}
                  placeholder={t('choreEdit.telegramGroupId')}
                  onChange={e => {
                    setNotificationMetadata({
                      ...notificationMetadata,
                      ['circleGroupID']: parseInt(e.target.value),
                    })
                  }}
                />
              </Box>
            )}
          </Card>
        </Box>
      )}
      <Box mt={2}>
        <Typography level='h4'>{t('choreEdit.labels')}</Typography>
        <Typography level='h5'>{t('choreEdit.labelsHint')}</Typography>
        <Select
          multiple
          onChange={(event, newValue) => {
            setLabelsV2(userLabels.filter(l => newValue.indexOf(l.name) > -1))
          }}
          value={labelsV2?.map(l => l.name)}
          renderValue={selected => (
            <Box sx={{ display: 'flex', gap: '0.25rem' }}>
              {labelsV2.map(selectedOption => {
                return (
                  <Chip
                    variant='soft'
                    color='primary'
                    key={selectedOption.id}
                    size='lg'
                    sx={{
                      background: selectedOption.color,
                      color: getTextColorFromBackgroundColor(
                        selectedOption.color,
                      ),
                    }}
                  >
                    {selectedOption.name}
                  </Chip>
                )
              })}
            </Box>
          )}
          sx={{ minWidth: '15rem' }}
          slotProps={{
            listbox: {
              sx: {
                width: '100%',
              },
            },
          }}
        >
          {userLabels &&
            userLabels
              .map(label => (
                <Option key={label.id + label.name} value={label.name}>
                  <div
                    style={{
                      width: '20 px',
                      height: '20 px',
                      borderRadius: '50%',
                      background: label.color,
                    }}
                  />
                  {label.name}
                </Option>
              ))}
          <MenuItem
            key={'addNewLabel'}
            value={' New Label'}
            onClick={() => {
              setAddLabelModalOpen(true)
            }}
          >
            <Add />
            {t('choreEdit.addNewLabel')}
          </MenuItem>
        </Select>
      </Box>
      <Box mt={2}>
        <Typography level='h4'>{t('choreEdit.priority')}</Typography>
        <Typography level='h5'>{t('choreEdit.priorityHint')}</Typography>
        <Select
          onChange={(event, newValue) => {
            setPriority(newValue)
          }}
          value={priority}
          sx={{ minWidth: '15rem' }}
          slotProps={{
            listbox: {
              sx: {
                width: '100%',
              },
            },
          }}
        >
          {Priorities.map(priority => (
            <Option key={priority.id + priority.name} value={priority.value}>
              <div
                style={{
                  width: '20 px',
                  height: '20 px',
                  borderRadius: '50%',
                  background: priority.color,
                }}
              />
              {priority.name}
            </Option>
          ))}
          <Option value={0}>{t('choreEdit.noPriority')}</Option>
        </Select>
      </Box>

      <Box mt={2}>
        <Typography level='h4' gutterBottom>
          {t('choreEdit.others')}
        </Typography>

        <FormControl sx={{ mt: 1 }}>
          <Checkbox
            onChange={e => {
              if (e.target.checked) {
                setSubTasks([])
              } else {
                setSubTasks(null)
              }
            }}
            overlay
            checked={subTasks != null}
            label={t('choreEdit.subTasks')}
          />
          <FormHelperText>{t('choreEdit.subTasksHint')}</FormHelperText>
        </FormControl>
        {subTasks != null && (
          <Card
            variant='outlined'
            sx={{
              p: 1,
            }}
          >
            <SubTasks
              editMode={true}
              tasks={subTasks}
              setTasks={setSubTasks}
              choreId={choreId}
            />
          </Card>
        )}
        <FormControl sx={{ mt: 1 }}>
          <Checkbox
            onChange={e => {
              if (e.target.checked) {
                setPoints(1)
              } else {
                setPoints(-1)
              }
            }}
            checked={points > -1}
            overlay
            label={t('choreEdit.assignPoints')}
          />
          <FormHelperText>{t('choreEdit.assignPointsHint')}</FormHelperText>
        </FormControl>

        {points != -1 && (
          <Card variant='outlined'>
            <Box
              sx={{
                mt: 0,
                ml: 4,
              }}
            >
              <Typography level='body-sm'>{t('choreEdit.points')}</Typography>

              <Input
                type='number'
                value={points}
                sx={{ maxWidth: 100 }}
                slotProps={{
                  input: {
                    min: 0,
                    max: 1000,
                  },
                }}
                placeholder={t('choreEdit.points')}
                onChange={e => {
                  setPoints(parseInt(e.target.value))
                }}
              />
            </Box>
          </Card>
        )}
      </Box>
      {choreId > 0 && (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, mt: 3 }}>
          <Sheet
            sx={{
              p: 2,
              borderRadius: 'md',
              boxShadow: 'sm',
            }}
          >
            <Typography level='body1'>
              {t('choreEdit.createdBy')}{' '}
              <Chip variant='solid'>
                {membersData.res.find(f => f.userId === createdBy)?.displayName}
              </Chip>{' '}
              {t('choreEdit.fromNow', { time: moment(chore.createdAt).fromNow() })}
            </Typography>
            {(chore.updatedAt && updatedBy > 0 && (
              <>
                <Divider sx={{ my: 1 }} />

                <Typography level='body1'>
                  {t('choreEdit.updatedBy')}{' '}
                  <Chip variant='solid'>
                    {
                      membersData.res.find(f => f.userId === updatedBy)
                        ?.displayName
                    }
                  </Chip>{' '}
                  {t('choreEdit.fromNow', { time: moment(chore.updatedAt).fromNow() })}
                </Typography>
              </>
            )) || <></>}
          </Sheet>
        </Box>
      )}

      <Divider sx={{ mb: 9 }} />
      <Sheet
        variant='outlined'
        sx={{
          position: 'fixed',
          bottom: 0,
          left: 0,
          right: 0,
          p: 2,
          display: 'flex',
          justifyContent: 'flex-end',
          gap: 2,
          'z-index': 1000,
          bgcolor: 'background.body',
          boxShadow: 'md',
        }}
      >
        {choreId > 0 && (
          <Button
            color='danger'
            variant='solid'
            onClick={() => {
              handleDelete()
            }}
          >
            {t('choreEdit.delete')}
          </Button>
        )}
        <Button
          color='neutral'
          variant='outlined'
          onClick={() => {
            window.history.back()
          }}
        >
          {t('choreEdit.cancel')}
        </Button>
        <Button color='primary' variant='solid' onClick={HandleSaveChore}>
          {choreId > 0 ? t('choreEdit.save') : t('choreEdit.create')}
        </Button>
      </Sheet>
      <ConfirmationModal config={confirmModelConfig} />
      {addLabelModalOpen && (
        <LabelModal
          isOpen={addLabelModalOpen}
          onSave={label => {
            const newLabels = [...labelsV2]
            newLabels.push(label)
            setUserLabels([...userLabels, label])
            setLabelsV2([...labelsV2, label])
            setAddLabelModalOpen(false)
          }}
          onClose={() => setAddLabelModalOpen(false)}
        />
      )}
    </Container>
  )
}

export default ChoreEdit
