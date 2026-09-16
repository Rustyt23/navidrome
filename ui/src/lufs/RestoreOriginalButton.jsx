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

// notifyRestoreResult reports a finished restore. A restore that was stopped
// says so, with how many songs it did not get to - "complete" would claim the
// whole selection was dealt with.
const notifyRestoreResult = (notify, final) => {
  const restored = final?.restored || 0
  const skipped = final?.skipped || 0
  const failed = final?.failed || 0
  const notRestored = Math.max(
    0,
    (final?.total || 0) - restored - skipped - failed,
  )
  const stopped = notRestored > 0 || !!final?.cancelled
  notify(
    stopped
      ? 'resources.song.notifications.lufsRestoreStopped'
      : 'resources.song.notifications.lufsRestored',
    {
      type: failed || stopped ? 'warning' : 'info',
      messageArgs: stopped
        ? { restored, skipped, failed, notRestored }
        : { restored, skipped, failed },
    },
  )
}

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
  const { status, follow, publish } = useRestoreStatus()
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
      const { data } = await dataProvider.restoreSongLoudness(selectedIds)
      // The progress bar shows "0 of N" from the reply itself.
      publish(data)
      // The selection is cleared as soon as the job owns the work: leaving it
      // highlighted invites a second press, which would only be refused.
      unselectAll(resource)

      follow((final) => {
        notifyRestoreResult(notify, final)
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
    publish,
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

// RestoreAllOriginalsButton puts back every stored original in the library,
// with no selection. The server picks the songs - every one with a stored
// original that is not already restored - and runs the same restore job, so
// progress and the stop button appear exactly as for a selection.
export const RestoreAllOriginalsButton = ({ disabled }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const dataProvider = useDataProvider()
  const { permissions } = usePermissions()
  const { status, follow, publish } = useRestoreStatus()
  const [confirming, setConfirming] = useState(false)
  const [saving, setSaving] = useState(false)

  const handleConfirm = useCallback(async () => {
    setConfirming(false)
    if (saving) {
      return
    }
    setSaving(true)
    try {
      const { data, started } = await dataProvider.restoreAllSongLoudness()
      if (!started) {
        notify(
          data?.message || 'resources.lufs.notifications.nothingToRestore',
          {
            type: 'info',
          },
        )
        return
      }
      publish(data)
      follow((final) => {
        notifyRestoreResult(notify, final)
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
  }, [dataProvider, follow, notify, publish, refresh, saving])

  if (permissions !== 'admin') {
    return null
  }

  return (
    <>
      <RaButton
        onClick={() => setConfirming(true)}
        label={translate('resources.lufs.actions.restoreAll')}
        disabled={saving || !!status?.running || disabled}
      >
        <RestoreIcon />
      </RaButton>
      <Confirm
        isOpen={confirming}
        loading={saving}
        title={translate('resources.lufs.restoreAll.title')}
        content={translate('resources.lufs.restoreAll.body')}
        confirm={translate('resources.lufs.actions.restoreAll')}
        onConfirm={handleConfirm}
        onClose={() => setConfirming(false)}
      />
    </>
  )
}

RestoreAllOriginalsButton.propTypes = {
  disabled: PropTypes.bool,
}

RestoreAllOriginalsButton.defaultProps = {
  disabled: false,
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
