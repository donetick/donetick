import resolveConfig from 'tailwindcss/resolveConfig'
import tailwindConfig from '/tailwind.config.js'

export const { theme: THEME } = resolveConfig(tailwindConfig)

export const COLORS = THEME.colors
