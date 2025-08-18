import { Button, Chip, Menu, MenuItem, Typography } from '@mui/joy'
import IconButton from '@mui/joy/IconButton'
import React, { useEffect, useRef, useState } from 'react'
import { getTextColorFromBackgroundColor } from '../../utils/Colors.jsx'

const IconButtonWithMenu = ({
  label,
  k,
  icon,
  options,
  onItemSelect,
  selectedItem,
  setSelectedItem,
  isActive,
  useChips,
  title,
}) => {
  const [anchorEl, setAnchorEl] = useState(null)
  const menuRef = useRef(null)

  const handleMenuOpen = event => {
    setAnchorEl(event.currentTarget)
  }

  const handleMenuClose = () => {
    setAnchorEl(null)
  }
  useEffect(() => {
    document.addEventListener('mousedown', handleMenuOutsideClick)
    return () => {
      document.removeEventListener('mousedown', handleMenuOutsideClick)
    }
  }, [anchorEl])

  const handleMenuOutsideClick = event => {
    if (menuRef.current && !menuRef.current.contains(event.target)) {
      handleMenuClose()
    }
  }

  return (
    <>
      {!label && (
        <IconButton
          onClick={handleMenuOpen}
          variant='outlined'
          color={isActive ? 'primary' : 'neutral'}
          size='sm'
          sx={{
            height: 24,
            borderRadius: 24,
          }}
        >
          {icon}
          {label ? label : null}
        </IconButton>
      )}
      {label && (
        <Button
          onClick={handleMenuOpen}
          variant='outlined'
          color={isActive ? 'primary' : 'neutral'}
          size='sm'
          startDecorator={icon}
          sx={{
            height: 24,
            borderRadius: 24,
          }}
        >
          {label}
        </Button>
      )}

      <Menu
        key={k}
        ref={menuRef}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleMenuClose}
      >
        {title && (
          <MenuItem key={`${k}-title`} disabled>
            <Typography level='body-sm' sx={{ fontWeight: 'bold' }}>
              {title}
            </Typography>
          </MenuItem>
        )}
        {options?.map(item => (
          <MenuItem
            key={`${k}-${item?.id}`}
            onClick={() => {
              onItemSelect(item)
              setSelectedItem?.selectedItem(item.name)
              handleMenuClose()
            }}
          >
            {useChips ? (
              <Chip
                sx={{
                  backgroundColor: item.color ? item.color : null,
                  color: getTextColorFromBackgroundColor(item.color),
                }}
              >
                {item.name}
              </Chip>
            ) : (
              <>
                {item?.icon}
                {item.name}
              </>
            )}
          </MenuItem>
        ))}
        {/* {selectedItem && selectedItem !== 'All' && (
            <MenuItem
              id={`${id}cancel-all-filters`}
              onClick={() => {
                onItemSelect(null)
                setSelectedItem?.setSelectedItem('All')
              }}
            >
              Cancel All Filters
            </MenuItem>
          )} */}
      </Menu>
    </>
  )
}
export default IconButtonWithMenu
