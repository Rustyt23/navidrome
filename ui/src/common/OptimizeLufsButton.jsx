import React, { useCallback, useState } from 'react'
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

  const handleClick = useCallback(async () => {
    if (!selectedCount || saving) {
      return
    }
    setSaving(true)
    try {
      const response = await dataProvider.optimizeSongLoudness(selectedIds)
      const normalized = response?.data?.normalized?.length || 0
      const skipped = response?.data?.skipped?.length || 0
      const failed = response?.data?.failed?.length || 0

      notify('resources.song.notifications.lufsOptimized', {
        type: failed > 0 ? 'warning' : 'info',
        messageArgs: { normalized, skipped, failed },
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
    <RaButton
      onClick={handleClick}
      className={clsx(classes.button, className)}
      label={translate('resources.song.actions.optimizeLufs')}
      disabled={!selectedCount || saving || disabled}
    >
      <EqualizerIcon />
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
