import * as allIcons from '@mui/icons-material' // Import all icons using * as
import { Grid, Input, SvgIcon } from '@mui/joy'
import React, { useEffect, useState } from 'react'

function MuiIconPicker({ onIconSelect }) {
  const [searchTerm, setSearchTerm] = useState('')
  const [filteredIcons, setFilteredIcons] = useState([])
  const outlined = Object.keys(allIcons).filter(name =>
    name.includes('Outlined'),
  )
  useEffect(() => {
    // Filter icons based on the search term
    setFilteredIcons(
      outlined.filter(name =>
        name
          .toLowerCase()
          .includes(searchTerm ? searchTerm.toLowerCase() : false),
      ),
    )
  }, [searchTerm])

  const handleIconClick = iconName => {
    onIconSelect(iconName) // Callback for selected icon
  }

  return (
    <div>
      {/* Autocomplete component for searching */}
      {JSON.stringify({ 1: searchTerm, filteredIcons: filteredIcons })}
      <Input
        onChange={(event, newValue) => {
          setSearchTerm(newValue)
        }}
      />
      {/* Grid to display icons */}
      <Grid container spacing={2}>
        {filteredIcons.map(iconName => {
          const IconComponent = allIcons[iconName]
          if (IconComponent) {
            // Add this check to prevent errors
            return (
              <Grid item key={iconName} xs={3} sm={2} md={1}>
                <SvgIcon
                  component={IconComponent}
                  onClick={() => handleIconClick(iconName)}
                  style={{ cursor: 'pointer' }}
                />
              </Grid>
            )
          }
          return null // Return null for non-icon exports
        })}
      </Grid>
    </div>
  )
}

export default MuiIconPicker
