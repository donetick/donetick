/**
 * Utility functions for platform detection
 */

/**
 * Detects if the current platform is macOS using modern APIs with fallback
 * @returns {boolean} True if running on macOS, false otherwise
 */
export const isMacOS = () => {
  // Modern approach using User-Agent Client Hints API
  if (navigator.userAgentData) {
    return navigator.userAgentData.platform === 'macOS'
  }

  // Fallback for older browsers
  return /Mac|iPhone|iPad|iPod/.test(navigator.userAgent)
}

/**
 * Gets the appropriate keyboard shortcut text for the current platform
 * @param {string} key - The key combination (e.g., 'F', 'K', 'S')
 * @param {boolean} withCtrl - Whether to include Ctrl/Cmd modifier
 * @param {boolean} withShift - Whether to include Shift modifier
 * @returns {string} Platform-appropriate keyboard shortcut text
 */
export const getKeyboardShortcut = (
  key,
  withCtrl = true,
  withShift = false,
) => {
  let shortcut = ''

  if (withCtrl) {
    const modifier = isMacOS() ? '⌘' : 'Ctrl+'
    shortcut += modifier
  }

  if (withShift) {
    if (isMacOS()) {
      shortcut += '⇧'
    } else {
      shortcut += 'Shift+'
    }
  }

  shortcut += key
  return shortcut
}

/**
 * Gets common keyboard shortcuts for the current platform
 */
export const getCommonShortcuts = () => ({
  search: getKeyboardShortcut('F'),
  newTask: getKeyboardShortcut('K'),
  selectAll: getKeyboardShortcut('A'),
  multiSelect: getKeyboardShortcut('S'),
  save: getKeyboardShortcut('S'),
  copy: getKeyboardShortcut('C'),
  paste: getKeyboardShortcut('V'),
  undo: getKeyboardShortcut('Z'),
  redo: getKeyboardShortcut('Z', true, true), // Ctrl/Cmd + Shift + Z
})
