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
  OriginalLufsField,
  SampleRatePairField,
  StatusField,
  TruePeakField,
  VerdictField,
} from './LufsFields'

const LufsFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
    <SelectInput
      source="loudness_verdict"
      label="Verdict"
      emptyText="-- All --"
      choices={[
        { id: 'safe', name: 'Safe' },
        { id: 'untouched', name: 'Untouched' },
        { id: 'dynamics_changed', name: 'Dynamics changed' },
        { id: 'reencoded', name: 'Re-encoded' },
        { id: 'failed', name: 'Failed' },
      ]}
    />
    <SelectInput
      source="loudness_status"
      label="Status"
      emptyText="-- All --"
      choices={[
        { id: 'analyzed', name: 'Analyzed' },
        { id: 'processed', name: 'Processed' },
        { id: 'failed', name: 'Failed' },
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
  </>
)

// LufsList is the audit view: every property that loudness normalization must
// leave untouched, shown before -> after alongside the loudness measurements,
// so a change can be proved rather than assumed.
const LufsList = (props) => {
  const [, setSettings] = useState(null)
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
    [],
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
      perPage={50}
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
