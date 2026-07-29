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
      const response = await dataProvider.restoreSongLoudness(selectedIds)
      const restored = response?.data?.restored?.length || 0
      const skipped = response?.data?.skipped?.length || 0
      const failed = response?.data?.failed?.length || 0

      notify('resources.song.notifications.lufsRestored', {
        type: failed > 0 ? 'warning' : 'info',
        messageArgs: { restored, skipped, failed },
      })

      unselectAll(resource)
      refresh({ hard: true })
    } catch (error) {
      notify(error?.message || 'ra.notification.http_error', {
        type: 'warning',
      })
    } finally {
      setSaving(false)
    }
  }, [
    dataProvider,
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
        disabled={!selectedCount || saving || disabled}
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
