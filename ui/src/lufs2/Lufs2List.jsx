import React, { useCallback, useEffect, useState } from 'react'
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
import { List, PathField } from '../common'
import { httpClient } from '../dataProvider'
import { useLibraryStatus } from '../lufs/useLibraryStatus'
import JobProgress from '../lufs/JobProgress'
import SetDecisionButton from './DecisionButtons'
import {
  DECISION_CEILING,
  DECISION_LIMIT,
  DECISION_SKIP,
} from './recommendation'
import {
  CurrentField,
  DecisionField,
  HeadroomField,
  OptionCeilingField,
  OptionLimitField,
  SuggestionField,
} from './Lufs2Fields'

const SETTINGS_URL = '/api/song/loudness/settings'
const RUN_URL = '/api/song/loudness/library?phase=2'

const Lufs2Filter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
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

const BulkActions = (props) => (
  <>
    <SetDecisionButton {...props} decision={DECISION_CEILING} />
    <SetDecisionButton {...props} decision={DECISION_LIMIT} />
    <SetDecisionButton {...props} decision={DECISION_SKIP} />
  </>
)

const RunPhase2Button = ({ status, onStarted }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const running = status?.running
  const handleClick = useCallback(() => {
    httpClient(RUN_URL, { method: 'POST' })
      .then(({ json }) => {
        notify(json?.message || 'Phase 2 run started', 'info')
        onStarted?.()
      })
      .catch((error) =>
        notify(
          error?.body?.message ||
            error?.message ||
            'ra.notification.http_error',
          'warning',
        ),
      )
  }, [notify, onStarted])

  return (
    <RaButton
      onClick={handleClick}
      disabled={!!running}
      label={translate('resources.lufs2.actions.runPhase2')}
    >
      <PlayArrowIcon />
    </RaButton>
  )
}

const Lufs2Actions = ({ status, onStarted, ...rest }) => {
  const { total } = useListContext()
  return (
    <TopToolbar {...rest}>
      <JobProgress
        label="Applying decisions"
        status={status}
        detail={
          status?.running
            ? `${status.normalized || 0} changed · ${status.failed || 0} failed`
            : undefined
        }
      />
      <RunPhase2Button status={status} onStarted={onStarted} />
      <ExportButton maxResults={total} />
    </TopToolbar>
  )
}

// Lufs2List is the review queue: every track whose peaks leave too little room
// to reach the target by turning it up. Each row shows why, and what each
// available option would actually produce, so the choice is made on numbers
// rather than guesswork.
const Lufs2List = (props) => {
  const [settings, setSettings] = useState(null)
  const { status, poll } = useLibraryStatus()

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

  const handleStarted = useCallback(() => poll(), [poll])

  return (
    <List
      {...props}
      sort={{ field: 'title', order: 'ASC' }}
      filter={{ loudness_phase: 2 }}
      filters={<Lufs2Filter />}
      actions={<Lufs2Actions status={status} onStarted={handleStarted} />}
      bulkActionButtons={<BulkActions />}
      perPage={50}
    >
      <Datagrid rowClick={null}>
        <TextField source="title" sortBy="title" />
        <TextField source="artist" sortBy="artist" />
        <CurrentField
          source="current"
          label="Now"
          settings={settings}
          sortBy="lufs_before"
        />
        <HeadroomField
          source="headroom"
          label="Headroom short by"
          settings={settings}
          sortable={false}
        />
        <OptionCeilingField
          source="optionCeiling"
          label="Option A · keep audio intact"
          settings={settings}
          sortable={false}
        />
        <OptionLimitField
          source="optionLimit"
          label="Option B · hit the target"
          settings={settings}
          sortable={false}
        />
        <SuggestionField
          source="suggested"
          label="Suggested"
          settings={settings}
          sortable={false}
        />
        <DecisionField
          source="decision"
          label="Decision"
          sortBy="loudness_decision"
        />
        <PathField source="path" sortBy="path" />
      </Datagrid>
    </List>
  )
}

export default Lufs2List
