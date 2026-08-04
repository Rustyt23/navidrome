import React, { useCallback, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button as RaButton,
  Confirm,
  useDataProvider,
  useNotify,
  usePermissions,
  useRefresh,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import RestoreIcon from '@material-ui/icons/Restore'
import { useRestoreStatus } from './useRestoreStatus'

// RestoreOriginalButton puts the client's untouched originals back.
//
// This is what the backups exist for. Nothing is re-encoded: the file stored
// before the song was rewritten is copied straight back, so the result is the
// original bit for bit rather than an attempt to reverse the processing.
//
// It overwrites audio in the library, so it asks first. Songs that were never
// rewritten have no stored original and are reported as skipped, which keeps
// selecting a whole page from looking like a failure.
export const RestoreOriginalButton = ({ resource, selectedIds, disabled }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const dataProvider = useDataProvider()
  const { permissions } = usePermissions()
  // The server hands the work to a background job and answers immediately, so
  // the outcome arrives by following that job rather than by waiting on the
  // request. A big selection used to hold the request open for minutes with
  // nothing to show, which reads as a hung page.
  const { status, follow } = useRestoreStatus()
  const [confirming, setConfirming] = useState(false)
  const [saving, setSaving] = useState(false)

  const selectedCount = selectedIds?.length || 0

  const handleConfirm = useCallback(async () => {
    setConfirming(false)
    if (!selectedCount || saving) {
      return
    }
    setSaving(true)
    try {
      await dataProvider.restoreSongLoudness(selectedIds)
      // The selection is cleared as soon as the job owns the work: leaving it
      // highlighted invites a second press, which would only be refused.
      unselectAll(resource)

      follow((final) => {
        notify('resources.song.notifications.lufsRestored', {
          type: final?.failed ? 'warning' : 'info',
          messageArgs: {
            restored: final?.restored || 0,
            skipped: final?.skipped || 0,
            failed: final?.failed || 0,
          },
        })
        refresh({ hard: true })
      })
    } catch (error) {
      notify(
        error?.body?.message || error?.message || 'ra.notification.http_error',
        { type: 'warning' },
      )
    } finally {
      setSaving(false)
    }
  }, [
    dataProvider,
    follow,
    notify,
    refresh,
    resource,
    saving,
    selectedCount,
    selectedIds,
    unselectAll,
  ])

  if (permissions !== 'admin') {
    return null
  }

  return (
    <>
      <RaButton
        onClick={() => setConfirming(true)}
        label={translate('resources.lufs.actions.restore')}
        disabled={!selectedCount || saving || !!status?.running || disabled}
      >
        <RestoreIcon />
      </RaButton>
      <Confirm
        isOpen={confirming}
        loading={saving}
        title={translate('resources.lufs.actions.restore')}
        content={translate('resources.lufs.confirmRestore', {
          smart_count: selectedCount,
        })}
        onConfirm={handleConfirm}
        onClose={() => setConfirming(false)}
      />
    </>
  )
}

RestoreOriginalButton.propTypes = {
  resource: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  disabled: PropTypes.bool,
}

RestoreOriginalButton.defaultProps = {
  selectedIds: [],
  disabled: false,
}

export default RestoreOriginalButton
