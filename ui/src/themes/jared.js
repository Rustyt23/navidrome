import stylesheet from './musicMatters.css.js'

const mmCol = {
  primary: '#ffffff',
  secondary: '#cfcfcf',
  accent: '#e91e63',
  text: '#212121',
  textAlt: '#616161',
  icon: '#212121',
  link: '#e91e63',
  border: '#bdbdbd',
}

export default {
  themeName: 'MusicMatters',
  palette: {
    primary: {
      main: mmCol['secondary'],
    },
    secondary: {
      main: mmCol['accent'],
    },
    background: {
      default: mmCol['primary'],
    },
    text: {
      primary: mmCol['text'],
      secondary: mmCol['textAlt'],
    },
    type: 'light',
  },
  overrides: {
    MuiTypography: {
      root: {
        color: mmCol['text'],
      },
      colorPrimary: {
        color: mmCol['text'],
      },
    },
    MuiPaper: {
      root: {
        backgroundColor: mmCol['primary'],
      },
    },
    MuiFormGroup: {
      root: {
        color: mmCol['text'],
      },
    },
    NDAlbumGridView: {
      albumName: {
        marginTop: '0.5rem',
        fontWeight: 700,
        textTransform: 'none',
        color: mmCol['text'],
      },
      albumSubtitle: {
        color: mmCol['textAlt'],
      },
    },
    MuiAppBar: {
      colorSecondary: {
        color: mmCol['accent'],
      },
      positionFixed: {
        backgroundColor: mmCol['secondary'],
        boxShadow:
          'rgba(0, 0, 0, 0.1) 0px 4px 6px, rgba(0, 0, 0, 0.06) 0px 5px 7px',
      },
    },
    MuiButton: {
      root: {
        border: '1px solid transparent',
        '&:hover': {
          backgroundColor: mmCol['accent'],
        },
      },
      label: {
        color: mmCol['text'],
      },
      contained: {
        boxShadow: 'none',
        '&:hover': {
          boxShadow: 'none',
        },
      },
    },
    MuiChip: {
      root: {
        backgroundColor: mmCol['accent'],
      },
      label: {
        color: '#ffffff',
      },
    },
    RaLink: {
      link: {
        color: mmCol['link'],
      },
    },
    MuiTableCell: {
      root: {
        borderBottom: 'none',
        color: mmCol['text'],
        padding: '10px !important',
      },
      head: {
        fontSize: '0.75rem',
        textTransform: 'uppercase',
        letterSpacing: 1.2,
        backgroundColor: mmCol['secondary'],
        color: mmCol['text'],
      },
      body: {
        color: mmCol['text'],
      },
    },
    MuiInput: {
      root: {
        color: mmCol['text'],
      },
    },
    MuiFormLabel: {
      root: {
        '&$focused': {
          color: mmCol['text'],
          fontWeight: 'bold',
        },
      },
    },
    MuiOutlinedInput: {
      notchedOutline: {
        borderColor: mmCol['border'],
      },
    },
    // Icons inherit the current color to allow contextual theming
    MuiIconButton: {
      label: {
        color: 'inherit',
      },
    },
    MuiListItemIcon: {
      root: {
        color: 'inherit',
      },
    },
    MuiSelect: {
      icon: {
        color: 'inherit',
      },
    },
    MuiSvgIcon: {
      root: {
        color: 'inherit',
      },
      colorDisabled: {
        color: 'inherit',
      },
    },
    MuiSwitch: {
      colorPrimary: {
        '&$checked + $track': {
          backgroundColor: mmCol['accent'],
        },
      },
      track: {
        backgroundColor: mmCol['border'],
      },
    },
    RaButton: {
      smallIcon: {
        color: 'inherit',
      },
    },
    RaDatagrid: {
      headerCell: {
        backgroundColor: mmCol['secondary'],
      },
    },
    //Login Screen
    NDLogin: {
      systemNameLink: {
        color: mmCol['text'],
      },
      card: {
        minWidth: 300,
        backgroundColor: mmCol['primary'],
      },
      button: {
        boxShadow: '3px 3px 5px #000000a3',
      },
    },
    NDMobileArtistDetails: {
      bgContainer: {
        background:
          'linear-gradient(to bottom, rgba(255 255 255 / 72%), rgb(240 240 240))!important',
      },
    },
  },
  player: {
    theme: 'light',
    stylesheet,
  },
}
