import { useColorScheme } from '@mui/joy'
import { useEffect, useRef, useState } from 'react'
import AutocompleteDropdown from '../TestView/AutocompleteDropdown'
import './SmartTaskTitleInput.css'
const renderHighlightedText = (text, cursorPosition) => {
  const parts = []
  let lastIndex = 0

  const regex = /(Tomorrow)|(Every\s+\d+\s+\w+s?)|(#\w+)|(P\d+)/gi
  let match

  while ((match = regex.exec(text)) !== null) {
    const matchedText = match[0]
    const matchIndex = match.index

    if (matchIndex > lastIndex) {
      parts.push(text.substring(lastIndex, matchIndex))
    }

    let className = ''
    if (matchedText.toLowerCase() === 'tomorrow') {
      className = 'highlight-date'
    } else if (matchedText.toLowerCase().startsWith('every')) {
      className = 'highlight-repeat'
    } else if (matchedText.startsWith('#')) {
      className = 'highlight-label'
    } else if (matchedText.startsWith('P')) {
      className = 'highlight-priority'
    }

    parts.push(
      <span key={matchIndex} className={className}>
        {matchedText}
      </span>,
    )

    lastIndex = regex.lastIndex
  }

  if (lastIndex < text.length) {
    parts.push(text.substring(lastIndex))
  }

  return parts
}

const SmartTaskTitleInput = ({
  value,
  placeholder,
  autoFocus,
  onChange,
  suggestions,
  onEnterPressed,
  customRenderer,
}) => {
  const { mode, setMode } = useColorScheme()
  const titleInputRef = useRef(null)
  const [cursorPosition, setCursorPosition] = useState(value?.length)
  const dropdownRef = useRef(null)
  const [lastWord, setLastWord] = useState('')
  const [suggestionTrigger, setSuggestionTrigger] = useState('P')
  const [showSuggestions, setShowSuggestions] = useState(false)
  const [selectedSuggestionIndex, setSelectedSuggestionIndex] = useState(0)

  useEffect(() => {
    setCursorPosition(prevPos => Math.min(prevPos, value.length))

    if (
      titleInputRef.current &&
      document.activeElement === titleInputRef.current
    ) {
      requestAnimationFrame(() => {
        titleInputRef.current.setSelectionRange(cursorPosition, cursorPosition)
      })
    }
  }, [value])
  useEffect(() => {
    // set focus on the input when the component is mounted:
    if (titleInputRef.current) {
      titleInputRef.current.focus()
      titleInputRef.current.setSelectionRange(cursorPosition, cursorPosition)
    }
  }, [])
  const handleSuggestionChange = text => {
    // if the last word start with '@' or '#' or 'P':
    const lastWord = text.split(' ').pop()
    if (
      lastWord.startsWith('@') ||
      lastWord.startsWith('#') ||
      lastWord.startsWith('!')
    ) {
      setSuggestionTrigger(lastWord[0])
      // last word without the first character:
      setLastWord(lastWord.slice(1))

      setShowSuggestions(true)
    } else {
      setShowSuggestions(false)
    }
  }

  const handleTextareaChange = e => {
    handleSuggestionChange(e.target.value)
    onChange(e.target.value)
    setCursorPosition(e.target.selectionStart)
  }

  const handleTextareaKeyDown = e => {
    if (showSuggestions) {
      const currentSuggestions = suggestions[suggestionTrigger].options.filter(
        option => {
          if (typeof option === 'string') {
            return option.toLowerCase().includes(lastWord.toLowerCase())
          }
          return option[suggestions[suggestionTrigger].display]
            .toLowerCase()
            .includes(lastWord.toLowerCase())
        },
      )

      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault()
        const newIndex =
          e.key === 'ArrowDown'
            ? (selectedSuggestionIndex + 1) % currentSuggestions.length
            : (selectedSuggestionIndex - 1 + currentSuggestions.length) %
              currentSuggestions.length
        setSelectedSuggestionIndex(newIndex)
      } else if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        const selectedSuggestion = currentSuggestions[selectedSuggestionIndex]
        const suggestionValue = suggestions[suggestionTrigger].display
          ? selectedSuggestion[suggestions[suggestionTrigger].display]
          : selectedSuggestion

        if (suggestionValue) {
          const newValue = `${value.slice(0, cursorPosition - lastWord.length)}${suggestionValue} ${value.slice(cursorPosition)}`
          onChange(newValue)
          titleInputRef.current.value = newValue

          setShowSuggestions(false)

          const newCursorPosition = cursorPosition + suggestionValue.length + 1
          titleInputRef.current.setSelectionRange(
            newCursorPosition,
            newCursorPosition,
          )
        }
      } else if (e.key === 'Escape') {
        e.preventDefault()
        setShowSuggestions(false)
      }
    } else {
      if (e.key === 'Enter') {
        e.preventDefault()
        if (onEnterPressed) {
          onEnterPressed(value)
        }
      }
    }

    const currentPos = e.target.selectionStart
    setCursorPosition(currentPos)
  }

  const handleTextareaClick = e => {
    const currentPos = e.target.selectionStart
    setCursorPosition(currentPos)
  }

  const handleDisplayClick = e => {
    const range = document.caretRangeFromPoint(e.clientX, e.clientY)
    if (range) {
      const offset = range.startOffset
      setCursorPosition(offset)

      if (titleInputRef.current) {
        titleInputRef.current.focus()
        titleInputRef.current.setSelectionRange(offset, offset)
      }
    }
  }

  return (
    <div>
      <div className='task-input overflow-auto rounded border'>
        <textarea
          ref={titleInputRef}
          autoFocus={autoFocus}
          rows={1}
          value={value}
          onChange={handleTextareaChange}
          onKeyDown={handleTextareaKeyDown}
          onClick={handleTextareaClick}
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            width: '100%',
            height: '100%',
            // opacity: 100,
            zIndex: 1,
            resize: 'none',
            overflow: 'hidden',
            padding: '0.5rem',
            boxSizing: 'border-box',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            fontFamily: 'inherit',
            fontSize: 'inherit',
            lineHeight: 'inherit',
            backgroundColor: 'transparent',
            color: mode === 'dark' ? '#cbd5e1' : '#1a202c',
            caretColor: mode === 'dark' ? '#fff' : '#000',
          }}
        />
        <div
          className='smart-task-display smart-task-common'
          ref={dropdownRef}
          style={{
            position: 'relative',
            zIndex: 1,
            minHeight: '100%',
            padding: '0.5rem',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            fontFamily: 'inherit',
            fontSize: 'inherit',
            lineHeight: 'inherit',
          }}
          onClick={handleDisplayClick}
        >
          {placeholder && !value && (
            <span className='pointer-events-none  text-gray-400'>
              {placeholder}
            </span>
          )}
          {customRenderer
            ? customRenderer
            : renderHighlightedText(value, cursorPosition)}
        </div>
      </div>
      {showSuggestions && (
        <AutocompleteDropdown
          currentValue={lastWord}
          suggestions={suggestions[suggestionTrigger]}
          selectedIndex={selectedSuggestionIndex}
          onMouseEnterSuggestion={index => {
            setSelectedSuggestionIndex(index)
          }}
          onSelectSuggestion={suggestion => {
            const suggestionValue = suggestions[suggestionTrigger].display
              ? suggestion[suggestions[suggestionTrigger].display]
              : suggestion
            const newValue = `${value.slice(0, cursorPosition)}${suggestionValue}${value.slice(cursorPosition)}`

            onChange(newValue)
            titleInputRef?.current?.focus()

            setCursorPosition(cursorPosition + suggestion.length)
            titleInputRef.current.value = newValue
            titleInputRef.current.setSelectionRange(
              cursorPosition + suggestionValue.length,
              cursorPosition + suggestionValue.length,
            )
            setShowSuggestions(false)
          }}
          parentRefer={dropdownRef}
        />
      )}
    </div>
  )
}

export default SmartTaskTitleInput
