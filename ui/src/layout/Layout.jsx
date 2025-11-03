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

const useStyles = makeStyles({
  root: { paddingBottom: (props) => (props.addPadding ? '80px' : 0) },
})

const Empty = () => null

const Layout = (props) => {
  const theme = useCurrentTheme()
  const queue = useSelector((state) => state.player?.queue)
  const location = useLocation()
  const hideNavigation = useMemo(() => {
    const path = location?.pathname || ''
    return /^\/(retailplayer|musicmatters)\//.test(path)
  }, [location.pathname])
  const classes = useStyles({ addPadding: !hideNavigation && queue.length > 0 })
  const dispatch = useDispatch()

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
