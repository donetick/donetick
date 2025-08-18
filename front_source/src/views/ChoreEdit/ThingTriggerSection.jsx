import { Widgets } from '@mui/icons-material'
import {
  Autocomplete,
  Box,
  Button,
  Card,
  Chip,
  FormControl,
  Input,
  Option,
  Select,
  TextField,
  Typography,
} from '@mui/joy'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
const isValidTrigger = (thing, condition, triggerState) => {
  const newErrors = {}
  if (!thing || !triggerState) {
    newErrors.thing = 'Please select a thing and trigger state'
    return false
  }
  if (thing.type === 'boolean') {
    if (['true', 'false'].includes(triggerState)) {
      return true
    } else {
      newErrors.type = 'Boolean type does not require a condition'
      return false
    }
  }
  if (thing.type === 'number') {
    if (isNaN(triggerState)) {
      newErrors.triggerState = 'Trigger state must be a number'
      return false
    }
    if (['eq', 'neq', 'gt', 'gte', 'lt', 'lte'].includes(condition)) {
      return true
    }
  }
  if (thing.type === 'text') {
    if (typeof triggerState === 'string') {
      return true
    }
  }
  newErrors.triggerState = 'Trigger state must be a number'

  return false
}

const ThingTriggerSection = ({
  things,
  onTriggerUpdate,
  onValidate,
  selected,
  isAttepmtingToSave,
}) => {
  const [selectedThing, setSelectedThing] = useState(null)
  const [condition, setCondition] = useState(null)
  const [triggerState, setTriggerState] = useState(null)
  const navigate = useNavigate()

  useEffect(() => {
    if (selected) {
      setSelectedThing(things?.find(t => t.id === selected.thingId))
      setCondition(selected.condition)
      setTriggerState(selected.triggerState)
    }
  }, [things])

  useEffect(() => {
    if (selectedThing && triggerState) {
      onTriggerUpdate({
        thing: selectedThing,
        condition: condition,
        triggerState: triggerState,
      })
    }
    if (isValidTrigger(selectedThing, condition, triggerState)) {
      onValidate(true)
    } else {
      onValidate(false)
    }
  }, [selectedThing, condition, triggerState])

  return (
    <Card sx={{ mt: 1 }}>
      <Typography level='h5'>
        Trigger a task when a thing state changes to a desired state
      </Typography>
      {things?.length === 0 && (
        <Typography level='body-sm'>
          it's look like you don't have any things yet, create a thing to
          trigger a task when the state changes.
          <Button
            startDecorator={<Widgets />}
            size='sm'
            onClick={() => {
              navigate('/things')
            }}
          >
            Go to Things
          </Button>{' '}
          to create a thing
        </Typography>
      )}
      <FormControl error={isAttepmtingToSave && !selectedThing}>
        <Autocomplete
          options={things}
          value={selectedThing}
          onChange={(e, newValue) => setSelectedThing(newValue)}
          getOptionLabel={option => option.name}
          renderOption={(props, option) => (
            <Box {...props}>
              <Box
                sx={{
                  display: 'flex',
                  flexDirection: 'column',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  p: 1,
                }}
              >
                <Box sx={{ alignSelf: 'flex-start' }}>
                  <Typography level='body-lg' textColor='primary'>
                    {option.name}
                  </Typography>
                </Box>
                <Box>
                  <Typography level='body2' textColor='text.secondary'>
                    <Chip>type: {option.type}</Chip>{' '}
                    <Chip>state: {option.state}</Chip>
                  </Typography>
                </Box>
              </Box>
            </Box>
          )}
          renderInput={params => (
            <TextField {...params} label='Select a thing' />
          )}
        />
      </FormControl>
      <Typography level='body-sm'>
        Create a condition to trigger a task when the thing state changes to
        desired state
      </Typography>
      {selectedThing?.type == 'boolean' && (
        <Box>
          <Typography level='body-sm'>
            When the state of {selectedThing.name} changes as specified below,
            the task will become due.
          </Typography>
          <Select
            value={triggerState}
            onChange={e => {
              if (e?.target.value === 'true' || e?.target.value === 'false')
                setTriggerState(e.target.value)
              else setTriggerState('false')
            }}
          >
            {['true', 'false'].map(state => (
              <Option
                key={state}
                value={state}
                onClick={() => setTriggerState(state)}
              >
                {state.charAt(0).toUpperCase() + state.slice(1)}
              </Option>
            ))}
          </Select>
        </Box>
      )}
      {selectedThing?.type == 'number' && (
        <Box>
          <Typography level='body-sm'>
            When the state of {selectedThing.name} changes as specified below,
            the task will become due.
          </Typography>

          <Box sx={{ display: 'flex', gap: 1, direction: 'row' }}>
            <Typography level='body-sm'>State is</Typography>
            <Select value={condition} sx={{ width: '50%' }}>
              {[
                { name: 'Equal', value: 'eq' },
                { name: 'Not equal', value: 'neq' },
                { name: 'Greater than', value: 'gt' },
                { name: 'Greater than or equal', value: 'gte' },
                { name: 'Less than', value: 'lt' },
                { name: 'Less than or equal', value: 'lte' },
              ].map(condition => (
                <Option
                  key={condition.value}
                  value={condition.value}
                  onClick={() => setCondition(condition.value)}
                >
                  {condition.name}
                </Option>
              ))}
            </Select>
            <Input
              type='number'
              value={triggerState}
              onChange={e => setTriggerState(e.target.value)}
              sx={{ width: '50%' }}
            />
          </Box>
        </Box>
      )}
      {selectedThing?.type == 'text' && (
        <Box>
          <Typography level='body-sm'>
            When the state of {selectedThing.name} changes as specified below,
            the task will become due.
          </Typography>

          <Input
            value={triggerState}
            onChange={e => setTriggerState(e.target.value)}
            label='Enter the text to trigger the task'
          />
        </Box>
      )}
    </Card>
  )
}

export default ThingTriggerSection
