// AutocompleteDropdown.jsx
import { Divider, Menu, MenuItem } from '@mui/joy'
import React, { useEffect } from 'react'

const AutocompleteDropdown = ({
  currentValue,
  suggestions,
  selectedIndex,
  onSelectSuggestion,
  onMouseEnterSuggestion, // Added for hover selection
  parentRefer, // Ref to the dropdown element
}) => {
  // Scroll selected item into view
  const dropdownMenuRef = React.useRef(null)
  useEffect(() => {
    const selectedElement = dropdownMenuRef.current?.querySelector('.selected')
    if (selectedElement) {
      selectedElement.scrollIntoView({
        block: 'nearest',
        inline: 'nearest',
      })
    }
  }, [selectedIndex, dropdownMenuRef])

  if (!suggestions || suggestions.options === 0) {
    return null // Don't render if no suggestions
  }

  return (
    <Menu
      open={Boolean(suggestions?.options?.length)}
      ref={dropdownMenuRef}
      anchorEl={parentRefer.current}
      onClose={() => {}}
      sx={{
        width: '300px',
        maxHeight: '160px',
        overflow: 'auto',
        position: 'relative',
        bottom: 0,
        left: 0,
        zIndex: 1300,
      }}
    >
      {suggestions?.options
        .filter(option => {
          if (typeof option === 'string') {
            return option.toLowerCase().includes(currentValue.toLowerCase())
          }
          return option[suggestions.display]
            .toLowerCase()
            .includes(currentValue.toLowerCase())
        })
        .map((option, index) => (
          <MenuItem
            key={suggestions.display ? option[suggestions.value] : option}
            selected={selectedIndex === index}
            onClick={() => onSelectSuggestion(option)}
            onMouseEnter={() => onMouseEnterSuggestion(index)} // Update selected index on hover
            className={selectedIndex === index ? 'selected' : ''} // Add class for selected item
            sx={{
              cursor: 'pointer',
              backgroundColor: selectedIndex === index ? 'gray.800' : 'inherit',
            }}
          >
            {suggestions.display ? option[suggestions.display] : option}
          </MenuItem>
        ))}
      <Divider orientation='horizontal' />
      {/* 
      <MenuItem
        selected={selectedIndex === suggestions?.options?.length}
        onClick={() => {
          onSelectSuggestion(currentValue)
        }}
        sx={{
          cursor: 'pointer',
          backgroundColor: 'gray.800',
        }}
      >
        <Add />
        Add new Label
      </MenuItem> */}
    </Menu>
  )
}

export default AutocompleteDropdown
