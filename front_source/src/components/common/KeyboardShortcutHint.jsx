import { Chip } from '@mui/joy'
import PropTypes from 'prop-types'

/**
 * A component that displays keyboard shortcut hints as small chips
 * Only visible on non-mobile devices
 * Supports platform-specific shortcuts (Cmd on Mac, Ctrl on Windows) and Shift key
 */
function KeyboardShortcutHint({
  shortcut,
  show = true,
  withCmd = true,
  withShift = false,
  sx = {},
  ...props
}) {
  if (!show) return null

  const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0
  const modifierKey = isMac ? '⌘' : 'Ctrl'

  // Build the shortcut display string
  let displayShortcut = ''
  if (withCmd) {
    displayShortcut += modifierKey
  }
  if (withShift) {
    displayShortcut += (displayShortcut ? ' + ' : '') + 'Shift'
  }
  if (shortcut) {
    displayShortcut += (displayShortcut ? ' + ' : '') + shortcut
  }

  return (
    <Chip
      size='sm'
      variant='outlined'
      color='neutral'
      sx={{
        fontSize: '0.75rem',
        maxHeight: '1.5rem',
        fontFamily: 'system-ui, -apple-system, sans-serif',
        fontWeight: '500',
        letterSpacing: '0.025em',
        border: '1px solid',
        borderColor: 'neutral.300',
        // backgroundColor: 'background.surface',
        color: 'text.secondary',
        borderRadius: '8px',
        px: 0.5,
        py: 0.125,
        boxShadow: '0 1px 2px rgba(0, 0, 0, 0.05)',
        display: { xs: 'none', md: 'inline-flex' }, // Hide on mobile
        ...sx,
      }}
      {...props}
    >
      {displayShortcut}
    </Chip>
  )
}

KeyboardShortcutHint.propTypes = {
  shortcut: PropTypes.string.isRequired,
  show: PropTypes.bool,
  withCmd: PropTypes.bool,
  withShift: PropTypes.bool,
  sx: PropTypes.object,
}

export default KeyboardShortcutHint
