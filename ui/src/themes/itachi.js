import red from '@material-ui/core/colors/red'
import stylesheet from './itachi.css.js'

export default {
  themeName: 'Itachi',
  palette: {
    background: {
      paper: '#0d0d0d',
      default: '#0d0d0d',
    },
    primary: {
      main: '#e53935',
      contrastText: '#f5f5f5',
    },
    secondary: red,
    type: 'dark',
  },
  overrides: {
    MuiFormGroup: {
      root: {
        color: 'white',
      },
    },
    NDLogin: {
      systemNameLink: {
        color: '#e53935',
      },
      welcome: {
        color: '#eee',
      },
    },
  },
  player: {
    theme: 'dark',
    stylesheet,
  },
}
