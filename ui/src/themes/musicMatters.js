import stylesheet from './musicMatters.css.js'
import '../styles/themes/music-matters.css'

const colors = {
  accent: '#FF2B8A',
  accent600: '#E11D74',
  bg: '#111827',
  surface1: '#1F2937',
  surface2: '#2A3444',
  border: '#404B5F',
  text: '#FFFFFF',
  textMuted: '#D1D5DB',
}

export default {
  themeName: 'Music Matters',
  themeId: 'music-matters',
  palette: {
    primary: {
      main: colors.accent,
    },
    secondary: {
      main: colors.accent,
    },
    background: {
      default: colors.bg,
      paper: colors.surface1,
    },
    text: {
      primary: colors.text,
      secondary: colors.textMuted,
    },
    type: 'dark',
  },
  overrides: {
    MuiPaper: {
      root: {
        backgroundColor: colors.surface1,
        color: colors.text,
      },
    },
    MuiAppBar: {
      colorSecondary: {
        color: colors.text,
      },
      positionFixed: {
        backgroundColor: colors.surface1,
        boxShadow: '0 6px 24px rgba(0,0,0,0.35)',
      },
    },
    MuiButton: {
      root: {
        border: '1px solid transparent',
        '&:hover': {
          backgroundColor: colors.accent600,
        },
      },
      contained: {
        backgroundColor: colors.accent,
        color: colors.text,
        '&:hover': {
          backgroundColor: colors.accent600,
        },
      },
    },
    MuiChip: {
      root: {
        backgroundColor: 'rgba(255,43,138,0.15)',
        border: '1px solid transparent',
      },
      label: {
        color: '#FFD2E7',
      },
    },
    MuiTableRow: {
      root: {
        '&:hover': {
          backgroundColor: 'rgba(255,255,255,0.06) !important',
        },
      },
      head: {
        backgroundColor: colors.surface2,
      },
    },
    MuiTableCell: {
      root: {
        borderBottom: `1px solid ${colors.border}`,
      },
    },
    MuiInputBase: {
      root: {
        color: colors.text,
      },
    },
    MuiOutlinedInput: {
      notchedOutline: {
        borderColor: colors.border,
      },
    },
    MuiTabs: {
      indicator: {
        backgroundColor: colors.accent,
      },
    },
    MuiTab: {
      root: {
        '&$selected': {
          color: colors.text,
        },
      },
    },
    MuiSlider: {
      track: {
        color: colors.accent,
      },
      rail: {
        color: '#3C465A',
      },
      thumb: {
        color: '#FFFFFF',
        border: `2px solid ${colors.accent}`,
      },
    },
  },
  player: {
    theme: 'dark',
    stylesheet,
  },
}
