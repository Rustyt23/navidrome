import React, { useCallback, useEffect, useState } from 'react'
import { useNotify, useTranslate } from 'react-admin'
import {
  CircularProgress,
  FormControlLabel,
  Switch,
  Tooltip,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { httpClient } from '../dataProvider'

const SETTINGS_URL = '/api/song/loudness/settings'

// How long to keep showing the switch as on while waiting for the server to
// report the run it was asked to start.
const STARTING_TIMEOUT_MS = 10000

const useStyles = makeStyles((theme) => ({
  root: {
    display: 'flex',
    alignItems: 'center',
    marginRight: theme.spacing(1),
  },
  label: {
    whiteSpace: 'nowrap',
  },
}))

// LufsToggle starts a whole-library optimisation run.
//
// It shows the run, not a stored preference. The two used to be different
// things: the switch wrote a setting that nothing ever cleared, so it stayed on
// after a run had finished, and turning it off while one was going reported
// "off" while the files were still being rewritten. Reading the job's own state
// means the switch cannot claim anything the server is not actually doing, and
// it goes off by itself when the run ends.
//
// It is also locked while a run is going. Stopping is the stop button's job -
// one control for one action - and the switch cannot be used to interrupt work
// or to start a second run on top of the first.
export const LufsToggle = ({
  onChange,
  onToggled,
  disabled,
  libraryStatus,
}) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const [settings, setSettings] = useState(null)
  const [saving, setSaving] = useState(false)
  // The run is started by the same request that saves the setting, so there is
  // a moment before the first status poll where the server is starting up a run
  // it has not reported yet. Without this the switch would spring back to off
  // and look like the click failed.
  const [starting, setStarting] = useState(false)

  const running = !!libraryStatus?.running
  const stopping = !!libraryStatus?.stopping

  const apply = useCallback(
    (json) => {
      setSettings(json)
      onChange?.(json)
    },
    [onChange],
  )

  useEffect(() => {
    let active = true
    httpClient(SETTINGS_URL)
      .then(({ json }) => {
        if (active) apply(json)
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [apply])

  // Once the run is really going, or has turned out not to start at all, the
  // optimistic flag has done its job.
  useEffect(() => {
    if (running) setStarting(false)
  }, [running])

  const handleToggle = useCallback(
    (event) => {
      const enabled = event.target.checked
      setSaving(true)
      if (enabled) setStarting(true)
      httpClient(SETTINGS_URL, {
        method: 'PUT',
        body: JSON.stringify({ enabled }),
      })
        .then(({ json }) => {
          apply(json)
          onToggled?.(enabled)
          notify(
            enabled
              ? 'resources.lufs.notifications.enabled'
              : 'resources.lufs.notifications.disabled',
            'info',
          )
        })
        .catch((error) => {
          setStarting(false)
          notify(error?.message || 'ra.notification.http_error', 'warning')
        })
        .finally(() => setSaving(false))
    },
    [apply, notify, onToggled],
  )

  // A run refused for a reason the switch cannot see - another job holding the
  // library, say - would otherwise leave it stuck on with nothing behind it.
  useEffect(() => {
    if (!starting) return undefined
    const timer = setTimeout(() => setStarting(false), STARTING_TIMEOUT_MS)
    return () => clearTimeout(timer)
  }, [starting])

  if (!settings) {
    return null
  }

  const busy = running || stopping || starting
  const hint = stopping
    ? translate('resources.lufs.toggleStopping')
    : busy
      ? translate('resources.lufs.toggleRunning')
      : translate('resources.lufs.toggleHelper', {
          target: settings.targetLUFS?.toFixed(1),
          tolerance: settings.tolerance?.toFixed(1),
        })

  return (
    <Tooltip title={hint}>
      <div className={classes.root}>
        <FormControlLabel
          classes={{ label: classes.label }}
          control={
            <Switch
              checked={busy}
              onChange={handleToggle}
              // Locked for the whole run, so the only way to stop is the stop
              // button, and a second run cannot be stacked on the first.
              disabled={saving || disabled || busy}
              color="primary"
            />
          }
          label={translate('resources.lufs.actions.toggle')}
        />
        {(saving || starting) && <CircularProgress size={16} />}
      </div>
    </Tooltip>
  )
}

export default LufsToggle
