import { Typography } from '@material-ui/core'
import { makeStyles } from '@material-ui/styles'
import InboxIcon from '@material-ui/icons/Inbox'
import DiscoveryFolderCreateButton from './DiscoveryFolderCreateButton'

const useStyles = makeStyles(theme => ({
  root: { flex: 1 },
  message: {
    textAlign: 'center', margin: '0 1em', color: theme.palette.text.disabled,
  },
  icon: {
    width: '9em', height: '9em', color: theme.palette.text.disabled,
  },
  title: { fontSize: '1.75rem', fontWeight: 400, marginBottom: theme.spacing(1) },
  subtitle: { fontSize: '1rem', fontWeight: 400, marginTop: theme.spacing(1) },
  toolbar: { textAlign: 'center', marginTop: '2em' },
}))

const EmptyDiscovery = () => {
  const classes = useStyles()
  return (
    <span className={classes.root}>
      <div className={classes.message}>
        <InboxIcon className={classes.icon} />
        <Typography className={classes.title}>No Discovery yet.</Typography>
        <Typography className={classes.subtitle}>Do you want to add one?</Typography>
      </div>
      <div className={classes.toolbar}>
        <DiscoveryFolderCreateButton />
      </div>
    </span>
  )
}

export default EmptyDiscovery
