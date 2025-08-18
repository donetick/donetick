import { Chip, List, ListItem, ListItemButton, Textarea } from '@mui/joy'
import React, { useEffect, useRef, useState } from 'react'

const AutocompleteInput = ({ options, ref, value, onChange, ...props }) => {
  const [filteredOptions, setFilteredOptions] = useState([])
  const [menuVisible, setMenuVisible] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(-1)
  const [triggerKey, setTriggerKey] = useState(null)
  // const inputRef = useRef(null)
  const menuRef = useRef(null)

  useEffect(() => {
    if (!triggerKey || !value.includes(triggerKey)) {
      setMenuVisible(false)
      return
    }

    const query = value.split(triggerKey).pop().toLowerCase()
    const matchedOptions = (options[triggerKey] || []).filter(option =>
      option.label.toLowerCase().startsWith(query),
    )

    setFilteredOptions(matchedOptions)
    setMenuVisible(matchedOptions.length > 0)
    setHighlightedIndex(0)
  }, [value, triggerKey, options])

  const handleKeyDown = e => {
    if (menuVisible) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setHighlightedIndex(prev => (prev + 1) % filteredOptions.length)
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setHighlightedIndex(
          prev => (prev - 1 + filteredOptions.length) % filteredOptions.length,
        )
      } else if (e.key === 'Tab' || e.key === 'Enter') {
        e.preventDefault()
        if (filteredOptions[highlightedIndex]) {
          selectOption(filteredOptions[highlightedIndex])
        }
      } else if (e.key === 'Escape') {
        setMenuVisible(false)
      }
    } else if (Object.keys(options).includes(e.key)) {
      setTriggerKey(e.key)
    }
  }

  const selectOption = option => {
    const parts = value.split(triggerKey)
    parts.pop()
    onChange(parts.join(triggerKey) + triggerKey + option.label + ' ')
    setMenuVisible(false)
    setTriggerKey(null)
  }

  const handleClickOutside = event => {
    if (
      menuRef.current &&
      !menuRef.current.contains(event.target) &&
      ref.current &&
      !ref.current.contains(event.target)
    ) {
      setMenuVisible(false)
    }
  }

  useEffect(() => {
    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [])

  return (
    <div style={{ position: 'relative' }}>
      <Textarea
        {...props}
        ref={ref}
        value={value}
        onChange={onChange}
        onKeyDown={handleKeyDown}
        placeholder='Type here...'
      />
      {menuVisible && (
        <List ref={menuRef} style={{ position: 'absolute', zIndex: 1000 }}>
          {filteredOptions.map((option, index) => (
            <ListItem key={option.value || option.label}>
              <ListItemButton
                selected={index === highlightedIndex}
                onClick={() => selectOption(option)}
              >
                {option.color && (
                  <Chip
                    style={{
                      backgroundColor: option.color,
                      marginRight: '0.5rem',
                      color: '#fff',
                    }}
                    size='sm'
                    variant='soft'
                  />
                )}
                {option.label}
              </ListItemButton>
            </ListItem>
          ))}
        </List>
      )}
    </div>
  )
}

export default AutocompleteInput
