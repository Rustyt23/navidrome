import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button as RaButton,
  Confirm,
  useListContext,
  useNotify,
  useRefresh,
  useUnselectAll,
} from 'react-admin'
import CircularProgress from '@material-ui/core/CircularProgress'
import Button from '@material-ui/core/Button'
import Dialog from '@material-ui/core/Dialog'
import DialogActions from '@material-ui/core/DialogActions'
import DialogContent from '@material-ui/core/DialogContent'
import DialogTitle from '@material-ui/core/DialogTitle'
import RestoreIcon from '@material-ui/icons/Restore'
import TransformIcon from '@material-ui/icons/Transform'
import VerifiedUserIcon from '@material-ui/icons/VerifiedUser'
import { httpClient } from '../dataProvider'
import { getSilenceTrimMargin } from './silenceTrimSettings'

const POLL_INTERVAL_MS = 1000
const RETRY_INTERVAL_MS = 2500
const CURRENT_THRESHOLD_DB = -60
const CURRENT_MINIMUM_SILENCE = 0.005
const endpoint = (mode) => `/api/song/silence/${mode}`

const canTrim = (record) => {
  const analysis = record?.silenceAnalysis
  const backupStatus = record?.silenceBackup?.status
  const suffix = String(record?.suffix || '').toLowerCase()
  const hasRemovableEdge =
    Number(analysis?.leadingSilence || 0) > CURRENT_MINIMUM_SILENCE ||
    Number(analysis?.trailingSilence || 0) > CURRENT_MINIMUM_SILENCE
  return (
    suffix === 'mp3' &&
    analysis?.status === 'analyzed' &&
    Number(analysis?.thresholdDb) === CURRENT_THRESHOLD_DB &&
    Number(analysis?.minimumSilence) === CURRENT_MINIMUM_SILENCE &&
    hasRemovableEdge &&
    backupStatus !== 'available' &&
    backupStatus !== 'prepared'
  )
}

const canRestore = (record) =>
  ['available', 'prepared'].includes(record?.silenceBackup?.status)

const IntegrityButton = ({ selectedIds: selectedIdsProp }) => {
  const { selectedIds: contextSelectedIds = [], data = {} } = useListContext()
  const selectedIds = selectedIdsProp?.length
    ? selectedIdsProp
    : contextSelectedIds
  const notify = useNotify()
  const refresh = useRefresh()
  const [checking, setChecking] = useState(false)
  const [reports, setReports] = useState([])
  const [reportOpen, setReportOpen] = useState(false)
  const candidates = (selectedIds || [])
    .map((id) => data?.[id] || data?.[String(id)])
    .filter((record) => record?.silenceBackup)

  const verify = useCallback(async () => {
    if (checking || !candidates.length) return
    setChecking(true)
    try {
      const reports = await Promise.all(
        candidates.map(async (record) => {
          const response = await httpClient(
            `/api/song/silence/integrity?id=${encodeURIComponent(record.id)}`,
          )
          return response.json
        }),
      )
      const failed = reports.filter((report) => report?.status === 'failed')
      const verified = reports.filter((report) => report?.status === 'verified')
      setReports(reports)
      setReportOpen(true)
      notify(
        `Integrity checked: ${verified.length} verified, ${reports.length - verified.length - failed.length} restored, ${failed.length} failed.${failed[0]?.error ? ` ${failed[0].error}` : ''}`,
        failed.length ? 'warning' : 'info',
      )
      refresh({ hard: true })
    } catch (error) {
      notify(error?.message || 'Could not verify song integrity.', 'warning')
    } finally {
      setChecking(false)
    }
  }, [candidates, checking, notify, refresh])

  return (
    <>
      <RaButton
        label={checking ? 'Verifying Integrity…' : 'Verify Integrity'}
        onClick={verify}
        disabled={checking || !candidates.length}
      >
        {checking ? <CircularProgress size={18} /> : <VerifiedUserIcon />}
      </RaButton>
      <Dialog
        open={reportOpen}
        onClose={() => setReportOpen(false)}
        fullWidth
        maxWidth="md"
      >
        <DialogTitle>Silence Trim Integrity Report</DialogTitle>
        <DialogContent>
          {reports.map((report) => (
            <div key={report.mediaFileId} style={{ marginBottom: 18 }}>
              <strong>{report.title || report.mediaFileId}</strong>
              <div>{report.artist}</div>
              <div>Status: {report.status}</div>
              <div>Original SHA-256: {report.originalSha256}</div>
              <div>Trimmed SHA-256: {report.trimmedSha256 || '—'}</div>
              <div>Current SHA-256: {report.currentSha256 || '—'}</div>
              <div>
                Checks: backup {report.checks?.backupSha256 ? '✓' : '✗'}, decode{' '}
                {report.checks?.decode ? '✓' : '✗'}, audio properties{' '}
                {report.checks?.audioProperties ? '✓' : '✗'}, metadata{' '}
                {report.checks?.metadata ? '✓' : '✗'}, artwork{' '}
                {report.checks?.artwork ? '✓' : '✗'}, audio packets{' '}
                {report.checks?.audioPackets ? '✓' : '✗'}
              </div>
              {report.error && <div>Error: {report.error}</div>}
            </div>
          ))}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setReportOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}

IntegrityButton.propTypes = {
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
}

const SilenceMutationButtons = ({ selectedIds: selectedIdsProp, resource }) => {
  const { selectedIds: contextSelectedIds = [], data = {} } = useListContext()
  const selectedIds = selectedIdsProp?.length
    ? selectedIdsProp
    : contextSelectedIds
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const timerRef = useRef()
  const mountedRef = useRef(true)
  const pollErrorNotifiedRef = useRef(false)
  const [confirming, setConfirming] = useState(null)
  const [starting, setStarting] = useState(false)
  const [status, setStatus] = useState(null)
  const selectedCount = selectedIds?.length || 0
  const eligible = useMemo(() => {
    const records = (selectedIds || [])
      .map((id) => data?.[id] || data?.[String(id)])
      .filter(Boolean)
    return {
      trim: records.filter(canTrim).map((record) => String(record.id)),
      restore: records.filter(canRestore).map((record) => String(record.id)),
    }
  }, [data, selectedIds])

  const finish = useCallback(
    (finalStatus) => {
      if (!mountedRef.current) return
      setStatus(finalStatus)
      refresh({ hard: true })
      unselectAll(resource)
      const mode = finalStatus?.mode || 'operation'
      const firstError = finalStatus?.errors?.[0]
      notify(
        `Silence ${mode} finished: ${finalStatus?.succeeded || 0} changed, ${finalStatus?.skipped || 0} skipped, ${finalStatus?.failed || 0} failed.${firstError ? ` ${firstError}` : ''}`,
        finalStatus?.failed ? 'warning' : 'info',
      )
    },
    [notify, refresh, resource, unselectAll],
  )

  const poll = useCallback(
    async (mode) => {
      try {
        const { json } = await httpClient(endpoint(mode))
        if (!mountedRef.current) return
        pollErrorNotifiedRef.current = false
        setStatus(json)
        if (json?.running) {
          timerRef.current = window.setTimeout(
            () => poll(json?.mode || mode),
            POLL_INTERVAL_MS,
          )
        } else {
          finish(json)
        }
      } catch (error) {
        if (!mountedRef.current) return
        setStarting(false)
        if (!pollErrorNotifiedRef.current) {
          pollErrorNotifiedRef.current = true
          notify(
            `${error?.message || `Could not read silence ${mode} status.`} Retrying…`,
            'warning',
          )
        }
        timerRef.current = window.setTimeout(
          () => poll(mode),
          RETRY_INTERVAL_MS,
        )
      }
    },
    [finish, notify],
  )

  useEffect(
    () => () => {
      mountedRef.current = false
      window.clearTimeout(timerRef.current)
    },
    [],
  )

  useEffect(() => {
    httpClient(endpoint('trim'))
      .then(({ json }) => {
        if (!mountedRef.current || !json?.running) return
        setStatus(json)
        timerRef.current = window.setTimeout(
          () => poll(json?.mode || 'trim'),
          POLL_INTERVAL_MS,
        )
      })
      .catch(() => {})
  }, [poll])

  const start = useCallback(async () => {
    const mode = confirming
    setConfirming(null)
    const ids = eligible[mode] || []
    if (!mode || !ids.length || starting || status?.running) return
    setStarting(true)
    try {
      const { json } = await httpClient(endpoint(mode), {
        method: 'POST',
        body: JSON.stringify({
          ids,
          marginSeconds: getSilenceTrimMargin(),
        }),
      })
      if (!mountedRef.current) return
      setStatus(json)
      notify(
        json?.message ||
          `Silence ${mode} started with a ${getSilenceTrimMargin().toFixed(2)} s margin.`,
        mode === 'trim' ? 'warning' : 'info',
      )
      timerRef.current = window.setTimeout(
        () => poll(json?.mode || mode),
        POLL_INTERVAL_MS,
      )
    } catch (error) {
      if (!mountedRef.current) return
      const errorStatus = error?.body || null
      setStatus(errorStatus)
      if (errorStatus?.running && errorStatus?.mode) {
        timerRef.current = window.setTimeout(
          () => poll(errorStatus.mode),
          POLL_INTERVAL_MS,
        )
      }
      notify(
        errorStatus?.message ||
          error?.message ||
          `Silence ${mode} could not start.`,
        'warning',
      )
    } finally {
      if (mountedRef.current) setStarting(false)
    }
  }, [confirming, eligible, notify, poll, starting, status])

  const running = starting || status?.running
  const activeMode = status?.mode
  const progress = status?.total
    ? `${status.processed || 0}/${status.total}`
    : ''
  const trimLabel =
    running && activeMode === 'trim'
      ? `Trimming${progress ? ` ${progress}` : '…'}`
      : 'Trim Silence'
  const restoreLabel =
    running && activeMode === 'restore'
      ? `Restoring${progress ? ` ${progress}` : '…'}`
      : 'Restore Original'
  const trimCount = eligible.trim.length
  const restoreCount = eligible.restore.length
  const ignoredTrimCount = Math.max(0, selectedCount - trimCount)
  const ignoredRestoreCount = Math.max(0, selectedCount - restoreCount)

  return (
    <>
      <IntegrityButton selectedIds={selectedIds} />
      <RaButton
        label={trimLabel}
        onClick={() => setConfirming('trim')}
        disabled={running || !trimCount}
      >
        {running && activeMode === 'trim' ? (
          <CircularProgress size={18} />
        ) : (
          <TransformIcon />
        )}
      </RaButton>
      <RaButton
        label={restoreLabel}
        onClick={() => setConfirming('restore')}
        disabled={running || !restoreCount}
      >
        {running && activeMode === 'restore' ? (
          <CircularProgress size={18} />
        ) : (
          <RestoreIcon />
        )}
      </RaButton>
      <Confirm
        isOpen={confirming === 'trim'}
        loading={starting}
        title="Trim selected silence?"
        content={`This will create exact originals for ${trimCount} eligible ${trimCount === 1 ? 'song' : 'songs'} in “silence trim backup”, then trim MP3 audio without re-encoding while preserving tags and embedded artwork. A ${getSilenceTrimMargin().toFixed(2)} second margin is retained at each detected edge.${ignoredTrimCount ? ` ${ignoredTrimCount} selected ${ignoredTrimCount === 1 ? 'song is' : 'songs are'} not analyzed, unsupported, already trimmed, or have no removable edge and will not be submitted.` : ''}`}
        onConfirm={start}
        onClose={() => setConfirming(null)}
      />
      <Confirm
        isOpen={confirming === 'restore'}
        loading={starting}
        title="Restore selected originals?"
        content={`Restore ${restoreCount} eligible ${restoreCount === 1 ? 'original' : 'originals'} from “silence trim backup”? Files changed externally after trimming will not be overwritten, and backups are retained.${ignoredRestoreCount ? ` ${ignoredRestoreCount} selected ${ignoredRestoreCount === 1 ? 'song has' : 'songs have'} no restorable backup and will not be submitted.` : ''}`}
        onConfirm={start}
        onClose={() => setConfirming(null)}
      />
    </>
  )
}

SilenceMutationButtons.propTypes = {
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ),
  resource: PropTypes.string,
}

SilenceMutationButtons.defaultProps = {
  selectedIds: [],
  resource: 'silenceTrim',
}

export default SilenceMutationButtons
