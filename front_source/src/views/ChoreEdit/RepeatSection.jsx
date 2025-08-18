import {
  Box,
  Button,
  Card,
  Checkbox,
  Chip,
  FormControl,
  FormHelperText,
  Grid,
  Input,
  List,
  ListItem,
  Option,
  Radio,
  RadioGroup,
  Select,
  Typography,
} from '@mui/joy'
import moment from 'moment'
import { useEffect } from 'react'

import { useUserProfile } from '../../queries/UserQueries'
import { isPlusAccount } from '../../utils/Helpers'
import ThingTriggerSection from './ThingTriggerSection'

const FREQUENCY_TYPES_RADIOS = [
  'daily',
  'weekly',
  'monthly',
  'yearly',
  'adaptive',
  'custom',
]

const FREQUENCY_TYPE_MESSAGE = {
  adaptive:
    'This chore will be scheduled dynamically based on previous completion dates.',
  custom: 'This chore will be scheduled based on a custom frequency.',
}
const REPEAT_ON_TYPE = ['interval', 'days_of_the_week', 'day_of_the_month']
const MONTHS = [
  'january',
  'february',
  'march',
  'april',
  'may',
  'june',
  'july',
  'august',
  'september',
  'october',
  'november',
  'december',
]

const DAYS = [
  'monday',
  'tuesday',
  'wednesday',
  'thursday',
  'friday',
  'saturday',
  'sunday',
]
const RepeatOnSections = ({
  frequencyType,
  frequency,
  onFrequencyUpdate,
  frequencyMetadata,
  onFrequencyMetadataUpdate,
}) => {
  // if time on frequencyMetadata is not set, try to set it to the nextDueDate if available,
  // otherwise set it to 18:00 of the current day
  useEffect(() => {
    if (!frequencyMetadata?.time) {
      frequencyMetadata.time = moment(
        moment(new Date()).format('YYYY-MM-DD') + 'T' + '18:00',
      ).format()
    }
  }, [frequencyMetadata])

  const timePickerComponent = (
    <Grid item sm={12} sx={{ display: 'flex', alignItems: 'center' }}>
      <Typography level='h5'>At: </Typography>
      <Input
        type='time'
        defaultValue={moment(frequencyMetadata?.time).format('HH:mm')}
        onChange={e => {
          onFrequencyMetadataUpdate({
            ...frequencyMetadata,
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            time: moment(
              moment(new Date()).format('YYYY-MM-DD') + 'T' + e.target.value,
            ).format(),
          })
        }}
      />
    </Grid>
  )

  switch (frequencyType) {
    case 'interval':
      return (
        <>
          <Grid item sm={12} sx={{ display: 'flex', alignItems: 'center' }}>
            <Typography level='h5'>Every: </Typography>
            <Input
              slotProps={{
                input: {
                  min: 1,
                  max: 1000,
                },
              }}
              type='number'
              value={frequency}
              onChange={e => {
                onFrequencyUpdate(e.target.value)
              }}
            />
            <Select
              placeholder='Unit'
              value={frequencyMetadata?.unit || 'days'}
              sx={{ ml: 1 }}
            >
              {['hours', 'days', 'weeks', 'months', 'years'].map(item => (
                <Option
                  key={item}
                  value={item}
                  onClick={() => {
                    onFrequencyMetadataUpdate({
                      ...frequencyMetadata,
                      unit: item,
                    })
                  }}
                >
                  {item.charAt(0).toUpperCase() + item.slice(1)}
                </Option>
              ))}
            </Select>
          </Grid>
          {timePickerComponent}
        </>
      )
    case 'days_of_the_week':
      return (
        <>
          <Grid item sm={12} sx={{ display: 'flex', alignItems: 'center' }}>
            <Card>
              <List
                orientation='horizontal'
                wrap
                sx={{
                  '--List-gap': '8px',
                  '--ListItem-radius': '20px',
                }}
              >
                {DAYS.map(item => (
                  <ListItem key={item}>
                    <Checkbox
                      checked={frequencyMetadata?.days?.includes(item) || false}
                      onClick={() => {
                        const newDaysOfTheWeek = frequencyMetadata['days'] || []
                        if (newDaysOfTheWeek.includes(item)) {
                          newDaysOfTheWeek.splice(
                            newDaysOfTheWeek.indexOf(item),
                            1,
                          )
                        } else {
                          newDaysOfTheWeek.push(item)
                        }

                        onFrequencyMetadataUpdate({
                          ...frequencyMetadata,
                          days: newDaysOfTheWeek.sort(),
                        })
                      }}
                      overlay
                      disableIcon
                      variant='soft'
                      label={item.charAt(0).toUpperCase() + item.slice(1)}
                    />
                  </ListItem>
                ))}
              </List>
              <Button
                size='sm'
                variant='soft'
                color='neutral'
                checked={frequencyMetadata?.days?.length === 7}
                onClick={() => {
                  if (frequencyMetadata?.days?.length === 7) {
                    onFrequencyMetadataUpdate({
                      ...frequencyMetadata,
                      days: [],
                    })
                  } else {
                    onFrequencyMetadataUpdate({
                      ...frequencyMetadata,
                      days: DAYS.map(item => item),
                    })
                  }
                }}
                overlay
                disableIcon
              >
                {frequencyMetadata?.days?.length === 7
                  ? 'Unselect All'
                  : 'Select All'}
              </Button>
            </Card>
          </Grid>
          {timePickerComponent}
        </>
      )
    case 'day_of_the_month':
      return (
        <>
          <Grid
            item
            sm={12}
            sx={{
              display: 'flex',
              alignItems: 'center',
              flexDirection: 'column',
              justifyContent: 'space-between',
            }}
          >
            <Card>
              <List
                orientation='horizontal'
                wrap
                sx={{
                  '--List-gap': '8px',
                  '--ListItem-radius': '20px',
                }}
              >
                {MONTHS.map(item => (
                  <ListItem key={item}>
                    <Checkbox
                      checked={frequencyMetadata?.months?.includes(item)}
                      onClick={() => {
                        const newMonthsOfTheYear =
                          frequencyMetadata['months'] || []
                        if (newMonthsOfTheYear.includes(item)) {
                          newMonthsOfTheYear.splice(
                            newMonthsOfTheYear.indexOf(item),
                            1,
                          )
                        } else {
                          newMonthsOfTheYear.push(item)
                        }

                        onFrequencyMetadataUpdate({
                          ...frequencyMetadata,
                          months: newMonthsOfTheYear.sort(),
                        })
                        console.log('newMonthsOfTheYear', newMonthsOfTheYear)
                        // setDaysOfTheWeek(newDaysOfTheWeek)
                      }}
                      overlay
                      disableIcon
                      variant='soft'
                      label={item.charAt(0).toUpperCase() + item.slice(1)}
                    />
                  </ListItem>
                ))}
              </List>
              <Button
                size='sm'
                variant='soft'
                color='neutral'
                checked={frequencyMetadata?.months?.length === 12}
                onClick={() => {
                  if (frequencyMetadata?.months?.length === 12) {
                    onFrequencyMetadataUpdate({
                      ...frequencyMetadata,
                      months: [],
                    })
                  } else {
                    onFrequencyMetadataUpdate({
                      ...frequencyMetadata,
                      months: MONTHS.map(item => item),
                    })
                  }
                }}
                overlay
                disableIcon
              >
                {frequencyMetadata?.months?.length === 12
                  ? 'Unselect All'
                  : 'Select All'}
              </Button>
            </Card>
          </Grid>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              mb: 1.5,
            }}
          >
            <Typography>on the </Typography>
            <Input
              sx={{ width: '80px' }}
              type='number'
              value={frequency}
              onChange={e => {
                if (e.target.value < 1) {
                  e.target.value = 1
                } else if (e.target.value > 31) {
                  e.target.value = 31
                }
                // setDayOftheMonth(e.target.value)

                onFrequencyUpdate(e.target.value)
              }}
            />
            <Typography>of the above month/s</Typography>
          </Box>
          {timePickerComponent}
        </>
      )

    default:
      return <></>
  }
}

const RepeatSection = ({
  frequencyType,
  frequency,
  onFrequencyUpdate,
  onFrequencyTypeUpdate,
  frequencyMetadata,
  onFrequencyMetadataUpdate,
  frequencyError,
  allUserThings,
  onTriggerUpdate,
  OnTriggerValidate,
  isAttemptToSave,
  selectedThing,
}) => {
  const { data: userProfile } = useUserProfile()

  return (
    <Box mt={2}>
      <Typography level='h4'>Repeat :</Typography>
      <FormControl sx={{ mt: 1 }}>
        <Checkbox
          onChange={e => {
            onFrequencyTypeUpdate(e.target.checked ? 'daily' : 'once')
            if (e.target.checked) {
              onTriggerUpdate(null)
            }
          }}
          defaultChecked={!['once', 'trigger'].includes(frequencyType)}
          checked={!['once', 'trigger'].includes(frequencyType)}
          value={!['once', 'trigger'].includes(frequencyType)}
          overlay
          label='Repeat this task'
        />
        <FormHelperText>
          Is this something needed to be done regularly?
        </FormHelperText>
      </FormControl>
      {!['once', 'trigger'].includes(frequencyType) && (
        <>
          <Card sx={{ mt: 1 }}>
            <Typography level='h5'>How often should it be repeated?</Typography>

            <List
              orientation='horizontal'
              wrap
              sx={{
                '--List-gap': '8px',
                '--ListItem-radius': '20px',
              }}
            >
              {FREQUENCY_TYPES_RADIOS.map(item => (
                <ListItem key={item}>
                  <Checkbox
                    // disabled={index === 0}
                    checked={
                      item === frequencyType ||
                      (item === 'custom' &&
                        REPEAT_ON_TYPE.includes(frequencyType))
                    }
                    // defaultChecked={item === frequencyType}
                    onClick={() => {
                      if (item === 'custom') {
                        onFrequencyTypeUpdate(REPEAT_ON_TYPE[0])
                        onFrequencyUpdate(1)
                        onFrequencyMetadataUpdate({
                          unit: 'days',
                          time: frequencyMetadata?.time
                            ? frequencyMetadata?.time
                            : moment(
                                moment(new Date()).format('YYYY-MM-DD') +
                                  'T' +
                                  '18:00',
                              ).format(),
                          timezone:
                            Intl.DateTimeFormat().resolvedOptions().timeZone,
                        })

                        return
                      }
                      onFrequencyTypeUpdate(item)
                    }}
                    overlay
                    disableIcon
                    variant='soft'
                    label={
                      item.charAt(0).toUpperCase() +
                      item.slice(1).replace('_', ' ')
                    }
                  />
                </ListItem>
              ))}
            </List>
            <Typography>{FREQUENCY_TYPE_MESSAGE[frequencyType]}</Typography>
            {frequencyType === 'custom' ||
              (REPEAT_ON_TYPE.includes(frequencyType) && (
                <>
                  <Grid container spacing={1} mt={2}>
                    <Grid item>
                      <Typography>Repeat on:</Typography>
                      <Box
                        sx={{ display: 'flex', alignItems: 'center', gap: 2 }}
                      >
                        <RadioGroup
                          orientation='horizontal'
                          aria-labelledby='segmented-controls-example'
                          name='justify'
                          // value={justify}
                          // onChange={event => setJustify(event.target.value)}
                          sx={{
                            minHeight: 48,
                            padding: '4px',
                            borderRadius: '12px',
                            bgcolor: 'neutral.softBg',
                            '--RadioGroup-gap': '4px',
                            '--Radio-actionRadius': '8px',
                            mb: 1,
                          }}
                        >
                          {REPEAT_ON_TYPE.map(item => (
                            <Radio
                              key={item}
                              color='neutral'
                              checked={item === frequencyType}
                              onClick={() => {
                                if (
                                  item === 'day_of_the_month' ||
                                  item === 'interval'
                                ) {
                                  onFrequencyUpdate(1)
                                }
                                onFrequencyTypeUpdate(item)
                                if (item === 'days_of_the_week') {
                                  onFrequencyMetadataUpdate({
                                    ...frequencyMetadata,
                                    days: [],
                                  })
                                } else if (item === 'day_of_the_month') {
                                  onFrequencyMetadataUpdate({
                                    ...frequencyMetadata,
                                    months: [],
                                  })
                                } else if (item === 'interval') {
                                  onFrequencyMetadataUpdate({
                                    ...frequencyMetadata,
                                    unit: 'days',
                                  })
                                }
                                // setRepeatOn(item)
                              }}
                              value={item}
                              disableIcon
                              label={item
                                .split('_')
                                .map((i, idx) => {
                                  // first or last word
                                  if (
                                    idx === 0 ||
                                    idx === item.split('_').length - 1
                                  ) {
                                    return (
                                      i.charAt(0).toUpperCase() + i.slice(1)
                                    )
                                  }
                                  return i
                                })
                                .join(' ')}
                              variant='plain'
                              sx={{
                                px: 2,
                                alignItems: 'center',
                              }}
                              slotProps={{
                                action: ({ checked }) => ({
                                  sx: {
                                    ...(checked && {
                                      bgcolor: 'background.surface',
                                      boxShadow: 'sm',
                                      '&:hover': {
                                        bgcolor: 'background.surface',
                                      },
                                    }),
                                  },
                                }),
                              }}
                            />
                          ))}
                        </RadioGroup>
                      </Box>
                    </Grid>

                    <RepeatOnSections
                      frequency={frequency}
                      onFrequencyUpdate={onFrequencyUpdate}
                      frequencyType={frequencyType}
                      onFrequencyTypeUpdate={onFrequencyTypeUpdate}
                      frequencyMetadata={frequencyMetadata || {}}
                      onFrequencyMetadataUpdate={onFrequencyMetadataUpdate}
                      things={allUserThings}
                    />
                  </Grid>
                </>
              ))}
            <FormControl error={Boolean(frequencyError)}>
              <FormHelperText error>{frequencyError}</FormHelperText>
            </FormControl>
          </Card>
        </>
      )}
      <FormControl sx={{ mt: 1 }}>
        <Checkbox
          onChange={e => {
            onFrequencyTypeUpdate(e.target.checked ? 'trigger' : 'once')
            //  if unchecked, set selectedThing to null:
            if (!e.target.checked) {
              onTriggerUpdate(null)
            }
          }}
          defaultChecked={frequencyType === 'trigger'}
          checked={frequencyType === 'trigger'}
          value={frequencyType === 'trigger'}
          disabled={!isPlusAccount(userProfile)}
          overlay
          label='Trigger this task based on a thing state'
        />
        <FormHelperText
          sx={{
            opacity: !isPlusAccount(userProfile) ? 0.5 : 1,
          }}
        >
          Is this something that should be done when a thing state changes?{' '}
          {userProfile && !isPlusAccount(userProfile) && (
            <Chip variant='soft' color='warning'>
              Plus Feature
            </Chip>
          )}
        </FormHelperText>
        {!isPlusAccount(userProfile) && (
          <Typography level='body-sm' color='warning' sx={{ mt: 1 }}>
            Thing-based triggers are not available in the Basic plan. Upgrade to
            Plus to automatically trigger tasks when device states change.
          </Typography>
        )}
      </FormControl>
      {frequencyType === 'trigger' && (
        <ThingTriggerSection
          things={allUserThings}
          onTriggerUpdate={onTriggerUpdate}
          onValidate={OnTriggerValidate}
          isAttemptToSave={isAttemptToSave}
          selected={selectedThing}
        />
      )}
    </Box>
  )
}

export default RepeatSection
