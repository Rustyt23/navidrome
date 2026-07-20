import React, { useCallback, useMemo } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { Layout as RALayout, toggleSidebar } from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import { HotKeys } from 'react-hotkeys'
import { useLocation } from 'react-router-dom'
import Menu from './Menu'
import AppBar from './AppBar'
import Notification from './Notification'
import useCurrentTheme from '../themes/useCurrentTheme'
import { useSearchRefocus } from '../common'

const useStyles = makeStyles({
  root: { paddingBottom: (props) => (props.addPadding ? '80px' : 0) },
})

const Empty = () => null

const Layout = (props) => {
  const baseTheme = useCurrentTheme()
  // React-admin renders its refresh button inside the AppBar and gives no prop
  // to remove it, so hide it here — QuickScanButton takes its place. Overriding
  // by stylesheet name (rather than the generated class) keeps this working in
  // production builds, where JSS drops the readable class name prefix.
  const theme = useMemo(
    () => ({
      ...baseTheme,
      overrides: {
        ...baseTheme.overrides,
        RaLoadingIndicator: {
          ...baseTheme.overrides?.RaLoadingIndicator,
          loadedIcon: { display: 'none' },
        },
      },
    }),
    [baseTheme],
  )
  const queue = useSelector((state) => state.player?.queue)
  const location = useLocation()
  const hideNavigation = useMemo(() => {
    const path = location?.pathname || ''
    if (/^\/(retailplayer|musicmatters|player)\//.test(path)) {
      return true
    }
    // Public (anonymous) visitors of shared retail player folder links get a
    // standalone page: the app chrome fires authenticated requests that would
    // bounce them to the login screen.
    if (/^\/retail-player\//.test(path)) {
      return localStorage.getItem('is-authenticated') !== 'true'
    }
    return false
  }, [location.pathname])
  const classes = useStyles({ addPadding: !hideNavigation && queue.length > 0 })
  const dispatch = useDispatch()
  useSearchRefocus()

  const keyHandlers = {
    TOGGLE_MENU: useCallback(() => dispatch(toggleSidebar()), [dispatch]),
  }

  return (
    <HotKeys handlers={keyHandlers}>
      <RALayout
        {...props}
        className={classes.root}
        menu={hideNavigation ? Empty : Menu}
        appBar={hideNavigation ? Empty : AppBar}
        sidebar={hideNavigation ? Empty : undefined}
        theme={theme}
        notification={Notification}
      />
    </HotKeys>
  )
}

export default Layout
