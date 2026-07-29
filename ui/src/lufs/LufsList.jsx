import React, { useCallback, useMemo, useState } from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  SearchInput,
  SelectInput,
  TextField,
} from 'react-admin'
import {
  DurationField,
  List,
  OptimizeLufsButton,
  PathField,
  SizeField,
  useSelectedFields,
} from '../common'
import LufsListActions from './LufsListActions'
import { AnalyzeLufsButton } from './LufsAnalyzeButton'
import { RestoreOriginalButton } from './RestoreOriginalButton'
import { useAnalyzeStatus } from './useAnalyzeStatus'
import { useLibraryStatus } from './useLibraryStatus'
import {
  ActionField,
  ArtField,
  BitDepthPairField,
  BitratePairField,
  ChannelsPairField,
  CodecPairField,
  DurationDiffField,
  GainField,
  IntegrityField,
  LraField,
  LufsPairField,
  NullResidualField,
  ReportField,
  OriginalLufsField,
  SampleRatePairField,
  StatusField,
  TruePeakField,
  VerdictField,
} from './LufsFields'

// The verdict and phase dropdowns are always visible rather than hidden behind
// "Add filter": on a library of this size, narrowing to the songs that were
// altered - or that still need a decision - is the main thing anyone comes
// here to do.
const LufsFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
    <SelectInput
      source="loudness_verdict"
      label="Verdict"
      emptyText="-- Any verdict --"
      alwaysOn
      choices={[
        { id: 'untouched', name: 'No change needed - already on target' },
        { id: 'safe', name: 'Volume only - nothing else altered' },
        { id: 'dynamics_changed', name: 'Peaks trimmed' },
        { id: 'reencoded', name: 'Quality lost - format degraded' },
        { id: 'failed', name: 'Could not process' },
      ]}
    />
    <SelectInput
      source="loudness_phase"
      label="Still to do"
      emptyText="-- Any --"
      alwaysOn
      // String ids on purpose: a numeric 0 is falsy and can be dropped before
      // it reaches the query. SQLite compares it to the integer column fine.
      choices={[
        { id: '0', name: 'Nothing - in range' },
        { id: '1', name: 'Volume change - automatic' },
        { id: '3', name: 'Inaudible peak trim - automatic' },
        { id: '2', name: 'Needs your decision' },
      ]}
    />
    <SelectInput
      source="loudness_status"
      label="Status"
      emptyText="-- Any status --"
      choices={[
        { id: 'analyzed', name: 'Measured only - file not changed' },
        { id: 'processed', name: 'Changed' },
        { id: 'failed', name: 'Could not process' },
      ]}
    />
  </Filter>
)

// Optimizing a hand-picked selection is an explicit action, so it stays
// available whether or not "Optimise all LUFS" is on.
const LufsBulkActions = (props) => (
  <>
    <AnalyzeLufsButton
      {...props}
      mode="original"
      label="resources.lufs.actions.fetchOriginal"
    />
    <AnalyzeLufsButton {...props} />
    <OptimizeLufsButton {...props} />
    <RestoreOriginalButton {...props} />
  </>
)

// LufsList is the audit view: every property that loudness normalization must
// leave untouched, shown before -> after alongside the loudness measurements,
// so a change can be proved rather than assumed.
const LufsList = (props) => {
  const [settings, setSettings] = useState(null)
  const { status: analyzeStatus, poll } = useAnalyzeStatus()
  const { status: libraryStatus, poll: pollLibrary } = useLibraryStatus()

  const handleAnalyzeStarted = useCallback(() => {
    poll()
  }, [poll])

  // Turning the switch starts or stops the whole-library run server-side, so
  // pick up the new job state right away instead of waiting for the next poll.
  const handleOptimiseAllToggled = useCallback(() => {
    pollLibrary()
  }, [pollLibrary])

  const toggleableFields = useMemo(
    () => ({
      artist: <TextField source="artist" sortBy="artist" />,
      album: <TextField source="album" sortBy="album" />,
      status: (
        <StatusField source="status" label="Status" sortBy="loudness_status" />
      ),
      verdict: (
        <VerdictField
          source="verdict"
          label="Verdict"
          sortBy="loudness_verdict"
        />
      ),
      lufs: <LufsPairField source="lufs" label="LUFS" sortBy="lufs_before" />,
      gain: <GainField source="gain" label="Gain" sortBy="gain_applied" />,
      truePeak: (
        <TruePeakField source="truePeak" label="True Peak" sortBy="tp_before" />
      ),
      lra: <LraField source="lra" label="LRA" sortBy="lra_before" />,
      action: (
        <ActionField source="action" label="Mode" sortBy="loudness_action" />
      ),
      report: (
        <ReportField
          source="report"
          label="Report"
          settings={settings}
          sortable={false}
        />
      ),
      integrity: (
        <IntegrityField
          source="integrity"
          label="Audio integrity"
          sortable={false}
        />
      ),
      codec: <CodecPairField source="codec" label="Codec" sortBy="suffix" />,
      bitrate: (
        <BitratePairField source="bitrate" label="Bitrate" sortBy="bitRate" />
      ),
      sampleRate: (
        <SampleRatePairField
          source="sampleRate"
          label="Sample Rate"
          sortBy="sampleRate"
        />
      ),
      bitDepth: (
        <BitDepthPairField
          source="bitDepth"
          label="Bit Depth"
          sortBy="bitDepth"
        />
      ),
      channels: (
        <ChannelsPairField
          source="channels"
          label="Channels"
          sortBy="channels"
        />
      ),
      art: <ArtField source="art" label="Art" sortable={false} />,
      durationDiff: (
        <DurationDiffField
          source="durationDiff"
          label="Δ Duration"
          sortable={false}
        />
      ),
      nullResidual: (
        <NullResidualField
          source="nullResidual"
          label="Null test"
          sortBy="null_residual"
        />
      ),
      duration: <DurationField source="duration" sortBy="duration" />,
      size: <SizeField source="size" sortBy="size" />,
      path: <PathField source="path" sortBy="path" />,
      analyzedAt: (
        <FunctionField
          source="analyzedAt"
          label="Analyzed"
          sortBy="analyzed_at"
          render={(r) =>
            r?.loudnessAudit?.analyzedAt
              ? new Date(r.loudnessAudit.analyzedAt).toLocaleString()
              : '-'
          }
        />
      ),
    }),
    [settings],
  )

  const columns = useSelectedFields({
    resource: 'lufs',
    columns: toggleableFields,
    defaultOff: [
      'album',
      'action',
      'codec',
      'bitDepth',
      'channels',
      'durationDiff',
      'duration',
      'size',
      'path',
      'analyzedAt',
    ],
  })

  return (
    <List
      {...props}
      sort={{ field: 'title', order: 'ASC' }}
      actions={
        <LufsListActions
          onSettingsChange={setSettings}
          analyzeStatus={analyzeStatus}
          onAnalyzeStarted={handleAnalyzeStarted}
          libraryStatus={libraryStatus}
          onOptimiseAllToggled={handleOptimiseAllToggled}
        />
      }
      filters={<LufsFilter />}
      bulkActionButtons={<LufsBulkActions />}
      perPage={200}
    >
      <Datagrid rowClick={null}>
        <TextField source="title" sortBy="title" />
        {/* Fixed column: the original loudness is the reference every other
            number is judged against, so it is never hidden by the picker. */}
        <OriginalLufsField
          source="originalLufs"
          label="Original LUFS"
          sortBy="lufs_before"
        />
        {columns}
      </Datagrid>
    </List>
  )
}

export default LufsList
