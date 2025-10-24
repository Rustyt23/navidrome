import React from 'react'
import { Paper, Typography, makeStyles } from '@material-ui/core'

const useStyles = makeStyles((theme) => ({
  root: {
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius * 1.5,
    border: `1px solid ${theme.palette.divider}`,
    padding: theme.spacing(4),
    boxShadow: theme.shadows[1],
    maxWidth: 720,
  },
  heading: {
    marginBottom: theme.spacing(1.5),
  },
}))

const ChannelsPage = () => {
  const classes = useStyles()

  return (
    <Paper className={classes.root} elevation={0}>
      <Typography variant="h4" component="h1" className={classes.heading}>
        Channels
      </Typography>
      <Typography variant="body1" color="textSecondary">
        Coming soon.
      </Typography>
    </Paper>
  )
}

export default ChannelsPage
