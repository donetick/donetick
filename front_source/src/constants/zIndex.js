// Z-index constants for consistent layering
// Lower values appear behind higher values

export const Z_INDEX = {
  // Base layer (0-99)
  BASE: 0,
  CARD_OVERLAY: 1,
  DROPDOWN_ITEM: 2,
  TOOLTIP: 3,

  // UI Components (100-999)
  SAFE_AREA: 100,
  CALENDAR: 110,
  SMART_INPUT: 110,
  AUTOCOMPLETE: 200,

  // Navigation (1000-1999)
  NAVBAR: 1000,
  DRAWER: 999,

  // Modals and Overlays (2000-8999)
  MODAL_BACKDROP: 2000,
  MODAL_CONTENT: 2001,
  TOAST: 3000,

  // Critical System UI (9000-9999)
  LOADING_SCREEN: 9000,
  ALERTS: 9500,
  NETWORK_BANNER: 9600,

  // Maximum (10000+) - Reserved for absolute emergencies
  EMERGENCY: 10000,
}

export default Z_INDEX
