const VALID_DAYS = {
  monday: 'Monday',
  mon: 'Monday',
  tuesday: 'Tuesday',
  tue: 'Tuesday',
  wednesday: 'Wednesday',
  wed: 'Wednesday',
  thursday: 'Thursday',
  thu: 'Thursday',
  friday: 'Friday',
  fri: 'Friday',
  saturday: 'Saturday',
  sat: 'Saturday',
  sunday: 'Sunday',
  sun: 'Sunday',
}

const VALID_MONTHS = {
  january: 'January',
  jan: 'January',
  february: 'February',
  feb: 'February',
  march: 'March',
  mar: 'March',
  april: 'April',
  apr: 'April',
  may: 'May',
  june: 'June',
  jun: 'June',
  july: 'July',
  jul: 'July',
  august: 'August',
  aug: 'August',
  september: 'September',
  sep: 'September',
  october: 'October',
  oct: 'October',
  november: 'November',
  nov: 'November',
  december: 'December',
  dec: 'December',
}

const ALL_MONTHS = Object.values(VALID_MONTHS).filter(
  (v, i, a) => a.indexOf(v) === i,
)

export const parsePriority = inputSentence => {
  let sentence = inputSentence.toLowerCase()
  const priorityMap = {
    1: ['!p1', 'priority 1', 'high priority', 'urgent', 'asap', 'important'],
    2: ['!p2', 'priority 2', 'medium priority'],
    3: ['!p3', 'priority 3', 'low priority'],
    4: ['!p4', 'priority 4'],
  }

  for (const [priority, terms] of Object.entries(priorityMap)) {
    if (terms.some(term => sentence.includes(term))) {
      return {
        result: priority,
        highlight: terms
          .map(term => {
            const index = sentence.indexOf(term)
            return {
              text: term,
              start: index,
              end: index + term.length,
            }
          })
          .filter(term => term.start !== -1),

        cleanedSentence: sentence.replace(
          new RegExp(`(${terms.join('|')})`, 'g'),
          '',
        ),
      }
    }
  }
  return {
    result: 0,
    highlight: [],
    cleanedSentence: inputSentence,
  }
}
export const parseLabels = (inputSentence, userLabels) => {
  let sentence = inputSentence.toLowerCase()
  const currentLabels = []
  // label will always be prefixed #:

  for (const label of userLabels) {
    if (sentence.includes(`#${label.name.toLowerCase()}`)) {
      currentLabels.push(label)
      sentence = sentence.replace(`#${label.name.toLowerCase()}`, '')
    }
  }
  if (currentLabels.length > 0) {
    return {
      result: currentLabels,
      highlight: currentLabels.map(label => {
        const index = inputSentence
          .toLowerCase()
          .indexOf(`#${label.name.toLowerCase()}`)
        return {
          text: `#${label.name}`,
          start: index,
          end: index + label.name.length + 1,
        }
      }),

      cleanedSentence: sentence.replace(
        new RegExp(`#(${userLabels.map(l => l.name).join('|')})`, 'g'),
        '',
      ),
    }
  }
  return { result: null, cleanedSentence: sentence }
}

export const parseRepeatV2 = inputSentence => {
  const sentence = inputSentence.toLowerCase()
  const result = {
    frequency: 1,
    frequencyType: null,
    frequencyMetadata: {
      days: [],
      months: [],
      unit: null,
      time: new Date().toISOString(),
    },
  }

  const patterns = [
    {
      frequencyType: 'day_of_the_month:every',
      regex: /(\d+)(?:th|st|nd|rd)? of every month/i,
      name: 'Every {day} of every month',
    },
    {
      frequencyType: 'daily',
      regex: /(every day|daily|everyday)/i,
      name: 'Every day',
    },
    {
      frequencyType: 'daily:time',
      regex: /every (morning|noon|afternoon|evening|night)/i,
      name: 'Every {time} daily',
    },
    {
      frequencyType: 'weekly',
      regex: /(every week|weekly)/i,
      name: 'Every week',
    },
    {
      frequencyType: 'monthly',
      regex: /(every month|monthly)/i,
      name: 'Every month',
    },
    {
      frequencyType: 'yearly',
      regex: /every year/i,
      name: 'Every year',
    },
    {
      frequencyType: 'monthly',
      regex: /every (?:other )?month/i,
      name: 'Bi Monthly',
      value: 2,
    },
    {
      frequencyType: 'interval:2week',
      regex: /(bi-?weekly|every other week)/i,
      value: 2,
      name: 'Bi Weekly',
    },
    {
      frequencyType: 'interval',
      regex: /every (\d+) (days?|weeks?|months?|years?)/i,
      name: 'Every {frequency} {unit}',
    },
    {
      frequencyType: 'interval:every_other',
      regex: /every other (days?|weeks?|months?|years?)/i,
      name: 'Every other {unit}',
    },
    {
      frequencyType: 'days_of_the_week',
      regex: /every ([\w, ]+(?:day)?(?:, [\w, ]+(?:day)?)*)/i,
      name: 'Every {days}',
    },
    {
      frequencyType: 'day_of_the_month',
      regex: /(\d+)(?:st|nd|rd|th)? of ([\w ]+(?:(?:,| and |\s)[\w ]+)*)/i,
      name: 'Every {day} days of {months}',
    },
  ]

  for (const pattern of patterns) {
    const match = sentence.match(pattern.regex)
    if (!match) continue

    result.frequencyType = pattern.frequencyType
    const unitMap = {
      daily: 'days',
      weekly: 'weeks',
      monthly: 'months',
      yearly: 'years',
    }

    switch (pattern.frequencyType) {
      case 'daily':
      case 'weekly':
      case 'monthly':
      case 'yearly':
        result.frequencyType = 'interval'
        result.frequency = pattern.value || 1
        result.frequencyMetadata.unit = unitMap[pattern.frequencyType]
        return {
          result,
          name: pattern.name,
          highlight: [
            {
              text: pattern.name,
              start: inputSentence.indexOf(match[0]),
              end: inputSentence.indexOf(match[0]) + match[0].length,
            },
          ],
          cleanedSentence: inputSentence.replace(match[0], '').trim(),
        }

      case 'interval':
        result.frequency = parseInt(match[1], 10)
        result.frequencyMetadata.unit = match[2]
        return {
          result,
          name: pattern.name
            .replace('{frequency}', result.frequency)
            .replace('{unit}', result.frequencyMetadata.unit),
          highlight: [
            {
              text: pattern.name,
              start: inputSentence.indexOf(match[0]),
              end: inputSentence.indexOf(match[0]) + match[0].length,
            },
          ],
          cleanedSentence: inputSentence.replace(match[0], '').trim(),
        }

      case 'days_of_the_week':
        result.frequencyMetadata.days = match[1]
          .toLowerCase()
          .split(/ and |,|\s/)
          .map(day => day.trim())
          .filter(day => VALID_DAYS[day])
          .map(day => VALID_DAYS[day])
        if (!result.frequencyMetadata.days.length)
          return { result: null, name: null, cleanedSentence: inputSentence }
        return {
          result,
          name: pattern.name.replace(
            '{days}',
            result.frequencyMetadata.days.join(', '),
          ),
          highlight: [
            {
              text: pattern.name,
              start: inputSentence.indexOf(match[0]),
              end: inputSentence.indexOf(match[0]) + match[0].length,
            },
          ],
          cleanedSentence: inputSentence.replace(match[0], '').trim(),
        }

      case 'day_of_the_month':
        result.frequency = parseInt(match[1], 10)
        result.frequencyMetadata.months = match[2]
          .toLowerCase()
          .split(/ and |,|\s/)
          .map(month => month.trim())
          .filter(month => VALID_MONTHS[month])
          .map(month => VALID_MONTHS[month])
        result.frequencyMetadata.unit = 'days'
        return {
          result,
          name: pattern.name
            .replace('{day}', result.frequency)
            .replace('{months}', result.frequencyMetadata.months.join(', ')),
          highlight: [
            {
              text: pattern.name,
              start: inputSentence.indexOf(match[0]),
              end: inputSentence.indexOf(match[0]) + match[0].length,
            },
          ],
          cleanedSentence: inputSentence.replace(match[0], '').trim(),
        }
      case 'interval:2week':
        result.frequency = 2
        result.frequencyMetadata.unit = 'weeks'
        result.frequencyType = 'interval'
        return {
          result,
          name: pattern.name,
          highlight: [
            {
              text: pattern.name,
              start: inputSentence.indexOf(match[0]),
              end: inputSentence.indexOf(match[0]) + match[0].length,
            },
          ],
          cleanedSentence: inputSentence.replace(match[0], '').trim(),
        }
      case 'daily:time':
        result.frequency = 1
        result.frequencyMetadata.unit = 'days'
        result.frequencyType = 'daily'
        return {
          result,
          name: pattern.name.replace('{time}', match[1]),
          // replace every x with ''
          highlight: [
            {
              text: pattern.name,
              start: inputSentence.indexOf(match[0]),
              end: inputSentence.indexOf(match[0]) + match[0].length,
            },
          ],
          cleanedSentence: inputSentence.replace(match[0], '').trim(),
        }

      case 'day_of_the_month:every':
        result.frequency = parseInt(match[1], 10)
        result.frequencyMetadata.months = ALL_MONTHS
        result.frequencyMetadata.unit = 'days'
        return {
          result,
          name: pattern.name
            .replace('{day}', result.frequency)
            .replace('{months}', result.frequencyMetadata.months.join(', ')),
          highlight: [
            {
              text: pattern.name,
              start: inputSentence.indexOf(match[0]),
              end: inputSentence.indexOf(match[0]) + match[0].length,
            },
          ],
          cleanedSentence: inputSentence.replace(match[0], '').trim(),
        }
      case 'interval:every_other':
        result.frequency = 2
        result.frequencyMetadata.unit = match[1]
        result.frequencyType = 'interval'
        return {
          result,
          name: pattern.name.replace('{unit}', result.frequencyMetadata.unit),
          highlight: [
            {
              text: pattern.name,
              start: inputSentence.indexOf(match[0]),
              end: inputSentence.indexOf(match[0]) + match[0].length,
            },
          ],
          cleanedSentence: inputSentence.replace(match[0], '').trim(),
        }
    }
  }
  return {
    result: null,
    name: null,
    highlight: [],
    cleanedSentence: inputSentence,
  }
}

export const parseAssignees = (inputSentence, users) => {
  const sentence = inputSentence.toLowerCase()
  const result = []
  const highlight = []

  for (const user of users) {
    if (sentence.includes(`@${user.username.toLowerCase()}`)) {
      result.push(user)
      const index = inputSentence
        .toLowerCase()
        .indexOf(`@${user.username.toLowerCase()}`)
      highlight.push({
        text: `@${user.username}`,
        start: index,
        end: index + user.username.length + 1,
      })
    }
  }

  if (result.length > 0) {
    return {
      result,
      highlight,
      cleanedSentence: sentence.replace(
        new RegExp(`@(${users.map(u => u.username).join('|')})`, 'g'),
        '',
      ),
    }
  }
  return { result: null, cleanedSentence: sentence }
}

export const parseDueDate = (inputSentence, chrono) => {
  // Parse the due date using chrono
  const parsedDueDate = chrono.parse(inputSentence, new Date(), {
    forwardDate: true,
  })

  if (!parsedDueDate[0] || parsedDueDate[0].index === -1) {
    return {
      result: null,
      highlight: [],
      cleanedSentence: inputSentence,
    }
  }

  const dueDateMatch = parsedDueDate[0]
  const dueDateText = dueDateMatch.text
  const dueDateStartIndex = dueDateMatch.index
  const dueDateEndIndex = dueDateStartIndex + dueDateText.length

  // Define words that might precede the due date and should be removed
  const precedingWords = [
    'starting',
    'from',
    'beginning',
    'begin',
    'commence',
    'commencing',
  ]

  // Look for preceding words before the due date
  let cleanStartIndex = dueDateStartIndex
  let highlightStartIndex = dueDateStartIndex
  let precedingWord = ''

  // Extract text before the due date to check for preceding words
  const textBeforeDueDate = inputSentence.substring(0, dueDateStartIndex).trim()

  for (const word of precedingWords) {
    // Check if the text before due date ends with this preceding word
    const wordPattern = new RegExp(`\\b${word}\\s*$`, 'i')
    const match = textBeforeDueDate.match(wordPattern)

    if (match) {
      // Found a preceding word, include it in the text to be removed
      const matchStart = textBeforeDueDate.length - match[0].length
      cleanStartIndex = matchStart
      highlightStartIndex = matchStart
      precedingWord = match[0].trim()
      break
    }
  }

  // Create the highlight text
  const fullHighlightText = precedingWord
    ? `${precedingWord} ${dueDateText}`
    : dueDateText

  // Create cleaned sentence by removing the full match (preceding word + due date)
  const textToRemove = inputSentence.substring(cleanStartIndex, dueDateEndIndex)
  const cleanedSentence = inputSentence
    .replace(textToRemove, '')
    .replace(/\s+/g, ' ') // Replace multiple spaces with single space
    .trim()

  return {
    result: dueDateMatch.start.date(),
    highlight: [
      {
        text: fullHighlightText,
        start: highlightStartIndex,
        end: dueDateEndIndex,
      },
    ],
    cleanedSentence: cleanedSentence,
  }
}
