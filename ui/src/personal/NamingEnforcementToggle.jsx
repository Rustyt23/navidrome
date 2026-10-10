import React, { useCallback, useEffect, useState } from 'react'
import { useNotify, useTranslate } from 'react-admin'
import { useDispatch, useSelector } from 'react-redux'
import {
  CircularProgress,
  FormControl,
  FormControlLabel,
  Switch,
  Tooltip,
} from '@material-ui/core'
import { setNamingEnforcement } from '../actions'
import { httpClient } from '../dataProvider'

const SETTINGS_URL = '/api/song/naming/settings'

// NamingEnforcementToggle controls whether tracks not named
// "Artist - Title.mp3" are held back from the storage bucket.
//
// Unlike the icon toggles beside it, this is not a display preference: the
// server persists it and enforces it during sync. Turning it off lets
// badly-named tracks upload again, where no playlist entry can resolve them -
// the bucket is flat and playlists reference tracks by bare filename. So the
// off state is the one that needs explaining, not the on state.
export const NamingEnforcementToggle = () => {
  const translate = useTranslate()
  const notify = useNotify()
  const dispatch = useDispatch()
  const enforced = useSelector(
    (state) => state.settings?.namingEnforcement !== false,
  )
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    httpClient(SETTINGS_URL)
      .then(({ json }) => {
        if (!cancelled && typeof json?.enforced === 'boolean') {
          dispatch(setNamingEnforcement(json.enforced))
        }
      })
      .catch(() => {
        // Leave the store at its last value; the toggle still works.
      })
    return () => {
      cancelled = true
    }
  }, [dispatch])

  const handleToggle = useCallback(
    (event) => {
      const next = event.target.checked
      setSaving(true)
      httpClient(SETTINGS_URL, {
        method: 'PUT',
        body: JSON.stringify({ enforced: next }),
      })
        .then(({ json }) => {
          dispatch(setNamingEnforcement(json?.enforced ?? next))
          notify(
            next
              ? 'notifications.namingIssuesEnforcementOn'
              : 'notifications.namingIssuesEnforcementOff',
            next ? 'info' : 'warning',
          )
        })
        .catch((error) => {
          notify(error?.message || 'ra.notification.http_error', 'warning')
        })
        .finally(() => setSaving(false))
    },
    [dispatch, notify],
  )

  return (
    <Tooltip
      title={translate(
        enforced
          ? 'menu.personal.options.enforceNamingOnHelper'
          : 'menu.personal.options.enforceNamingOffHelper',
      )}
    >
      <FormControl>
        <FormControlLabel
          control={
            <Switch
              id="namingEnforcementToggle"
              color="primary"
              checked={enforced}
              disabled={saving}
              onChange={handleToggle}
            />
          }
          label={
            <>
              {translate('menu.personal.options.enforceNaming')}
              {saving && (
                <CircularProgress size={12} style={{ marginLeft: 8 }} />
              )}
            </>
          }
        />
      </FormControl>
    </Tooltip>
  )
}

export default NamingEnforcementToggle
