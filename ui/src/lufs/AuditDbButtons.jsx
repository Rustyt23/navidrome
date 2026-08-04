import React, { useCallback, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button,
  Confirm,
  useNotify,
  usePermissions,
  useRefresh,
  useTranslate,
} from 'react-admin'
import SaveIcon from '@material-ui/icons/SaveAlt'
import SettingsBackupRestoreIcon from '@material-ui/icons/SettingsBackupRestore'
import { makeStyles, alpha } from '@material-ui/core/styles'
import { httpClient } from '../dataProvider'

const DB_URL = '/api/song/loudness/db'

const useStyles = makeStyles((theme) => ({
  danger: {
    color: theme.palette.error.main,
    '&:hover': {
      backgroundColor: alpha(theme.palette.error.main, 0.12),
    },
  },
}))

const when = (value) => {
  if (!value) return ''
  const at = new Date(value)
  return Number.isNaN(at.getTime()) ? '' : at.toLocaleString()
}

// SaveAuditDbButton copies the LUFS audit data to its own database file.
//
// This happens by itself whenever a job finishes; the button is for taking one
// on demand - before a risky change, or to check the copies are being written
// where the client expects them.
export const SaveAuditDbButton = ({ disabled }) => {
  const notify = useNotify()
  const { permissions } = usePermissions()
  const [saving, setSaving] = useState(false)

  const save = useCallback(() => {
    setSaving(true)
    httpClient(DB_URL, { method: 'POST' })
      .then(({ json }) => {
        notify('resources.lufs.notifications.auditDbSaved', {
          type: 'info',
          messageArgs: {
            rows: json?.snapshot?.rows ?? 0,
            file: json?.snapshot?.file || '',
          },
        })
      })
      .catch((error) =>
        notify(
          error?.body?.message ||
            error?.message ||
            'ra.notification.http_error',
          { type: 'warning' },
        ),
      )
      .finally(() => setSaving(false))
  }, [notify])

  if (permissions !== 'admin') {
    return null
  }

  return (
    <Button
      onClick={save}
      disabled={saving || disabled}
      label="resources.lufs.actions.saveAuditDb"
    >
      <SaveIcon />
    </Button>
  )
}

SaveAuditDbButton.propTypes = { disabled: PropTypes.bool }
SaveAuditDbButton.defaultProps = { disabled: false }

// RestoreAuditDbButton puts the most recent copy of the LUFS audit data back.
//
// It replaces exactly: the audit table ends up matching the copy, so anything
// analysed since it was taken is dropped. That is what makes it a restore
// rather than a merge, and it is why the copy is named and dated in the
// confirmation - the question "which point am I going back to" has to be
// answerable before the button does anything.
//
// Nothing outside the LUFS audit is touched. Songs, backups, playlists, play
// counts and ratings are not part of a copy and cannot be rolled back by one.
export const RestoreAuditDbButton = ({ disabled }) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const { permissions } = usePermissions()
  const [latest, setLatest] = useState(null)
  const [busy, setBusy] = useState(false)

  // The list is fetched on click rather than held, so the confirmation always
  // names the copy that is actually about to be restored.
  const load = useCallback(() => {
    setBusy(true)
    httpClient(DB_URL)
      .then(({ json }) => {
        const snapshots = json?.snapshots || []
        if (snapshots.length === 0) {
          notify('resources.lufs.notifications.noAuditDb', { type: 'warning' })
          return
        }
        setLatest({
          ...snapshots[0],
          folder: json?.folder,
          count: snapshots.length,
        })
      })
      .catch((error) =>
        notify(
          error?.body?.message ||
            error?.message ||
            'ra.notification.http_error',
          { type: 'warning' },
        ),
      )
      .finally(() => setBusy(false))
  }, [notify])

  const restore = useCallback(() => {
    setBusy(true)
    httpClient(`${DB_URL}/restore`, {
      method: 'POST',
      body: JSON.stringify({ file: latest?.file }),
    })
      .then(({ json }) => {
        setLatest(null)
        notify('resources.lufs.notifications.auditDbRestored', {
          type: json?.skipped ? 'warning' : 'info',
          messageArgs: {
            restored: json?.restored ?? 0,
            removed: json?.removed ?? 0,
            skipped: json?.skipped ?? 0,
          },
        })
        refresh()
      })
      .catch((error) =>
        notify(
          error?.body?.message ||
            error?.message ||
            'ra.notification.http_error',
          { type: 'warning' },
        ),
      )
      .finally(() => setBusy(false))
  }, [latest, notify, refresh])

  if (permissions !== 'admin') {
    return null
  }

  return (
    <>
      <Button
        className={classes.danger}
        onClick={load}
        disabled={busy || disabled}
        label="resources.lufs.actions.restoreAuditDb"
      >
        <SettingsBackupRestoreIcon />
      </Button>
      <Confirm
        isOpen={!!latest}
        loading={busy}
        title={translate('resources.lufs.restoreAuditDb.title')}
        content={
          latest
            ? translate('resources.lufs.restoreAuditDb.content', {
                file: latest.file,
                when: when(latest.createdAt),
                rows: latest.rows ?? 0,
              })
            : ''
        }
        onConfirm={restore}
        onClose={() => setLatest(null)}
      />
    </>
  )
}

RestoreAuditDbButton.propTypes = { disabled: PropTypes.bool }
RestoreAuditDbButton.defaultProps = { disabled: false }
