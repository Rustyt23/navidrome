import React, { useCallback, useEffect, useRef, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button as RaButton,
  useNotify,
  useRefresh,
  useUnselectAll,
} from 'react-admin'
import CircularProgress from '@material-ui/core/CircularProgress'
import AssessmentIcon from '@material-ui/icons/Assessment'
import { httpClient } from '../dataProvider'

const ANALYZE_URL = '/api/song/silence/analyze'
const POLL_INTERVAL_MS = 1000

const AnalyzeSilenceButton = ({ selectedIds, all, resource }) => {
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const timerRef = useRef()
  const mountedRef = useRef(true)
  const [status, setStatus] = useState(null)
  const [starting, setStarting] = useState(false)
  const selectedCount = selectedIds?.length || 0

  const finish = useCallback(
    (finalStatus) => {
      if (!mountedRef.current) return
      setStatus(finalStatus)
      refresh({ hard: true })
      if (!all) unselectAll(resource)
      const failed = finalStatus?.failed || 0
      const completed = finalStatus?.processed || 0
      notify(
        failed
          ? `Silence analysis completed: ${completed} processed, ${failed} failed.`
          : `Silence analysis completed for ${completed} ${completed === 1 ? 'song' : 'songs'}.`,
        failed ? 'warning' : 'info',
      )
    },
    [all, notify, refresh, resource, unselectAll],
  )

  const poll = useCallback(async () => {
    try {
      const { json } = await httpClient(ANALYZE_URL)
      if (!mountedRef.current) return
      setStatus(json)
      if (json?.running) {
        timerRef.current = window.setTimeout(poll, POLL_INTERVAL_MS)
      } else {
        finish(json)
      }
    } catch (error) {
      if (!mountedRef.current) return
      setStarting(false)
      setStatus(null)
      notify(
        error?.message || 'Could not read silence analysis status.',
        'warning',
      )
    }
  }, [finish, notify])

  useEffect(
    () => () => {
      mountedRef.current = false
      window.clearTimeout(timerRef.current)
    },
    [],
  )

  useEffect(() => {
    httpClient(ANALYZE_URL)
      .then(({ json }) => {
        if (!mountedRef.current || !json?.running) return
        setStatus(json)
        timerRef.current = window.setTimeout(poll, POLL_INTERVAL_MS)
      })
      .catch(() => {})
  }, [poll])

  const handleClick = useCallback(async () => {
    if (starting || status?.running || (!all && !selectedCount)) return
    setStarting(true)
    try {
      const body = all ? { all: true } : { ids: selectedIds.map(String) }
      const { json } = await httpClient(ANALYZE_URL, {
        method: 'POST',
        body: JSON.stringify(body),
      })
      if (!mountedRef.current) return
      setStatus(json)
      notify(json?.message || 'Silence analysis started.', 'info')
      timerRef.current = window.setTimeout(poll, POLL_INTERVAL_MS)
    } catch (error) {
      if (!mountedRef.current) return
      const errorStatus = error?.body || null
      setStatus(errorStatus)
      if (errorStatus?.running) {
        timerRef.current = window.setTimeout(poll, POLL_INTERVAL_MS)
      }
      notify(
        error?.body?.message ||
          error?.message ||
          'Silence analysis could not start.',
        'warning',
      )
    } finally {
      if (mountedRef.current) setStarting(false)
    }
  }, [all, notify, poll, selectedCount, selectedIds, starting, status])

  const running = starting || status?.running
  const progress = status?.total
    ? `${status.processed || 0}/${status.total}`
    : ''
  const label = running
    ? `Analyzing${progress ? ` ${progress}` : '…'}`
    : all
      ? 'Analyze All Songs'
      : 'Analyze Silence'

  return (
    <RaButton
      label={label}
      onClick={handleClick}
      disabled={running || (!all && !selectedCount)}
    >
      {running ? <CircularProgress size={18} /> : <AssessmentIcon />}
    </RaButton>
  )
}

AnalyzeSilenceButton.propTypes = {
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  all: PropTypes.bool,
  resource: PropTypes.string,
}

AnalyzeSilenceButton.defaultProps = {
  selectedIds: [],
  all: false,
  resource: 'silenceTrim',
}

export default AnalyzeSilenceButton
