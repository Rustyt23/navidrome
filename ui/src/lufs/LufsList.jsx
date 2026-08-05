import React, { useCallback, useMemo, useState } from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  SearchInput,
  SelectArrayInput,
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
import LufsSummary from './LufsSummary'
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
  DecisionMarkField,
  DurationDiffField,
  GainField,
  IntegrityField,
  LraField,
  OutcomeField,
  LufsPairField,
  NullResidualField,
  ReportField,
  OriginalLufsField,
  SampleRatePairField,
  StatusField,
  TruePeakField,
  VerdictField,
} from './LufsFields'

// Both dropdowns are always visible rather than hidden behind "Add filter": on
// a library of this size, narrowing to the songs that were altered is the main
// thing anyone comes here to do. Both take several values at once, because the
// useful questions are usually plural - "show me everything that was touched"
// is two verdicts, not one.
//
// The choices are exactly what the Verdict and Status columns can display, and
// worded exactly as the chips word them. A filter that offers an outcome the
// column never shows sends people looking for rows that cannot exist; one that
// omits an outcome the column does show leaves rows visibly there and
// impossible to narrow to. "Left as-is" is the second of those - it is derived
// from the action rather than stored as a verdict, and was missing entirely.
const LufsFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
    <SelectArrayInput
      source="loudness_verdict"
      label="Verdict"
      style={{ minWidth: 240 }}
      alwaysOn
      choices={[
        { id: 'untouched', name: 'No change needed' },
        { id: 'safe', name: 'Volume only' },
        { id: 'dynamics_changed', name: 'Levelled + peaks capped' },
        { id: 'rewrite_costly', name: 'Differs from original' },
        { id: 'reencoded', name: 'Quality lost' },
        { id: 'left_as_is', name: 'Left as-is' },
        { id: 'failed', name: 'Could not process' },
      ]}
    />
    <SelectArrayInput
      source="loudness_outcome"
      label="Outcome"
      alwaysOn
      choices={[
        { id: 'on_target', name: 'On target' },
        { id: 'short', name: 'Short of target' },
        { id: 'not_measured', name: 'Not measured' },
      ]}
    />
    <SelectArrayInput
      source="loudness_status"
      label="Status"
      alwaysOn
      choices={[
        { id: 'analyzed', name: 'Measured only' },
        { id: 'processed', name: 'Changed' },
        { id: 'failed', name: 'Could not process' },
      ]}
    />
    {/* The songs that went through the exceptions page. Not alwaysOn: it
        answers a question about a handful of songs out of hundreds, so it
        earns a place in the filter menu rather than a permanent seat. */}
    <SelectArrayInput
      source="loudness_decision"
      label="Chosen by hand"
      style={{ minWidth: 200 }}
      choices={[
        { id: 'limit', name: 'Limit to target' },
        { id: 'gain_ceiling', name: 'Gain to ceiling' },
        { id: 'skip', name: 'Leave alone' },
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
          settings={settings}
          source="verdict"
          label="Verdict"
          sortBy="loudness_verdict"
        />
      ),
      lufs: <LufsPairField source="lufs" label="LUFS" sortBy="lufs_before" />,
      // Which songs came off the exceptions page. Sortable, so they group
      // together rather than having to be hunted for.
      decision: (
        <DecisionMarkField
          source="decision"
          label="Chosen by hand"
          sortBy="loudness_decision"
        />
      ),
      gain: <GainField source="gain" label="Gain" sortBy="gain_applied" />,
      truePeak: (
        <TruePeakField source="truePeak" label="True Peak" sortBy="tp_before" />
      ),
      lra: <LraField source="lra" label="LRA" sortBy="lra_before" />,
      outcome: (
        <OutcomeField
          source="outcome"
          label="Outcome"
          settings={settings}
          sortable={false}
        />
      ),
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
    <>
      {/* Recounted whenever a job ends, so the headline follows the work
          instead of going stale behind it. */}
      <LufsSummary
        refreshKey={`${libraryStatus?.running}-${analyzeStatus?.running}`}
      />
      <List
        {...props}
        sort={{ field: 'title', order: 'ASC' }}
        actions={
          <LufsListActions
            settings={settings}
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
    </>
  )
}

export default LufsList
