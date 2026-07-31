import React, { useCallback, useState } from 'react'
import PropTypes from 'prop-types'
import { useNotify, useTranslate } from 'react-admin'
import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
  Switch,
  Tooltip,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { httpClient } from '../dataProvider'

const SETTINGS_URL = '/api/song/loudness/settings'

const useStyles = makeStyles((theme) => ({
  root: {
    display: 'flex',
    alignItems: 'center',
    marginRight: theme.spacing(1),
  },
  label: { whiteSpace: 'nowrap' },
  warn: { color: theme.palette.error.main },
}))

// BackupToggle controls whether an untouched original is kept before a song is
// rewritten.
//
// Keeping one costs about as much disk as the library itself, which a client
// who already holds the masters elsewhere may not want to pay twice. It is
// still the only setting on this page that cannot be undone: without a stored
// original there is nothing for Restore to put back, and no way to show
// afterwards that only the level changed. So turning it off asks first, and
// says what is being given up rather than warning vaguely.
export const BackupToggle = ({
  settings,
  onChange,
  disabled,
  libraryStatus,
}) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const [saving, setSaving] = useState(false)
  const [confirming, setConfirming] = useState(false)

  const running = !!libraryStatus?.running
  const enabled = !!settings?.backup

  const save = useCallback(
    (backup) => {
      setSaving(true)
      httpClient(SETTINGS_URL, {
        method: 'PUT',
        body: JSON.stringify({ backup }),
      })
        .then(({ json }) => {
          onChange?.(json)
          notify(
            backup
              ? 'resources.lufs.notifications.backupOn'
              : 'resources.lufs.notifications.backupOff',
            backup ? 'info' : 'warning',
          )
        })
        .catch((error) => {
          notify(error?.message || 'ra.notification.http_error', 'warning')
        })
        .finally(() => setSaving(false))
    },
    [notify, onChange],
  )

  const handleToggle = useCallback(
    (event) => {
      // Turning it back on is harmless and immediate. Turning it off is what
      // needs agreeing to.
      if (event.target.checked) {
        save(true)
        return
      }
      setConfirming(true)
    },
    [save],
  )

  if (!settings) {
    return null
  }

  const hint = running
    ? translate('resources.lufs.backupLockedHelper')
    : translate(
        enabled
          ? 'resources.lufs.backupOnHelper'
          : 'resources.lufs.backupOffHelper',
      )

  return (
    <>
      <Tooltip title={hint}>
        <div className={classes.root}>
          <FormControlLabel
            classes={{ label: enabled ? classes.label : classes.warn }}
            control={
              <Switch
                checked={enabled}
                onChange={handleToggle}
                // Changing this mid-run would leave one half of the run
                // restorable and the other half not.
                disabled={saving || disabled || running}
                color="primary"
              />
            }
            label={translate('resources.lufs.actions.keepOriginals')}
          />
          {saving && <CircularProgress size={16} />}
        </div>
      </Tooltip>

      <Dialog open={confirming} onClose={() => setConfirming(false)}>
        <DialogTitle>
          {translate('resources.lufs.backupOffConfirm.title')}
        </DialogTitle>
        <DialogContent>
          <DialogContentText>
            {translate('resources.lufs.backupOffConfirm.body')}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirming(false)} color="primary">
            {translate('ra.action.cancel')}
          </Button>
          <Button
            onClick={() => {
              setConfirming(false)
              save(false)
            }}
            className={classes.warn}
          >
            {translate('resources.lufs.backupOffConfirm.confirm')}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}

BackupToggle.propTypes = {
  settings: PropTypes.object,
  onChange: PropTypes.func,
  disabled: PropTypes.bool,
  libraryStatus: PropTypes.object,
}

export default BackupToggle
