import React, { useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { verifyRetailPlayerLockPassword } from './deviceLock'

const useStyles = makeStyles((theme) => ({
  helperText: {
    marginTop: theme.spacing(1),
    minHeight: theme.spacing(2.5),
  },
}))

const RetailPlayerUnlockDialog = ({ open, deviceName, onUnlocked }) => {
  const classes = useStyles()
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleSubmit = async (event) => {
    event.preventDefault()
    if (isSubmitting) {
      return
    }

    const trimmedPassword = password.trim()
    if (!trimmedPassword) {
      setError('Please enter the device password.')
      return
    }

    setIsSubmitting(true)
    setError('')

    try {
      await verifyRetailPlayerLockPassword(trimmedPassword)
      setPassword('')
      setError('')
      onUnlocked()
    } catch (requestError) {
      if (requestError?.message === 'INVALID_PASSWORD') {
        setError('Incorrect password. Please try again.')
      } else {
        setError('Unable to verify password right now. Please try again.')
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      disableEscapeKeyDown
      fullWidth
      maxWidth="xs"
      onClose={(event, reason) => {
        if (reason === 'backdropClick' || reason === 'escapeKeyDown') {
          return
        }
      }}
      aria-labelledby="retail-player-unlock-dialog-title"
    >
      <form onSubmit={handleSubmit}>
        <DialogTitle id="retail-player-unlock-dialog-title">
          Unlock Device
        </DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="textSecondary">
            {deviceName
              ? `Enter the password to access ${deviceName}.`
              : 'Enter the password to access this device.'}
          </Typography>
          <TextField
            autoFocus
            margin="dense"
            label="Password"
            type="password"
            fullWidth
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            inputProps={{ 'aria-label': 'Device unlock password' }}
          />
          <Typography
            variant="caption"
            color={error ? 'error' : 'textSecondary'}
            className={classes.helperText}
          >
            {error || ' '}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            type="submit"
            color="primary"
            variant="contained"
            disabled={isSubmitting}
          >
            {isSubmitting ? 'Unlocking…' : 'Unlock'}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  )
}

RetailPlayerUnlockDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  deviceName: PropTypes.string,
  onUnlocked: PropTypes.func.isRequired,
}

RetailPlayerUnlockDialog.defaultProps = {
  deviceName: '',
}

export default RetailPlayerUnlockDialog
