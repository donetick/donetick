import {
  Button,
  Chip,
  Divider,
  Menu,
  MenuItem,
  Radio,
  Typography,
} from '@mui/joy'
import IconButton from '@mui/joy/IconButton'
import { useEffect, useRef, useState } from 'react'
import { getTextColorFromBackgroundColor } from '../../utils/Colors.jsx'

const SortAndGrouping = ({
  label,
  k,
  icon,
  onItemSelect,
  selectedItem,
  setSelectedItem,
  selectedFilter,
  setFilter,
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
    const handleMenuOutsideClick = event => {
      if (menuRef.current && !menuRef.current.contains(event.target)) {
        handleMenuClose()
      }
    }

    document.addEventListener('mousedown', handleMenuOutsideClick)
    return () => {
      document.removeEventListener('mousedown', handleMenuOutsideClick)
    }
  }, [])

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
        sx={{
          '& .MuiMenuItem-root': {
            padding: '8px 16px', // Consistent padding for menu items
          },
        }}
      >
        <MenuItem key={`${k}-title`} disabled>
          <Typography level='body-sm' fontWeight='lg'>
            Group By
          </Typography>
        </MenuItem>

        {[
          { name: 'Smart', value: 'default' },
          { name: 'Due Date', value: 'due_date' },
          { name: 'Priority', value: 'priority' },
          { name: 'Labels', value: 'labels' },
        ].map(item => (
          <MenuItem
            key={`${k}-${item?.value}`}
            onClick={() => {
              onItemSelect(item)
              setSelectedItem?.(item.name)
              handleMenuClose()
            }}
          >
            {useChips ? (
              <Chip
                size='sm'
                sx={{
                  backgroundColor: item.color ? item.color : null,
                  color: getTextColorFromBackgroundColor(item.color),
                  fontWeight: 'md',
                }}
              >
                {item.name}
              </Chip>
            ) : (
              <>
                {item?.icon}
                <Typography level='body-sm' sx={{ ml: 1 }}>
                  {item.name}
                </Typography>
              </>
            )}
          </MenuItem>
        ))}

        <Divider />

        <MenuItem key={`${k}-quick-filter`} disabled>
          <Typography level='body-sm' fontWeight='lg'>
            Quick Filters
          </Typography>
        </MenuItem>

        <MenuItem key={`${k}-assignee-title`} disabled>
          <Typography level='body-xs' fontWeight='md'>
            Assigned to:
          </Typography>
        </MenuItem>

        <MenuItem
          key={`${k}-assignee-anyone`}
          onClick={() => {
            setFilter('anyone')
            handleMenuClose()
          }}
        >
          <Radio checked={selectedFilter === 'anyone'} variant='outlined' />
          <Typography level='body-sm'>Anyone</Typography>
        </MenuItem>

        <MenuItem
          key={`${k}-assignee-assigned-to-me`}
          onClick={() => {
            setFilter('assigned_to_me')
            handleMenuClose()
          }}
        >
          <Radio
            checked={selectedFilter === 'assigned_to_me'}
            variant='outlined'
          />
          <Typography level='body-sm'>Assigned to me</Typography>
        </MenuItem>

        <MenuItem
          key={`${k}-assignee-assigned-to-others`}
          onClick={() => {
            setFilter('assigned_to_others')
            handleMenuClose()
          }}
        >
          <Radio
            checked={selectedFilter === 'assigned_to_others'}
            variant='outlined'
          />
          <Typography level='body-sm'>Assigned to others</Typography>
        </MenuItem>
      </Menu>
    </>
  )
}

export default SortAndGrouping
