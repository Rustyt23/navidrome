import LightTheme from './light'
import DarkTheme from './dark'
import ExtraDarkTheme from './extradark'
import GreenTheme from './green'
import SpotifyTheme from './spotify'
import LigeraTheme from './ligera'
import MonokaiTheme from './monokai'
import MusicMattersTheme from './musicMatters'
import ElectricPurpleTheme from './electricPurple'
import NordTheme from './nord'
import GruvboxDarkTheme from './gruvboxDark'
import CatppuccinMacchiatoTheme from './catppuccinMacchiato'
import NuclearTheme from './nuclear'
import ItachiTheme from './itachi'

const themes = {
  // Classic default themes
  LightTheme,
  DarkTheme,

  // New themes should be added here, in alphabetic order
  CatppuccinMacchiatoTheme,
  ElectricPurpleTheme,
  ExtraDarkTheme,
  GreenTheme,
  GruvboxDarkTheme,
  ItachiTheme,
  LigeraTheme,
  MonokaiTheme,
  MusicMattersTheme,
  NordTheme,
  NuclearTheme,
  SpotifyTheme,
}

const commonMuiTableRoot = {
  borderCollapse: 'separate',
  borderSpacing: '0 1px',
}

const applyCommonOverrides = (theme) => {
  const overrides = theme.overrides ?? {}
  const muiTable = overrides.MuiTable ?? {}
  const muiTableRoot = muiTable.root ?? {}

  return {
    ...theme,
    overrides: {
      ...overrides,
      MuiTable: {
        ...muiTable,
        root: {
          ...muiTableRoot,
          ...commonMuiTableRoot,
        },
      },
    },
  }
}

export default Object.fromEntries(
  Object.entries(themes).map(([name, theme]) => [
    name,
    applyCommonOverrides(theme),
  ]),
)
