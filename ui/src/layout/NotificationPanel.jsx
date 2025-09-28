import React, { useMemo, useState, useCallback, useEffect } from 'react'
import {
  Badge,
  Box,
  Button,
  Divider,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Popover,
  Typography,
  makeStyles,
} from '@material-ui/core'
import { FiBell } from 'react-icons/fi'
import {
  subscribeToNotifications,
  getNotifications,
  clearNotifications,
} from './notificationStore'

const useStyles = makeStyles((theme) => ({
  button: {
    color: 'inherit',
  },
  badge: {
    '& .MuiBadge-badge': {
      minWidth: theme.spacing(2.5),
      height: theme.spacing(2.5),
      fontSize: '0.7rem',
      fontWeight: 600,
      padding: theme.spacing(0.25, 0.75),
      backgroundColor: theme.palette.primary.main,
      color: theme.palette.primary.contrastText,
      boxShadow: `0 0 0 2px ${theme.palette.background.default}`,
    },
  },
  popoverPaper: {
    width: 320,
    maxWidth: '80vw',
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.spacing(1.5),
    boxShadow: theme.shadows[8],
    color: theme.palette.text.primary,
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: theme.spacing(2, 2, 1.5, 2),
  },
  list: {
    maxHeight: 320,
    overflowY: 'auto',
    padding: 0,
  },
  item: {
    alignItems: 'flex-start',
    padding: theme.spacing(1.5, 2),
    '&:not(:last-child)': {
      borderBottom: `1px solid ${theme.palette.divider}`,
    },
  },
  title: {
    fontWeight: 600,
    color: theme.palette.text.primary,
  },
  description: {
    marginTop: theme.spacing(0.5),
    color: theme.palette.text.secondary,
  },
  timestamp: {
    marginTop: theme.spacing(0.75),
    display: 'block',
    fontSize: '0.75rem',
    color: theme.palette.text.secondary,
  },
  empty: {
    padding: theme.spacing(3),
    textAlign: 'center',
    color: theme.palette.text.secondary,
  },
  clearButton: {
    color: theme.palette.primary.main,
    textTransform: 'none',
    fontWeight: 600,
  },
}))

const NotificationPanel = () => {
  const classes = useStyles()
  const [anchorEl, setAnchorEl] = useState(null)
  const [notifications, setNotifications] = useState(() => getNotifications())

  useEffect(() => subscribeToNotifications(setNotifications), [])

  const badgeCount = notifications.length
  const open = Boolean(anchorEl)

  const handleToggle = useCallback(
    (event) => {
      if (open) {
        setAnchorEl(null)
      } else {
        setAnchorEl(event.currentTarget)
      }
    },
    [open],
  )

  const handleClose = useCallback(() => setAnchorEl(null), [])

  const handleClear = useCallback(() => {
    clearNotifications()
  }, [])

  const notificationItems = useMemo(
    () =>
      notifications.map((notification) => (
        <ListItem key={notification.id} className={classes.item} alignItems="flex-start">
          <ListItemText
            primary={
              <Typography variant="body1" className={classes.title}>
                {notification.title}
              </Typography>
            }
            secondary={
              <>
                <Typography variant="body2" className={classes.description}>
                  {notification.description}
                </Typography>
                <Typography variant="caption" className={classes.timestamp}>
                  {notification.time}
                </Typography>
              </>
            }
          />
        </ListItem>
      )),
    [classes.description, classes.item, classes.timestamp, classes.title, notifications],
  )

  return (
    <>
      <IconButton
        className={classes.button}
        aria-haspopup="true"
        aria-controls={open ? 'notification-panel' : undefined}
        aria-expanded={open ? 'true' : undefined}
        aria-label="Notifications"
        onClick={handleToggle}
      >
        <Badge
          badgeContent={badgeCount}
          color="primary"
          className={classes.badge}
          invisible={badgeCount === 0}
        >
          <FiBell size={20} />
        </Badge>
      </IconButton>
      <Popover
        id="notification-panel"
        open={open}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        classes={{ paper: classes.popoverPaper }}
      >
        <Box className={classes.header}>
          <Typography variant="subtitle1">Notifications</Typography>
          {notifications.length > 0 && (
            <Button size="small" onClick={handleClear} className={classes.clearButton}>
              Clear All
            </Button>
          )}
        </Box>
        <Divider />
        {notifications.length > 0 ? (
          <List className={classes.list}>{notificationItems}</List>
        ) : (
          <Typography variant="body2" className={classes.empty}>
            You\'re all caught up!
          </Typography>
        )}
      </Popover>
    </>
  )
}

export default NotificationPanel
