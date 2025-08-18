import moment from 'moment'
import { TASK_COLOR } from './Colors.jsx'

const priorityOrder = [1, 2, 3, 4, 0]
// ChoreGrouperOptions enum:
export const GROUPING_OPTIONS = {
  SMART: 'default',
  DUE_DATE: 'due_date',
  PRIORITY: 'priority',
  LABELS: 'labels',
}

export const ChoresGrouper = (groupBy, chores, filter) => {
  if (filter) {
    chores = chores.filter(chore => filter(chore))
  }

  // sort by priority then due date:
  chores.sort(ChoreSorter)
  var groups = []
  switch (groupBy) {
    case 'default':
      // same as due_date but hide empty groups: and if status is 1 or 2 have seperated catigory as Started:
      var groupRaw = {
        Started: [],
        Today: [],
        Tomorrow: [],
        'Next 7 Days': [],
        'Later This Month': [],
        Future: [],
        Overdue: [],
        Anytime: [],
      }
      chores.forEach(chore => {
        if (chore.status === 1 || chore.status === 2) {
          groupRaw['Started'].push(chore)
        } else if (chore.nextDueDate === null) {
          groupRaw['Anytime'].push(chore)
        } else if (new Date(chore.nextDueDate) < new Date()) {
          groupRaw['Overdue'].push(chore)
        } else if (
          new Date(chore.nextDueDate).toDateString() ===
          new Date().toDateString()
        ) {
          groupRaw['Today'].push(chore)
        } else if (
          new Date(chore.nextDueDate).toDateString() ===
          new Date(Date.now() + 24 * 60 * 60 * 1000).toDateString()
        ) {
          groupRaw['Tomorrow'].push(chore)
        } else if (
          new Date(chore.nextDueDate) <
            new Date(Date.now() + 8 * 24 * 60 * 60 * 1000) &&
          new Date(chore.nextDueDate) >
            new Date(Date.now() + 24 * 60 * 60 * 1000)
        ) {
          groupRaw['Next 7 Days'].push(chore)
        } else if (
          new Date(chore.nextDueDate).getMonth() === new Date().getMonth() &&
          new Date(chore.nextDueDate).getFullYear() === new Date().getFullYear()
        ) {
          groupRaw['Later This Month'].push(chore)
        } else {
          groupRaw['Future'].push(chore)
        }
      })
      groups = []
      if (groupRaw['Started'].length > 0) {
        groups.push({
          name: 'Started',
          content: groupRaw['Started'],
          color: TASK_COLOR.STARTED,
        })
      }
      if (groupRaw['Overdue'].length > 0) {
        groups.push({
          name: 'Overdue',
          content: groupRaw['Overdue'],
          color: TASK_COLOR.OVERDUE,
        })
      }
      if (groupRaw['Today'].length > 0) {
        groups.push({
          name: 'Today',
          content: groupRaw['Today'],
          color: TASK_COLOR.TODAY,
        })
      }
      if (groupRaw['Tomorrow'].length > 0) {
        groups.push({
          name: 'Tomorrow',
          content: groupRaw['Tomorrow'],
          color: TASK_COLOR.TOMORROW,
        })
      }
      if (groupRaw['Next 7 Days'].length > 0) {
        groups.push({
          name: 'Next 7 Days',
          content: groupRaw['Next 7 Days'],
          color: TASK_COLOR.NEXT_7_DAYS,
        })
      }
      if (groupRaw['Later This Month'].length > 0) {
        groups.push({
          name: 'Later This Month',
          content: groupRaw['Later This Month'],
          color: TASK_COLOR.LATER_THIS_MONTH,
        })
      }
      if (groupRaw['Future'].length > 0) {
        groups.push({
          name: 'Future',
          content: groupRaw['Future'],
          color: TASK_COLOR.FUTURE,
        })
      }
      if (groupRaw['Anytime'].length > 0) {
        groups.push({
          name: 'Anytime',
          content: groupRaw['Anytime'],
          color: TASK_COLOR.ANYTIME,
        })
      }
      break

    case 'due_date':
      var groupRaw = {
        Today: [],
        Tomorrow: [],
        'Next 7 Days': [],
        'Later This Month': [],
        Future: [],
        Overdue: [],
        Anytime: [],
      }
      chores.forEach(chore => {
        if (chore.nextDueDate === null) {
          groupRaw['Anytime'].push(chore)
        } else if (new Date(chore.nextDueDate) < new Date()) {
          groupRaw['Overdue'].push(chore)
        } else if (
          new Date(chore.nextDueDate).toDateString() ===
          new Date().toDateString()
        ) {
          groupRaw['Today'].push(chore)
        } else if (
          new Date(chore.nextDueDate).toDateString() ===
          new Date(Date.now() + 24 * 60 * 60 * 1000).toDateString()
        ) {
          groupRaw['Tomorrow'].push(chore)
        } else if (
          new Date(chore.nextDueDate) <
            new Date(Date.now() + 8 * 24 * 60 * 60 * 1000) &&
          new Date(chore.nextDueDate) >
            new Date(Date.now() + 24 * 60 * 60 * 1000)
        ) {
          groupRaw['Next 7 Days'].push(chore)
        } else if (
          new Date(chore.nextDueDate).getMonth() === new Date().getMonth() &&
          new Date(chore.nextDueDate).getFullYear() === new Date().getFullYear()
        ) {
          groupRaw['Later This Month'].push(chore)
        } else {
          groupRaw['Future'].push(chore)
        }
      })
      groups = [
        {
          name: 'Overdue',
          content: groupRaw['Overdue'],
          color: TASK_COLOR.OVERDUE,
        },
        { name: 'Today', content: groupRaw['Today'], color: TASK_COLOR.TODAY },
        {
          name: 'Tomorrow',
          content: groupRaw['Tomorrow'],
          color: TASK_COLOR.TOMORROW,
        },
        {
          name: 'Next 7 Days',
          content: groupRaw['Next 7 Days'],
          color: TASK_COLOR.NEXT_7_DAYS,
        },
        {
          name: 'Later This Month',
          content: groupRaw['Later This Month'],
          color: TASK_COLOR.LATER_THIS_MONTH,
        },
        {
          name: 'Future',
          content: groupRaw['Future'],
          color: TASK_COLOR.FUTURE,
        },
        {
          name: 'Anytime',
          content: groupRaw['Anytime'],
          color: TASK_COLOR.ANYTIME,
        },
      ]
      break
    case 'priority':
      groupRaw = {
        p1: [],
        p2: [],
        p3: [],
        p4: [],
        no_priority: [],
      }
      chores.forEach(chore => {
        switch (chore.priority) {
          case 1:
            groupRaw['p1'].push(chore)
            break
          case 2:
            groupRaw['p2'].push(chore)
            break
          case 3:
            groupRaw['p3'].push(chore)
            break
          case 4:
            groupRaw['p4'].push(chore)
            break
          default:
            groupRaw['no_priority'].push(chore)
            break
        }
      })
      groups = [
        {
          name: 'Priority 1',
          content: groupRaw['p1'],
          color: TASK_COLOR.PRIORITY_1,
        },
        {
          name: 'Priority 2',
          content: groupRaw['p2'],
          color: TASK_COLOR.PRIORITY_2,
        },
        {
          name: 'Priority 3',
          content: groupRaw['p3'],
          color: TASK_COLOR.PRIORITY_3,
        },
        {
          name: 'Priority 4',
          content: groupRaw['p4'],
          color: TASK_COLOR.PRIORITY_4,
        },
        {
          name: 'No Priority',
          content: groupRaw['no_priority'],
          color: TASK_COLOR.NO_PRIORITY,
        },
      ]
      break
    case 'labels':
      groupRaw = {}
      var labels = {}
      chores.forEach(chore => {
        chore.labelsV2.forEach(label => {
          labels[label.id] = label
          if (groupRaw[label.id] === undefined) {
            groupRaw[label.id] = []
          }
          groupRaw[label.id].push(chore)
        })
      })
      groups = Object.keys(groupRaw).map(key => {
        return {
          name: labels[key].name,
          content: groupRaw[key],
        }
      })
      groups.sort((a, b) => {
        a.name < b.name ? 1 : -1
      })
  }
  return groups
}
export const ChoreSorter = (a, b) => {
  const priorityA = priorityOrder.indexOf(a.priority)
  const priorityB = priorityOrder.indexOf(b.priority)
  if (priorityA !== priorityB) {
    return priorityA - priorityB
  }

  // Status sorting (0 > 1 > ... ascending order)
  if (a.status !== b.status) {
    return a.status - b.status
  }

  // Due date sorting (earlier dates first, null/undefined last)
  if (!a.nextDueDate && !b.nextDueDate) return 0
  if (!a.nextDueDate) return 1
  if (!b.nextDueDate) return -1

  return new Date(a.nextDueDate) - new Date(b.nextDueDate)
}
export const notInCompletionWindow = chore => {
  return (
    chore.completionWindow &&
    chore.completionWindow > -1 &&
    chore.nextDueDate &&
    moment().add(chore.completionWindow, 'hours') < moment(chore.nextDueDate)
  )
}
export const ChoreFilters = userProfile => ({
  anyone: () => true,
  assigned_to_me: chore => {
    return chore.assignedTo && chore.assignedTo === userProfile?.id
  },
  assigned_to_others: chore => {
    return chore.assignedTo && chore.assignedTo !== userProfile?.id
  },
})
