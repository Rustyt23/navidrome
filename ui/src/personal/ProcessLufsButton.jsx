import { useCallback, useEffect, useState } from 'react'
import { useNotify, useTranslate } from 'react-admin'
import { Button, FormControl, FormHelperText } from '@material-ui/core'
import GraphicEqIcon from '@material-ui/icons/GraphicEq'
import { httpClient } from '../dataProvider'

// ProcessLufsButton triggers LUFS normalization for the entire library
// (admin only). Useful when automatic processing was skipped during scan
// because too many tracks were pending.
export const ProcessLufsButton = () => {
  const translate = useTranslate()
  const notify = useNotify()
  const [status, setStatus] = useState(null)

  const refreshStatus = useCallback(() => {
    httpClient('/api/song/loudness/library')
      .then(({ json }) => setStatus(json))
      .catch(() => {})
  }, [])

  useEffect(() => {
    refreshStatus()
  }, [refreshStatus])

  useEffect(() => {
    if (!status?.running) return
    const timer = setInterval(refreshStatus, 10000)
    return () => clearInterval(timer)
  }, [status?.running, refreshStatus])

  const start = () => {
    httpClient('/api/song/loudness/library', { method: 'POST' })
      .then(({ json }) => {
        notify(json?.message || 'LUFS processing started', 'info')
        refreshStatus()
      })
      .catch((error) => {
        const msg =
          error?.body?.message || translate('ra.page.error') || 'Error'
        notify(msg, 'warning')
        refreshStatus()
      })
  }

  const progress = status?.running
    ? ` (${status.processed || 0} processed, ${status.normalized || 0} normalized, ${status.skipped || 0} skipped, ${status.failed || 0} failed)`
    : ''

  return (
    <FormControl>
      <Button
        variant="outlined"
        color="primary"
        startIcon={<GraphicEqIcon />}
        disabled={!!status?.running}
        onClick={start}
      >
        {status?.running
          ? translate('menu.personal.options.processLufsRunning')
          : translate('menu.personal.options.processLufsLibrary')}
      </Button>
      <FormHelperText id="process-lufs-helper-text">
        {translate('menu.personal.options.processLufsHelper')}
        {progress}
      </FormHelperText>
    </FormControl>
  )
}
