import useStickyState from '@/hooks/useStickyState'
import {
  BrightnessAuto,
  DarkModeOutlined,
  LightModeOutlined,
} from '@mui/icons-material'
import { FormControl, IconButton, useColorScheme } from '@mui/joy'

const ELEMENTID = 'select-theme-mode'

const ThemeToggleButton = ({ sx }) => {
  const { mode, setMode } = useColorScheme()
  const [themeMode, setThemeMode] = useStickyState(mode, 'themeMode')

  const handleThemeModeChange = e => {
    e.preventDefault()
    e.stopPropagation()

    let newThemeMode
    switch (themeMode) {
      case 'light':
        newThemeMode = 'dark'
        break
      case 'dark':
        newThemeMode = 'system'
        break
      case 'system':
      default:
        newThemeMode = 'light'
        break
    }
    setThemeMode(newThemeMode)
    setMode(newThemeMode)
  }

  return (
    <FormControl sx={sx}>
      <IconButton onClick={handleThemeModeChange}>
        {themeMode === 'light' ? (
          <DarkModeOutlined />
        ) : themeMode === 'dark' ? (
          <BrightnessAuto />
        ) : (
          <LightModeOutlined />
        )}
      </IconButton>
    </FormControl>
  )
}

export default ThemeToggleButton
