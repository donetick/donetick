import {
  HorizontalRule,
  KeyboardControlKey,
  KeyboardDoubleArrowUp,
  PriorityHigh,
} from '@mui/icons-material'

const Priorities = [
  {
    name: 'P1',
    value: 1,
    icon: <PriorityHigh />,
    color: 'danger',
  },
  {
    name: 'P2',
    value: 2,
    icon: <KeyboardDoubleArrowUp />,
    color: 'warning',
  },
  {
    name: 'P3 ',
    value: 3,
    icon: <KeyboardControlKey />,
    color: '',
  },
  {
    name: 'P4',
    value: 4,
    icon: <HorizontalRule />,
    color: '',
  },
]

export default Priorities
