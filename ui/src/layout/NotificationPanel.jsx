import React, { useState } from 'react'
import {
  Badge,
  Button,
  Divider,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Popover,
  Tooltip,
  Typography,
  makeStyles,
} from '@material-ui/core'
import { MdNotificationsNone } from 'react-icons/md'

const initialNotifications = [
  {
    id: 1,
    title: 'Library Updated',
    description: 'New albums have been added to your library.',
    time: '2 minutes ago',
  },
  {
    id: 2,
    title: 'Playlist Shared',
    description: 'Anna just shared the “Weekend Vibes” playlist with you.',
    time: '15 minutes ago',
  },
  {
    id: 3,
    title: 'Sync Complete',
    description: 'Your latest device sync finished successfully.',
    time: 'Today, 9:24 AM',
  },
]

const useStyles = makeStyles((theme) => ({
  triggerButton: {
    color: 'inherit',
  },
  badge: {
    '& .MuiBadge-badge': {
      backgroundColor: theme.palette.primary.main,
      color: theme.palette.primary.contrastText,
      minWidth: theme.spacing(2.5),
      height: theme.spacing(2.5),
      fontSize: '0.65rem',
      padding: 0,
    },
  },
  popoverPaper: {
    backgroundColor: theme.palette.background.paper,
    minWidth: '20rem',
    maxWidth: '22rem',
    borderRadius: theme.shape.borderRadius,
    boxShadow: theme.shadows[8],
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: theme.spacing(1.5, 2),
  },
  list: {
    maxHeight: '16rem',
    overflowY: 'auto',
    padding: 0,
  },
  notificationItem: {
    alignItems: 'flex-start',
    padding: theme.spacing(1.5, 2),
  },
  description: {
    color: theme.palette.text.secondary,
    marginTop: theme.spacing(0.5),
  },
  timestamp: {
    color: theme.palette.text.hint,
    fontSize: '0.75rem',
    marginTop: theme.spacing(0.75),
  },
  emptyState: {
    padding: theme.spacing(2),
    textAlign: 'center',
    color: theme.palette.text.secondary,
  },
  clearButton: {
    textTransform: 'none',
  },
}))

const NotificationPanel = () => {
  const classes = useStyles()
  const [anchorEl, setAnchorEl] = useState(null)
  const [notifications, setNotifications] = useState(() => initialNotifications)

  const open = Boolean(anchorEl)
  const count = notifications.length

  const handleToggle = (event) => {
    if (open) {
      setAnchorEl(null)
    } else {
      setAnchorEl(event.currentTarget)
    }
  }

  const handleClose = () => setAnchorEl(null)

  const handleClearAll = () => {
    setNotifications([])
  }

  return (
    <>
      <Tooltip title="Notifications">
        <IconButton
          className={classes.triggerButton}
          aria-label="Notifications"
          aria-haspopup="true"
          onClick={handleToggle}
          size="small"
        >
          <Badge
            badgeContent={count}
            max={99}
            color="primary"
            overlap="rectangular"
            className={classes.badge}
          >
            <MdNotificationsNone size={20} />
          </Badge>
        </IconButton>
      </Tooltip>
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        classes={{ paper: classes.popoverPaper }}
      >
        <div className={classes.header}>
          <Typography variant="subtitle1" color="textPrimary">
            Notifications
          </Typography>
          <Button
            size="small"
            color="primary"
            onClick={handleClearAll}
            disabled={count === 0}
            className={classes.clearButton}
          >
            Clear All
          </Button>
        </div>
        <Divider />
        {count > 0 ? (
          <List className={classes.list} disablePadding>
            {notifications.map((notification, index) => (
              <React.Fragment key={notification.id}>
                <ListItem className={classes.notificationItem} alignItems="flex-start">
                  <ListItemText
                    primary={
                      <Typography variant="subtitle2" color="textPrimary">
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
                {index < notifications.length - 1 && <Divider component="li" />}
              </React.Fragment>
            ))}
          </List>
        ) : (
          <div className={classes.emptyState}>
            <Typography variant="body2">You're all caught up!</Typography>
          </div>
        )}
      </Popover>
    </>
  )
}

export default NotificationPanel
