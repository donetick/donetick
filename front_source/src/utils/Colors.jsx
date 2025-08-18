const LABEL_COLORS = [
  { name: 'Default', value: '#FFFFFF' },
  { name: 'Salmon', value: '#ff7961' },
  { name: 'Teal', value: '#26a69a' },
  { name: 'Sky Blue', value: '#80d8ff' },
  { name: 'Grape', value: '#7e57c2' },
  { name: 'Sunshine', value: '#ffee58' },
  { name: 'Coral', value: '#ff7043' },
  { name: 'Lavender', value: '#ce93d8' },
  { name: 'Rose', value: '#f48fb1' },
  { name: 'Charcoal', value: '#616161' },
  { name: 'Sienna', value: '#8d6e63' },
  { name: 'Mint', value: '#a7ffeb' },
  { name: 'Amber', value: '#ffc107' },
  { name: 'Cobalt', value: '#3f51b5' },
  { name: 'Emerald', value: '#4caf50' },
  { name: 'Peach', value: '#ffab91' },
  { name: 'Ocean', value: '#0288d1' },
  { name: 'Mustard', value: '#ffca28' },
  { name: 'Ruby', value: '#d32f2f' },
  { name: 'Periwinkle', value: '#b39ddb' },
  { name: 'Turquoise', value: '#00bcd4' },
  { name: 'Lime', value: '#cddc39' },
  { name: 'Blush', value: '#f8bbd0' },
  { name: 'Ash', value: '#90a4ae' },
  { name: 'Sand', value: '#d7ccc8' },
]

export const COLORS = {
  white: '#FFFFFF',
  salmon: '#ff7961',
  teal: '#26a69a',
  skyBlue: '#80d8ff',
  grape: '#7e57c2',
  sunshine: '#ffee58',
  coral: '#ff7043',
  lavender: '#ce93d8',
  rose: '#f48fb1',
  charcoal: '#616161',
  sienna: '#8d6e63',
  mint: '#a7ffeb',
  amber: '#ffc107',
  cobalt: '#3f51b5',
  emerald: '#4caf50',
  peach: '#ffab91',
  ocean: '#0288d1',
  mustard: '#ffca28',
  ruby: '#d32f2f',
  periwinkle: '#b39ddb',
  turquoise: '#00bcd4',
  lime: '#cddc39',
  blush: '#f8bbd0',
  ash: '#90a4ae',
  sand: '#d7ccc8',
}

export const TASK_COLOR = {
  COMPLETED: '#4ec1a2',
  LATE: '#f6ad55',
  MISSED: '#F03A47',
  UPCOMING: '#AF5B5B',
  SKIPPED: '#E2C2FF',

  // For the calendar
  OVERDUE: '#F03A47',
  TODAY: '#ffc107',
  TOMORROW: '#4ec1a2',
  NEXT_7_DAYS: '#00bcd4',
  LATER_THIS_MONTH: '#b39ddb',
  FUTURE: '#d7ccc8',
  ANYTIME: '#90a4ae',

  // Legacy colors for backward compatibility
  IN_A_WEEK: '#4ec1a2',
  THIS_MONTH: '#00bcd4',
  LATER: '#d7ccc8',

  // FOR ASSIGNEE:
  ASSIGNED_TO_ME: '#4ec1a2',
  ASSIGNED_TO_OTHER: '#b39ddb',

  // FOR PRIORITY:
  // PRIORITY_1: '#F03A47',
  // PRIORITY_2: '#ffc107',
  // PRIORITY_3: '#00bcd4',
  // PRIORITY_4: '#7e57c2',
  // NO_PRIORITY: '#90a4ae',
  // FOR PRIORITY:
  // PRIORITY_1: '#F03A4780',
  // PRIORITY_2: '#ffc10780',
  // PRIORITY_3: '#00bcd480',
  // PRIORITY_4: '#7e57c280',
  PRIORITY_1: '#d32f2f',
  PRIORITY_2: '#ed6c02',
  PRIORITY_3: '#0288d1',
  // PRIORITY_4: '#388e3c',
  PRIORITY_4: '#90a4ae',
  // NO_PRIORITY: '#90a4ae80',
}
export default LABEL_COLORS

export const getTextColorFromBackgroundColor = bgColor => {
  if (!bgColor) return ''
  const hex = bgColor.replace('#', '')
  const r = parseInt(hex.substring(0, 2), 16)
  const g = parseInt(hex.substring(2, 4), 16)
  const b = parseInt(hex.substring(4, 6), 16)
  return r * 0.299 + g * 0.587 + b * 0.114 > 186 ? '#000000' : '#ffffff'
}
