import React, { useState, useMemo } from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  List,
  NumberField,
  sanitizeListRestProps,
  SearchInput,
  TextField,
  TopToolbar,
  useNotify,
} from 'react-admin'
import Button from '@material-ui/core/Button'
import CircularProgress from '@material-ui/core/CircularProgress'
import Dialog from '@material-ui/core/Dialog'
import DialogActions from '@material-ui/core/DialogActions'
import DialogContent from '@material-ui/core/DialogContent'
import DialogTitle from '@material-ui/core/DialogTitle'
import config from '../config'
import { httpClient } from '../dataProvider'
import {
  DurationField,
  Pagination,
  PathField,
  QualityInfo,
  ToggleFieldsMenu,
  useSelectedFields,
  SizeField,
} from '../common'
import AnalyzeSilenceButton from './AnalyzeSilenceButton'
import SilenceMutationButtons from './SilenceMutationButtons'
import SilenceTrimMarginSelect from './SilenceTrimMarginSelect'

const CURRENT_THRESHOLD_DB = -60
const CURRENT_MINIMUM_SILENCE = 0.005

const SilenceTrimFilter = (props) => (
  <Filter {...props} variant="outlined">
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const SilenceTrimListActions = (props) => (
  <TopToolbar {...sanitizeListRestProps(props)}>
    <SilenceTrimMarginSelect />
    <AnalyzeSilenceButton all />
    <ToggleFieldsMenu resource="silenceTrim" />
  </TopToolbar>
)

// A custom bulk action keeps row selection available without exposing
// react-admin's default destructive bulk delete.
const SilenceTrimBulkActions = (props) => (
  <>
    <AnalyzeSilenceButton {...props} />
    <SilenceMutationButtons {...props} />
  </>
)

const formatSilence = (record, edge) => {
  const analysis = record?.silenceAnalysis
  if (analysis?.status === 'failed') return 'Failed'
  if (
    analysis?.status === 'analyzed' &&
    (Number(analysis.thresholdDb) !== CURRENT_THRESHOLD_DB ||
      Number(analysis.minimumSilence) !== CURRENT_MINIMUM_SILENCE)
  ) {
    return 'Re-analyze'
  }
  const seconds = analysis?.[edge]
  if (seconds === null || seconds === undefined) return 'Not analyzed'
  if (Number(seconds) === 0) return 'N/D'
  return `${Number(seconds).toFixed(3)} s`
}

const renderSilence = (record, edge) => {
  const analysis = record?.silenceAnalysis
  const seconds = analysis?.[edge]
  const measured =
    analysis?.status === 'analyzed' && Number(seconds) > 0
  return (
    <span
      title="Silence below -60 dB lasting at least 5 ms"
      style={measured ? { color: 'var(--accent, #FF2B8A)' } : undefined}
    >
      {formatSilence(record, edge)}
    </span>
  )
}

const formatBackup = (record) => {
  const backup = record?.silenceBackup
  if (!backup) return 'No backup'
  if (backup.status === 'available') {
    return <span style={{ color: 'var(--accent, #FF2B8A)' }}>Original available</span>
  }
  if (backup.status === 'prepared') return 'Recovery available'
  if (backup.status === 'restored') return 'Restored (kept)'
  return 'Backup ready'
}

const IntegrityReportDialog = ({ report, onClose }) => (
  <Dialog open={Boolean(report)} onClose={onClose} fullWidth maxWidth="md">
    <DialogTitle>Silence Trim Integrity Report</DialogTitle>
    <DialogContent>
      {report && (
        <div>
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
            {report.checks?.audioPackets ? '✓' : '✗'}, restore{' '}
            {report.checks?.restoreSha256 ? '✓' : '✗'}
          </div>
          {report.error && <div>Error: {report.error}</div>}
        </div>
      )}
    </DialogContent>
    <DialogActions>
      <Button onClick={onClose}>Close</Button>
    </DialogActions>
  </Dialog>
)

const IntegrityStatusField = ({ record }) => {
  const notify = useNotify()
  const [report, setReport] = useState(null)
  const [loading, setLoading] = useState(false)
  const backup = record?.silenceBackup
  if (!backup?.integrityStatus) return 'Not verified'

  const status = backup.integrityStatus
  const label =
    status === 'verified'
      ? 'Verified'
      : status === 'restored' && backup.restoreVerified
        ? 'Restored exactly'
        : status === 'prepared'
          ? 'Validation prepared'
          : status === 'failed'
            ? 'Failed'
            : 'Pending'
  const verified = status === 'verified' || status === 'restored'

  const openReport = async (event) => {
    event.stopPropagation()
    setLoading(true)
    try {
      const response = await httpClient(
        `/api/song/silence/integrity?id=${encodeURIComponent(record.id)}`,
      )
      setReport(response.json)
    } catch (error) {
      notify(error?.message || 'Could not load the integrity report.', 'warning')
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <button
        type="button"
        onClick={openReport}
        disabled={loading}
        title="Open the integrity audit report"
        style={{
          border: 0,
          padding: 0,
          background: 'transparent',
          color: verified ? 'var(--accent, #FF2B8A)' : undefined,
          cursor: loading ? 'wait' : 'pointer',
          font: 'inherit',
          textAlign: 'left',
        }}
      >
        {loading ? <CircularProgress size={16} /> : label}
      </button>
      <IntegrityReportDialog report={report} onClose={() => setReport(null)} />
    </>
  )
}

const formatVerificationStatus = (record) => {
  const status = record?.silenceBackup?.integrityStatus
  if (!status) return 'Not checked'
  const labels = {
    verified: 'Passed',
    restored: 'Restored',
    prepared: 'Pending',
    failed: 'Failed',
    pending: 'Pending',
  }
  const label = labels[status] || status
  const color =
    status === 'verified' || status === 'restored'
      ? 'var(--accent, #FF2B8A)'
      : status === 'failed'
        ? '#f44336'
        : undefined
  return <span style={color ? { color } : undefined}>{label}</span>
}

const SilenceTrimList = (props) => {
  const toggleableFields = useMemo(
    () => ({
      artist: <TextField source="artist" label="Artist" />,
      album: <TextField source="album" label="Album" />,
      startSilence: (
        <FunctionField
          source="silenceAnalysis.leadingSilence"
          label="Start Silence"
          sortable={false}
          render={(record) => renderSilence(record, 'leadingSilence')}
        />
      ),
      endSilence: (
        <FunctionField
          source="silenceAnalysis.trailingSilence"
          label="End Silence"
          sortable={false}
          render={(record) => renderSilence(record, 'trailingSilence')}
        />
      ),
      afterTrimStart: (
        <FunctionField
          source="silenceAnalysis.leadingSilenceAfter"
          label="After Trim Start Silence"
          sortable={false}
          render={(record) => renderSilence(record, 'leadingSilenceAfter')}
        />
      ),
      afterTrimEnd: (
        <FunctionField
          source="silenceAnalysis.trailingSilenceAfter"
          label="After Trim End Silence"
          sortable={false}
          render={(record) => renderSilence(record, 'trailingSilenceAfter')}
        />
      ),
      backup: (
        <FunctionField
          source="silenceBackup.status"
          label="Backup"
          sortable={false}
          render={formatBackup}
        />
      ),
      integrity: (
        <FunctionField
          source="silenceBackup.integrityStatus"
          label="Integrity"
          sortable={false}
          render={(record) => <IntegrityStatusField record={record} />}
        />
      ),
      verificationStatus: (
        <FunctionField
          source="silenceBackup.integrityStatus"
          label="Verification Status"
          sortable={false}
          render={formatVerificationStatus}
        />
      ),
      format: <QualityInfo source="quality" label="Format" sortable={false} />,
      sampleRate: <NumberField source="sampleRate" label="Sample Rate" />,
      bitDepth: <NumberField source="bitDepth" label="Bit Depth" />,
      channels: <NumberField source="channels" label="Channels" />,
      duration: <DurationField source="duration" label="Duration" />,
      size: <SizeField source="size" label="Size" />,
      path: <PathField source="path" label="Path" sortable={false} />,
    }),
    [],
  )
  const columns = useSelectedFields({
    resource: 'silenceTrim',
    columns: toggleableFields,
  })

  return (
    <List
      {...props}
      title="Silence Trim"
      sort={{ field: 'title', order: 'ASC' }}
      filter={{ missing: false }}
      filters={<SilenceTrimFilter />}
      actions={<SilenceTrimListActions />}
      exporter={false}
      perPage={500}
      pagination={<Pagination />}
      debounce={config.uiSearchDebounceMs}
      bulkActionButtons={<SilenceTrimBulkActions />}
    >
      <Datagrid rowClick={null}>
        <TextField source="title" label="Title" />
        {columns}
      </Datagrid>
    </List>
  )
}

export default SilenceTrimList
