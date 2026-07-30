import React, { useCallback, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import { IconButton, Tooltip } from '@material-ui/core'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import {
  BooleanInput,
  Button,
  Datagrid,
  ExportButton,
  Filter,
  FunctionField,
  NumberField,
  SearchInput,
  SelectInput,
  TextField,
  TopToolbar,
  useListContext,
  useNotify,
  useRecordContext,
} from 'react-admin'
import StopIcon from '@material-ui/icons/Stop'
import { useDispatch } from 'react-redux'
import { playTracks } from '../actions'
import {
  BitrateField,
  DurationField,
  List,
  PathField,
  ToggleFieldsMenu,
  useSelectedFields,
} from '../common'
import { useSelectedLibraries } from '../common/useLibrarySelection'
import JobProgress from '../lufs/JobProgress'
import { httpClient } from '../dataProvider'
import {
  AnalyzeSilenceButton,
  ApplySilenceButton,
  RestoreSilenceButton,
  SetSilenceDecisionButton,
} from './SilenceTrimButtons'
import {
  ClassificationField,
  DecisionField,
  DurationAfterField,
  EdgeField,
  IntegrityField,
  MethodField,
  PaddingField,
  ProposalField,
  ReasonField,
  StatusField,
} from './SilenceTrimFields'
import {
  SILENCE_ANALYZE_URL,
  SILENCE_APPLY_URL,
  useSilenceAnalyzeStatus,
  useSilenceApplyStatus,
} from './useSilenceTrimStatus'

const SilenceTrimFilter = (props) => (
  <Filter {...props} variant="outlined">
    <SearchInput source="title" alwaysOn />
    <SelectInput
      source="silence_trim_status"
      label="Status"
      emptyText="-- All --"
      choices={[
        { id: 'analyzed', name: 'Dry run only' },
        { id: 'processed', name: 'Trimmed' },
        { id: 'failed', name: 'Could not verify' },
      ]}
    />
    <SelectInput
      source="silence_trim_classification"
      label="Safety"
      emptyText="-- All --"
      choices={[
        { id: 'safe', name: 'Safe to trim' },
        { id: 'review', name: 'Review first' },
        { id: 'blocked', name: 'Protected' },
        { id: 'none', name: 'No trim needed' },
      ]}
    />
    <SelectInput
      source="silence_trim_decision"
      label="Decision"
      emptyText="-- All --"
      choices={[
        { id: 'approve', name: 'Approved' },
        { id: 'skip', name: 'Leave unchanged' },
      ]}
    />
    <BooleanInput source="silence_trim_removable" label="Has edge blank" />
  </Filter>
)

const StopJobButton = ({ url, label, disabled, onStopped }) => {
  const notify = useNotify()
  const handleClick = useCallback(() => {
    httpClient(`${url}/stop`, { method: 'POST' })
      .then(({ json }) => {
        notify(json?.message || 'Stopping…', 'info')
        onStopped?.()
      })
      .catch((error) =>
        notify(error?.body?.message || error?.message, 'warning'),
      )
  }, [notify, onStopped, url])
  return (
    <Button label={label} onClick={handleClick} disabled={disabled}>
      <StopIcon />
    </Button>
  )
}

const SilenceTrimActions = ({
  analyzeStatus,
  applyStatus,
  analyzeStarting,
  applyStarting,
  onAnalyzeStarted,
  onApplyStarted,
  onStatusChanged,
  libraryIds,
  ...rest
}) => {
  const { total } = useListContext()
  const busy =
    analyzeStarting ||
    applyStarting ||
    analyzeStatus?.running ||
    applyStatus?.running
  return (
    <TopToolbar {...rest}>
      <JobProgress
        label="Analyzing edges"
        status={
          analyzeStatus?.running
            ? analyzeStatus
            : analyzeStarting
              ? { running: true, processed: 0, total: 0 }
              : null
        }
        detail={
          analyzeStatus?.running
            ? `${analyzeStatus.safe || 0} safe · ${
                analyzeStatus.review || 0
              } review · ${analyzeStatus.blocked || 0} protected`
            : undefined
        }
      />
      <JobProgress
        label="Trimming edges"
        status={
          applyStatus?.running
            ? applyStatus
            : applyStarting
              ? { running: true, processed: 0, total: 0 }
              : null
        }
        detail={
          applyStatus?.running
            ? `${applyStatus.trimmed || 0} trimmed · ${
                applyStatus.skipped || 0
              } skipped · ${applyStatus.failed || 0} failed`
            : undefined
        }
      />
      {analyzeStatus?.running ? (
        <StopJobButton
          url={SILENCE_ANALYZE_URL}
          label="Stop analysis"
          onStopped={onStatusChanged}
        />
      ) : (
        <AnalyzeSilenceButton
          all
          libraryIds={libraryIds}
          disabled={busy}
          onStarted={onAnalyzeStarted}
        />
      )}
      {applyStatus?.running ? (
        <StopJobButton
          url={SILENCE_APPLY_URL}
          label="Stop trimming"
          onStopped={onStatusChanged}
        />
      ) : (
        <ApplySilenceButton
          all
          libraryIds={libraryIds}
          resource="silencetrim"
          disabled={busy}
          onStarted={onApplyStarted}
        />
      )}
      <ExportButton maxResults={total} />
      <ToggleFieldsMenu resource="silencetrim" />
    </TopToolbar>
  )
}

const BulkActions = ({ onAnalyzeStarted, onApplyStarted, ...props }) => (
  <>
    <AnalyzeSilenceButton {...props} onStarted={onAnalyzeStarted} />
    <SetSilenceDecisionButton {...props} decision="approve" />
    <SetSilenceDecisionButton {...props} decision="skip" />
    <ApplySilenceButton {...props} onStarted={onApplyStarted} />
    <RestoreSilenceButton {...props} />
  </>
)

BulkActions.propTypes = {
  onAnalyzeStarted: PropTypes.func,
  onApplyStarted: PropTypes.func,
}

const SilenceReviewPlayField = (props) => {
  const record = useRecordContext(props)
  const dispatch = useDispatch()
  if (!record) return null

  const label = `Play ${record.title || 'song'} for start/end silence review`
  return (
    <Tooltip title={label}>
      <span>
        <IconButton
          size="small"
          aria-label={label}
          disabled={record.missing}
          onClick={(event) => {
            event.stopPropagation()
            event.preventDefault()
            dispatch(playTracks({ [record.id]: record }))
          }}
        >
          <PlayArrowIcon fontSize="small" />
        </IconButton>
      </span>
    </Tooltip>
  )
}

SilenceReviewPlayField.propTypes = {
  record: PropTypes.object,
}

SilenceReviewPlayField.defaultProps = {
  addLabel: true,
}

const SilenceTrimList = (props) => {
  const [analyzeStarting, setAnalyzeStarting] = useState(false)
  const [applyStarting, setApplyStarting] = useState(false)
  const notify = useNotify()
  const {
    status: analyzeStatus,
    poll: pollAnalyze,
    watch: watchAnalyze,
  } = useSilenceAnalyzeStatus()
  const {
    status: applyStatus,
    poll: pollApply,
    watch: watchApply,
  } = useSilenceApplyStatus()
  const selectedLibraries = useSelectedLibraries()
  const libraryIds = useMemo(
    () =>
      selectedLibraries
        .map(Number)
        .filter((libraryId) => Number.isInteger(libraryId) && libraryId > 0),
    [selectedLibraries],
  )
  const libraryFilter = useMemo(
    () => ({ library_id: libraryIds.length > 0 ? libraryIds : [-1] }),
    [libraryIds],
  )

  const onAnalyzeStarted = useCallback(() => {
    setAnalyzeStarting(true)
    watchAnalyze((final) => {
      setAnalyzeStarting(false)
      notify(
        `Analysis finished: ${final?.safe || 0} safe, ${
          final?.review || 0
        } review, ${final?.blocked || 0} protected, ${
          final?.failed || 0
        } failed`,
        final?.failed ? 'warning' : 'info',
      )
    })
  }, [notify, watchAnalyze])

  const onApplyStarted = useCallback(() => {
    setApplyStarting(true)
    watchApply((final) => {
      setApplyStarting(false)
      notify(
        `Trimming finished: ${final?.trimmed || 0} changed, ${
          final?.skipped || 0
        } skipped, ${final?.failed || 0} failed`,
        final?.failed ? 'warning' : 'info',
      )
    })
  }, [notify, watchApply])

  const toggleableFields = useMemo(
    () => ({
      artist: <TextField source="artist" sortBy="artist" />,
      album: <TextField source="album" sortBy="album" />,
      format: (
        <FunctionField
          source="suffix"
          label="Format"
          render={(record) => record?.suffix?.toUpperCase() || '—'}
          sortBy="suffix"
        />
      ),
      bitrate: (
        <BitrateField source="bitRate" label="Bitrate" sortBy="bitRate" />
      ),
      sampleRate: (
        <FunctionField
          source="sampleRate"
          label="Sample rate"
          render={(record) =>
            record?.sampleRate ? `${record.sampleRate} Hz` : '—'
          }
          sortBy="sampleRate"
        />
      ),
      bitDepth: (
        <FunctionField
          source="bitDepth"
          label="Bit depth"
          render={(record) =>
            record?.bitDepth ? `${record.bitDepth}-bit` : '—'
          }
          sortBy="bitDepth"
        />
      ),
      channels: <NumberField source="channels" label="Channels" />,
      duration: <DurationField source="duration" label="Current duration" />,
      status: (
        <StatusField
          source="silenceTrimAudit.status"
          label="Status"
          sortBy="silence_trim_status"
        />
      ),
      safety: (
        <ClassificationField
          source="silenceTrimAudit.classification"
          label="Safety"
          sortBy="silence_trim_classification"
        />
      ),
      leading: (
        <EdgeField
          edge="leading"
          source="silenceTrimAudit.leadingSilence"
          label="Blank at start"
          sortBy="silence_trim_leading"
        />
      ),
      trailing: (
        <EdgeField
          edge="trailing"
          source="silenceTrimAudit.trailingSilence"
          label="Blank at end"
          sortBy="silence_trim_trailing"
        />
      ),
      startTrim: (
        <ProposalField
          edge="leading"
          source="silenceTrimAudit.proposedStartTrim"
          label="Proposed start cut"
          sortable={false}
        />
      ),
      endTrim: (
        <ProposalField
          edge="trailing"
          source="silenceTrimAudit.proposedEndTrim"
          label="Proposed end cut"
          sortable={false}
        />
      ),
      padding: (
        <PaddingField
          source="silenceTrimAudit.retainedPadding"
          label="Blank retained"
          sortable={false}
        />
      ),
      method: (
        <MethodField
          source="silenceTrimAudit.method"
          label="Quality-safe method"
          sortBy="silence_trim_method"
        />
      ),
      expectedDuration: (
        <DurationAfterField
          source="silenceTrimAudit.durationAfter"
          label="After / expected"
          sortable={false}
        />
      ),
      integrity: (
        <IntegrityField
          source="silenceTrimAudit.integrity"
          label="Integrity proof"
          sortable={false}
        />
      ),
      decision: (
        <DecisionField
          source="silenceTrimAudit.decision"
          label="Decision"
          sortBy="silence_trim_decision"
        />
      ),
      reason: (
        <ReasonField
          source="silenceTrimAudit.reason"
          label="Why / protection"
          sortable={false}
        />
      ),
      backup: (
        <FunctionField
          source="silenceTrimAudit.hasBackup"
          label="Pre-trim backup"
          render={(record) =>
            record?.silenceTrimAudit?.hasBackup ? 'Available' : '—'
          }
          sortable={false}
        />
      ),
      analyzedAt: (
        <FunctionField
          source="silenceTrimAudit.analyzedAt"
          label="Analyzed"
          render={(record) =>
            record?.silenceTrimAudit?.analyzedAt
              ? new Date(record.silenceTrimAudit.analyzedAt).toLocaleString()
              : '—'
          }
          sortBy="silence_trim_analyzed_at"
        />
      ),
      appliedAt: (
        <FunctionField
          source="silenceTrimAudit.appliedAt"
          label="Applied"
          render={(record) =>
            record?.silenceTrimAudit?.appliedAt
              ? new Date(record.silenceTrimAudit.appliedAt).toLocaleString()
              : '—'
          }
          sortable={false}
        />
      ),
      path: <PathField source="path" sortBy="path" />,
    }),
    [],
  )

  const columns = useSelectedFields({
    resource: 'silencetrim',
    columns: toggleableFields,
    defaultOff: [
      'album',
      'sampleRate',
      'bitDepth',
      'channels',
      'duration',
      'expectedDuration',
      'decision',
      'backup',
      'analyzedAt',
      'appliedAt',
      'path',
    ],
  })

  const busy =
    analyzeStarting ||
    applyStarting ||
    analyzeStatus?.running ||
    applyStatus?.running

  return (
    <List
      {...props}
      sort={{ field: 'title', order: 'ASC' }}
      filter={libraryFilter}
      filters={<SilenceTrimFilter />}
      actions={
        <SilenceTrimActions
          analyzeStatus={analyzeStatus}
          applyStatus={applyStatus}
          analyzeStarting={analyzeStarting}
          applyStarting={applyStarting}
          onAnalyzeStarted={onAnalyzeStarted}
          onApplyStarted={onApplyStarted}
          libraryIds={libraryIds}
          onStatusChanged={() => {
            pollAnalyze()
            pollApply()
          }}
        />
      }
      bulkActionButtons={
        <BulkActions
          disabled={busy}
          onAnalyzeStarted={onAnalyzeStarted}
          onApplyStarted={onApplyStarted}
        />
      }
      perPage={200}
    >
      <Datagrid rowClick={null}>
        <SilenceReviewPlayField label="Listen" sortable={false} />
        <TextField source="title" sortBy="title" />
        {columns}
      </Datagrid>
    </List>
  )
}

export default SilenceTrimList
