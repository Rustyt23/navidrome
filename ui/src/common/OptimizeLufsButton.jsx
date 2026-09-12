import React, { useCallback, useEffect, useRef, useState } from 'react'
import PropTypes from 'prop-types'
import clsx from 'clsx'
import {
  Button as RaButton,
  useDataProvider,
  useNotify,
  usePermissions,
  useRefresh,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import EqualizerIcon from '@material-ui/icons/Equalizer'
import { CircularProgress } from '@material-ui/core'
import { useJobStatus } from '../lufs/useJobStatus'
import { LIBRARY_URL } from '../lufs/useLibraryStatus'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

export const OptimizeLufsButton = ({
  resource,
  selectedIds,
  className,
  disabled,
}) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const dataProvider = useDataProvider()
  const { permissions } = usePermissions()
  const [saving, setSaving] = useState(false)

  const selectedCount = selectedIds?.length || 0
  const { follow } = useJobStatus(LIBRARY_URL)
  const stopFollowing = useRef(null)

  // Leaving the page stops the watching, not the run.
  useEffect(() => () => stopFollowing.current?.(), [])

  // The work now happens in a background run, so the request returns as soon as
  // it has started and the outcome has to be collected from the job afterwards.
  // On the LUFS page the progress bar and stop button appear on their own; on
  // the song and playlist pages this button's own spinner is the only sign, so
  // it stays busy for the whole run rather than only for the request.
  const handleClick = useCallback(async () => {
    if (!selectedCount || saving) {
      return
    }
    setSaving(true)
    try {
      await dataProvider.optimizeSongLoudness(selectedIds)
      notify('resources.song.notifications.lufsOptimiseStarted', {
        type: 'info',
        messageArgs: { count: selectedCount },
      })
      unselectAll(resource)

      stopFollowing.current = follow((json) => {
        stopFollowing.current = null
        setSaving(false)
        notify('resources.song.notifications.lufsOptimized', {
          type:
            json?.failed || json?.rejected || json?.error ? 'warning' : 'info',
          messageArgs: {
            normalized: json?.normalized || 0,
            skipped: json?.skipped || 0,
            rejected: json?.rejected || 0,
            failed: json?.failed || 0,
          },
        })
        refresh({ hard: true })
      })
    } catch (error) {
      setSaving(false)
      notify(error?.message || 'ra.notification.http_error', {
        type: 'warning',
      })
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
    <RaButton
      onClick={handleClick}
      className={clsx(classes.button, className)}
      label={translate('resources.song.actions.optimizeLufs')}
      disabled={!selectedCount || saving || disabled}
    >
      {saving ? <CircularProgress size={16} /> : <EqualizerIcon />}
    </RaButton>
  )
}

OptimizeLufsButton.propTypes = {
  resource: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  className: PropTypes.string,
  disabled: PropTypes.bool,
}

OptimizeLufsButton.defaultProps = {
  selectedIds: [],
  className: undefined,
  disabled: false,
}

export default OptimizeLufsButton
