import React, { useCallback, useMemo, useState } from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  NullableBooleanInput,
  SearchInput,
  SelectArrayInput,
  TextField,
} from 'react-admin'
import { List, PathField, SizeField, useSelectedFields } from '../common'
import SilenceListActions from './SilenceListActions'
import SilenceSummary from './SilenceSummary'
import { SilenceBulkActions } from './SilenceButtons'
import {
  useSilenceAnalyzeStatus,
  useSilenceTrimStatus,
} from './useSilenceStatus'
import {
  DurationChangeField,
  GaplessField,
  OnsetField,
  RemovedPeakField,
  SilenceFoundField,
  SilenceMethodField,
  SilenceStatusField,
  SilenceVerdictField,
  SizeChangeField,
  TotalTrimField,
  TrimBreakdownField,
} from './SilenceFields'

// Both dropdowns are always visible rather than tucked behind "Add filter":
// narrowing to the songs that will actually be cut is the main thing anyone
// comes here to do, and it should not take two clicks to reach.
//
// The choices are exactly what the columns can display, worded the same way. A
// filter offering an outcome the column never shows sends people hunting for
// rows that cannot exist.
const SilenceFilter = (props) => (
  <Filter {...props} variant="outlined">
    <SearchInput source="title" alwaysOn />
    <SelectArrayInput
      source="silence_verdict"
      label="Result"
      style={{ minWidth: 220 }}
      alwaysOn
      choices={[
        { id: 'trimmable', name: 'Ready to trim' },
        { id: 'clean', name: 'Nothing to remove' },
        { id: 'skipped', name: 'Left alone' },
        { id: 'failed', name: 'Could not measure' },
      ]}
    />
    {/* The working set: analysed as trimmable and not yet cut. This is what
        the client selects before pressing Trim, so it is one click away. */}
    <NullableBooleanInput
      source="silence_pending"
      label="Waiting to be trimmed"
      alwaysOn
    />
    <SelectArrayInput
      source="silence_reason"
      label="Why left alone"
      style={{ minWidth: 220 }}
      choices={[
        { id: 'audible', name: 'Something audible in it' },
        { id: 'too_long', name: 'Too much to be dead air' },
        { id: 'gapless', name: 'Album plays continuously' },
        { id: 'too_short', name: 'Less than the margin' },
        { id: 'fade', name: 'Measurement unreliable' },
      ]}
    />
    <SelectArrayInput
      source="silence_method"
      label="How it is cut"
      choices={[
        { id: 'copy', name: 'Lossless (bit-exact)' },
        { id: 'encode', name: 'Re-encoded' },
      ]}
    />
  </Filter>
)

// SilenceList is the silence-trim page: what each song has at its head and
// tail, how much would come off, and why anything is being left alone.
//
// Entirely separate from the LUFS pages. It shares the media files and nothing
// else - different table, different endpoints, different engine.
const SilenceList = (props) => {
  const [summary, setSummary] = useState(null)
  const { status: analyzeStatus, poll: pollAnalyze } = useSilenceAnalyzeStatus()
  const { status: trimStatus, poll: pollTrim } = useSilenceTrimStatus()

  const handleAnalyzeStarted = useCallback(() => pollAnalyze(), [pollAnalyze])
  const handleTrimStarted = useCallback(() => pollTrim(), [pollTrim])

  const toggleableFields = useMemo(
    () => ({
      artist: <TextField source="artist" sortBy="artist" />,
      album: <TextField source="album" sortBy="album" />,
      silenceFound: (
        <SilenceFoundField
          source="silenceFound"
          label="Silence found"
          sortBy="lead_silence"
        />
      ),
      trimBreakdown: (
        <TrimBreakdownField
          source="trimBreakdown"
          label="Start / end"
          sortBy="lead_trim"
        />
      ),
      verdict: (
        <SilenceVerdictField
          source="verdict"
          label="Result"
          sortBy="silence_verdict"
        />
      ),
      status: (
        <SilenceStatusField source="status" label="State" sortBy="silence_trimmed" />
      ),
      method: (
        <SilenceMethodField source="method" label="Method" sortBy="silence_method" />
      ),
      durationChange: (
        <DurationChangeField
          source="durationChange"
          label="Length"
          sortBy="silence_duration"
        />
      ),
      removedPeak: (
        <RemovedPeakField
          source="removedPeak"
          label="Removed audio level"
          sortable={false}
        />
      ),
      onset: <OnsetField source="onset" label="Onset" sortable={false} />,
      gapless: <GaplessField source="gapless" label="Album" sortable={false} />,
      sizeSaved: (
        <SizeChangeField
          source="sizeSaved"
          label="Space saved"
          sortBy="silence_size_diff"
        />
      ),
      size: <SizeField source="size" sortBy="size" />,
      path: <PathField source="path" sortBy="path" />,
      analyzedAt: (
        <FunctionField
          source="analyzedAt"
          label="Analysed"
          sortBy="silence_analyzed"
          render={(r) =>
            r?.silenceAudit?.analyzedAt
              ? new Date(r.silenceAudit.analyzedAt).toLocaleString()
              : '-'
          }
        />
      ),
    }),
    [],
  )

  const columns = useSelectedFields({
    resource: 'silence',
    columns: toggleableFields,
    defaultOff: ['album', 'onset', 'size', 'path', 'analyzedAt', 'sizeSaved'],
  })

  return (
    <>
      {/* Recounted whenever a job ends, so the headline follows the work
          instead of going stale behind it. */}
      <SilenceSummary
        refreshKey={`${analyzeStatus?.running}-${trimStatus?.running}`}
        onLoaded={setSummary}
      />
      <List
        {...props}
        sort={{ field: 'total_trim', order: 'DESC' }}
        actions={
          <SilenceListActions
            analyzeStatus={analyzeStatus}
            trimStatus={trimStatus}
            summary={summary}
            onAnalyzeStarted={handleAnalyzeStarted}
            onTrimStarted={handleTrimStarted}
          />
        }
        filters={<SilenceFilter />}
        bulkActionButtons={<SilenceBulkActions />}
        perPage={200}
      >
        <Datagrid rowClick={null}>
          <TextField source="title" sortBy="title" />
          {/* Fixed column: how many seconds come off this song is the question
              the page exists to answer, so the picker can never hide it. The
              default sort puts the biggest at the top. */}
          <TotalTrimField source="totalTrim" label="To trim" sortBy="total_trim" />
          {columns}
        </Datagrid>
      </List>
    </>
  )
}

export default SilenceList
