import React, { useCallback, useState } from 'react'
import {
  Button as RaButton,
  Confirm,
  useNotify,
  usePermissions,
  useTranslate,
} from 'react-admin'
import DeleteSweepIcon from '@material-ui/icons/DeleteSweep'
import { httpClient } from '../dataProvider'

const REPORT_URL = '/api/song/loudness/backups'
const CLEANUP_URL = '/api/song/loudness/backups/cleanup'

const mb = (bytes) => `${(Number(bytes || 0) / 1048576).toFixed(1)} MB`

// BackupCleanupButton finds stored originals that no longer protect anything.
//
// It reports before it does anything, and it never deletes: orphans are moved
// into a dated folder alongside them. A backup is the only copy of a song the
// client handed over, "orphaned" is decided by whether a matching song can be
// found right now, and that reads as false in several innocent situations - a
// library mid-reorganisation, or a drive that is not mounted. Nothing here is
// allowed to act on that answer by itself.
export const BackupCleanupButton = () => {
  const translate = useTranslate()
  const notify = useNotify()
  const { permissions } = usePermissions()
  const [report, setReport] = useState(null)
  const [busy, setBusy] = useState(false)

  const loadReport = useCallback(() => {
    setBusy(true)
    httpClient(REPORT_URL)
      .then(({ json }) => {
        if (!json?.orphans?.length) {
          notify(
            json?.warning ||
              translate('resources.lufs.notifications.noOrphanedBackups'),
            { type: json?.warning ? 'warning' : 'info' },
          )
          return
        }
        setReport(json)
      })
      .catch((error) =>
        notify(error?.message || 'ra.notification.http_error', {
          type: 'warning',
        }),
      )
      .finally(() => setBusy(false))
  }, [notify, translate])

  const confirmCleanup = useCallback(() => {
    setBusy(true)
    httpClient(CLEANUP_URL, { method: 'POST' })
      .then(({ json }) => {
        notify('resources.lufs.notifications.orphanedBackupsMoved', {
          type: 'info',
          messageArgs: { moved: json?.moved || 0, folder: json?.folder || '' },
        })
        setReport(null)
      })
      .catch((error) =>
        notify(
          error?.body?.warning ||
            error?.message ||
            'ra.notification.http_error',
          {
            type: 'warning',
          },
        ),
      )
      .finally(() => setBusy(false))
  }, [notify])

  if (permissions !== 'admin') {
    return null
  }

  const preview = report?.orphans
    ?.slice(0, 8)
    .map((o) => `${o.path}  (${mb(o.size)})`)
    .join('\n')
  const more = report && report.orphans.length > 8

  return (
    <>
      <RaButton
        onClick={loadReport}
        disabled={busy}
        label={translate('resources.lufs.actions.findOrphanedBackups')}
      >
        <DeleteSweepIcon />
      </RaButton>
      <Confirm
        isOpen={!!report}
        loading={busy}
        title={translate('resources.lufs.actions.findOrphanedBackups')}
        content={
          report
            ? translate('resources.lufs.confirmOrphanCleanup', {
                count: report.orphans.length,
                size: mb(report.orphanBytes),
                total: report.total,
                list:
                  preview +
                  (more ? `\n… and ${report.orphans.length - 8} more` : ''),
              })
            : ''
        }
        onConfirm={confirmCleanup}
        onClose={() => setReport(null)}
      />
    </>
  )
}

export default BackupCleanupButton
