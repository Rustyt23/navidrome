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

// LufsToggle switches whole-library optimisation on and off. Turning it on
// starts optimising every song; turning it off stops the run in progress and
// leaves optimisation to hand-picked selections. The state is persisted
// server-side, so it replaces editing Scanner.LoudnessNormalization in
// navidrome.toml and survives restarts.
export const LufsToggle = ({ onChange, onToggled, disabled }) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const [settings, setSettings] = useState(null)
  const [saving, setSaving] = useState(false)

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

  const handleToggle = useCallback(
    (event) => {
      const enabled = event.target.checked
      setSaving(true)
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
          notify(error?.message || 'ra.notification.http_error', 'warning')
        })
        .finally(() => setSaving(false))
    },
    [apply, notify, onToggled],
  )

  if (!settings) {
    return null
  }

  const hint = translate('resources.lufs.toggleHelper', {
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
              checked={!!settings.enabled}
              onChange={handleToggle}
              disabled={saving || disabled}
              color="primary"
            />
          }
          label={translate('resources.lufs.actions.toggle')}
        />
        {saving && <CircularProgress size={16} />}
      </div>
    </Tooltip>
  )
}

export default LufsToggle
