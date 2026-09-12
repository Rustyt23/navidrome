import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Button as RaButton,
  Datagrid,
  ExportButton,
  Filter,
  SearchInput,
  SelectInput,
  TextField,
  TopToolbar,
  useListContext,
  useNotify,
  useTranslate,
} from 'react-admin'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import { CircularProgress } from '@material-ui/core'
import {
  BitrateField,
  DownloadSongsButton,
  List,
  PathField,
  ToggleFieldsMenu,
  useSelectedFields,
} from '../common'
import { httpClient } from '../dataProvider'
import { useLibraryStatus } from '../lufs/useLibraryStatus'
import { useAnalyzeStatus } from '../lufs/useAnalyzeStatus'
import RecheckButton from './RecheckButton'
import { RestoreOriginalButton } from '../lufs/RestoreOriginalButton'
import JobProgress from '../lufs/JobProgress'
import {
  GainField,
  LraField,
  OriginalLufsField,
  StatusField,
  TruePeakField,
  VerdictField,
} from '../lufs/LufsFields'
import SetDecisionButton from './DecisionButtons'
import {
  DECISION_CEILING,
  DECISION_LIMIT,
  DECISION_SKIP,
} from './recommendation'
import { REASON_CHOICES } from './reason'
import {
  BestWithoutDistortionField,
  CurrentField,
  DecisionField,
  HeadroomField,
  OptionCeilingField,
  OptionLimitField,
  ReasonField,
  SuggestionField,
} from './Lufs2Fields'

const SETTINGS_URL = '/api/song/loudness/settings'
const RUN_URL = '/api/song/loudness/library?phase=2'

// Where the column layout is remembered. Versioned, because saved visibility
// wins over defaultOff for any column the browser already knows about - that is
// deliberate, so adding a column never wipes someone's layout, but it also
// means a change to the default layout reaches nobody who has opened the page
// before. Bumping this retires the old layout once. Bump it again only for
// another deliberate change to the defaults, never for adding a column.
const COLUMNS_KEY = 'lufs2.v2'

const Lufs2Filter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
    {/* Alongside the clickable headline in each row rather than instead of it:
        the headline is how you get here from a song you are looking at, this is
        how you get here when you already know which cause you want. Both set
        the same server-side filter, so a selection made after either covers the
        whole library and not just the page. */}
    <SelectInput
      source="loudness_reason"
      label="Why it is here"
      emptyText="-- All --"
      choices={REASON_CHOICES}
      alwaysOn
    />
    <SelectInput
      source="loudness_decision"
      label="Decision"
      emptyText="-- All --"
      choices={[
        { id: DECISION_LIMIT, name: 'Limit to target' },
        { id: DECISION_CEILING, name: 'Gain to ceiling' },
        { id: DECISION_SKIP, name: 'Leave alone' },
      ]}
    />
  </Filter>
)

// A song that a run built a file for and then refused is left alone by every
// later run, so the same rejected file is not rebuilt for ever. That mark is
// only as current as the settings it was made under: change the ceiling, the
// tolerance, or the file itself, and the refusal may no longer hold. Checking
// again measures the song and retries it, so it either comes back with a
// current reason or turns out to be fixed.
const BulkActions = (props) => (
  <>
    <SetDecisionButton {...props} decision={DECISION_CEILING} />
    <SetDecisionButton {...props} decision={DECISION_LIMIT} />
    <SetDecisionButton {...props} decision={DECISION_SKIP} />
    <RecheckButton {...props} />
    <RestoreOriginalButton {...props} />
    <DownloadSongsButton {...props} />
  </>
)

// The click has to be acknowledged before the server has been asked anything.
// Waiting for the first status poll leaves the button looking untouched for up
// to a second, which reads as "nothing happened" and invites a second click.
const RunPhase2Button = ({ status, starting, onStarting, onStarted }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const busy = !!status?.running || starting

  const handleClick = useCallback(() => {
    onStarting?.()
    httpClient(RUN_URL, { method: 'POST' })
      .then(({ json }) => {
        notify(json?.message || 'Applying decisions…', 'info')
        onStarted?.(true)
      })
      .catch((error) => {
        notify(
          error?.body?.message ||
            error?.message ||
            'ra.notification.http_error',
          'warning',
        )
        onStarted?.(false)
      })
  }, [notify, onStarting, onStarted])

  return (
    <RaButton
      onClick={handleClick}
      disabled={busy}
      label={translate('resources.lufs2.actions.runPhase2')}
    >
      {busy ? <CircularProgress size={16} /> : <PlayArrowIcon />}
    </RaButton>
  )
}

const Lufs2Actions = ({
  status,
  analyzeStatus,
  starting,
  onStarting,
  onStarted,
  ...rest
}) => {
  const { total } = useListContext()
  // Shown from the click itself, not from the first status that comes back:
  // a run over a few tracks can be finished before the server is next asked.
  const progress = status?.running
    ? status
    : starting
      ? { running: true, processed: 0, total: 0 }
      : null
  return (
    <TopToolbar {...rest}>
      <JobProgress
        label="Applying decisions"
        status={progress}
        detail={
          status?.running
            ? `${status.normalized || 0} changed · ${status.failed || 0} failed`
            : starting
              ? 'starting…'
              : undefined
        }
      />
      <JobProgress label="Measuring" status={analyzeStatus} />
      <RunPhase2Button
        status={status}
        starting={starting}
        onStarting={onStarting}
        onStarted={onStarted}
      />
      <ExportButton maxResults={total} />
      <ToggleFieldsMenu resource={COLUMNS_KEY} />
    </TopToolbar>
  )
}

// Lufs2List is the standing record of every song whose handling was not
// routine: one needing its peaks cut by enough to be audible, one a run built a
// file for and then refused, one whose peaks were trimmed, or one a person has
// already decided about.
//
// A song stays once it qualifies rather than dropping off the moment it is
// dealt with. This is the list someone reviews, answers to a client from, and
// restores out of, and none of that works if a song vanishes the instant it is
// processed. Trims small enough to be inaudible are still applied automatically
// and never appear here at all - keeping the page to the songs that genuinely
// needed a person is what stops it becoming something nobody reads.
const Lufs2List = (props) => {
  const [settings, setSettings] = useState(null)
  const [starting, setStarting] = useState(false)
  const notify = useNotify()

  // A run over a handful of decisions can be over before the next poll, so
  // without this the only sign it happened is the rows quietly changing.
  const report = useCallback(
    (final) => {
      setStarting(false)
      notify('resources.lufs2.notifications.runFinished', {
        type:
          final?.failed || final?.error || final?.cancelled
            ? 'warning'
            : 'info',
        messageArgs: {
          changed: final?.normalized || 0,
          skipped: final?.skipped || 0,
          failed: final?.failed || 0,
          cancelled: final?.cancelled || 0,
          remaining: Math.max(
            0,
            (final?.total || 0) -
              (final?.processed || 0) -
              (final?.cancelled || 0),
          ),
        },
      })
    },
    [notify],
  )
  const { status, follow } = useLibraryStatus()
  const stopFollowing = useRef(null)
  useEffect(() => () => stopFollowing.current?.(), [])
  const { status: analyzeStatus } = useAnalyzeStatus()

  useEffect(() => {
    let active = true
    httpClient(SETTINGS_URL)
      .then(({ json }) => {
        if (active) setSettings(json)
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [])

  // Clear the local flag as soon as the server confirms a run, so the progress
  // bar hands over from "starting" to real counts.
  useEffect(() => {
    if (status?.running) setStarting(false)
  }, [status?.running])

  // Every column is optional except the title - a row has to be identifiable.
  // The order here is the default layout; the picker remembers whatever you
  // change it to.
  const toggleableFields = useMemo(
    () => ({
      artist: <TextField source="artist" sortBy="artist" />,
      bitRate: (
        <BitrateField source="bitRate" label="Bitrate" sortBy="bitRate" />
      ),
      current: (
        <CurrentField
          source="current"
          label="Now"
          settings={settings}
          sortBy="lufs_before"
        />
      ),
      reason: (
        <ReasonField
          source="reason"
          label="Why it is here"
          settings={settings}
          sortable={false}
        />
      ),
      headroom: (
        <HeadroomField
          source="headroom"
          label="Headroom short by"
          settings={settings}
          sortable={false}
        />
      ),
      optionCeiling: (
        <OptionCeilingField
          source="optionCeiling"
          label="Option A · keep audio intact"
          settings={settings}
          sortable={false}
        />
      ),
      optionLimit: (
        <OptionLimitField
          source="optionLimit"
          label="Option B · hit the target"
          settings={settings}
          sortable={false}
        />
      ),
      best: (
        <BestWithoutDistortionField
          source="best"
          label="Best without distortion"
          settings={settings}
          sortable={false}
        />
      ),
      suggested: (
        <SuggestionField
          source="suggested"
          label="Suggested"
          settings={settings}
          sortable={false}
        />
      ),
      // Named for what someone comes to it for. "Decision" answered "what did
      // I choose", which they already knew - the question the page could not
      // answer was "and what did that do to the song".
      decision: (
        <DecisionField
          source="decision"
          label="Final result"
          settings={settings}
          sortBy="loudness_decision"
        />
      ),
      originalLufs: (
        <OriginalLufsField
          source="originalLufs"
          label="Original LUFS"
          sortBy="lufs_before"
        />
      ),
      truePeak: (
        <TruePeakField source="truePeak" label="True Peak" sortBy="tp_before" />
      ),
      lra: <LraField source="lra" label="LRA" sortBy="lra_before" />,
      gain: <GainField source="gain" label="Gain" sortBy="gain_applied" />,
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
      path: <PathField source="path" sortBy="path" />,
    }),
    [settings],
  )

  const columns = useSelectedFields({
    resource: COLUMNS_KEY,
    columns: toggleableFields,
    // Six columns on by default: what the song is, where it sits now, why it
    // is here, what we propose, and the decision. Everything else is working.
    //
    // Headroom, the two options and the best volume-only result are four
    // columns of the same arithmetic - they answer "how far over is the peak"
    // four times over. On a page of eight rows that reads as a spreadsheet
    // demanding to be reconciled rather than as eight questions, and every one
    // of those numbers is now in the Suggested column or its tooltip. They stay
    // one click away in the column picker for anyone checking the working.
    defaultOff: [
      'bitRate',
      'headroom',
      'optionCeiling',
      'optionLimit',
      'best',
      'originalLufs',
      'truePeak',
      'lra',
      'gain',
      'status',
      'verdict',
      'path',
    ],
  })

  const handleStarting = useCallback(() => setStarting(true), [])
  const handleStarted = useCallback(
    (ok) => {
      if (!ok) {
        setStarting(false)
        return
      }
      stopFollowing.current?.()
      stopFollowing.current = follow(report)
    },
    [follow, report],
  )

  return (
    <List
      {...props}
      sort={{ field: 'title', order: 'ASC' }}
      filter={{ loudness_exception: true }}
      filters={<Lufs2Filter />}
      actions={
        <Lufs2Actions
          status={status}
          analyzeStatus={analyzeStatus}
          starting={starting}
          onStarting={handleStarting}
          onStarted={handleStarted}
        />
      }
      bulkActionButtons={<BulkActions />}
      perPage={50}
    >
      <Datagrid rowClick={null}>
        <TextField source="title" sortBy="title" />
        {columns}
      </Datagrid>
    </List>
  )
}

export default Lufs2List
