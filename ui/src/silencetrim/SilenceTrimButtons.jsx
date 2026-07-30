import React, { useCallback, useMemo, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button,
  Confirm,
  useNotify,
  useRefresh,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import AssessmentIcon from '@material-ui/icons/Assessment'
import CheckCircleOutlineIcon from '@material-ui/icons/CheckCircleOutline'
import BlockIcon from '@material-ui/icons/Block'
import CropIcon from '@material-ui/icons/Crop'
import RestoreIcon from '@material-ui/icons/Restore'
import { httpClient } from '../dataProvider'
import { SILENCE_ANALYZE_URL, SILENCE_APPLY_URL } from './useSilenceTrimStatus'

const request = (url, body, method = 'POST') =>
  httpClient(url, { method, body: JSON.stringify(body) })

export const AnalyzeSilenceButton = ({
  selectedIds,
  libraryIds,
  all,
  disabled,
  onStarted,
}) => {
  const notify = useNotify()
  const [saving, setSaving] = useState(false)
  const ids = useMemo(() => selectedIds || [], [selectedIds])
  const libraries = useMemo(() => libraryIds || [], [libraryIds])
  const hasTargets = all ? libraries.length > 0 : ids.length > 0
  const handleClick = useCallback(() => {
    if (saving || disabled || !hasTargets) return
    setSaving(true)
    request(
      SILENCE_ANALYZE_URL,
      all ? { all: true, libraryIds: libraries } : { ids },
    )
      .then(({ json }) => {
        notify(json?.message || 'Dry-run analysis started', 'info')
        onStarted?.()
      })
      .catch((error) =>
        notify(error?.body?.message || error?.message, 'warning'),
      )
      .finally(() => setSaving(false))
  }, [all, disabled, hasTargets, ids, libraries, notify, onStarted, saving])
  return (
    <Button
      label={all ? 'Analyze library' : 'Analyze selected'}
      onClick={handleClick}
      disabled={saving || disabled || !hasTargets}
    >
      <AssessmentIcon />
    </Button>
  )
}

AnalyzeSilenceButton.propTypes = {
  selectedIds: PropTypes.array,
  libraryIds: PropTypes.array,
  all: PropTypes.bool,
  disabled: PropTypes.bool,
  onStarted: PropTypes.func,
}

AnalyzeSilenceButton.defaultProps = {
  selectedIds: [],
  libraryIds: [],
  all: false,
  disabled: false,
}

export const SetSilenceDecisionButton = ({
  selectedIds,
  resource,
  decision,
  disabled,
}) => {
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const [saving, setSaving] = useState(false)
  const ids = useMemo(() => selectedIds || [], [selectedIds])
  const approve = decision === 'approve'
  const handleClick = useCallback(() => {
    if (!ids.length || saving || disabled) return
    setSaving(true)
    request('/api/song/silence-trim/decision', { ids, decision }, 'PUT')
      .then(() => {
        notify(
          approve
            ? `${ids.length} proposal(s) approved`
            : `${ids.length} song(s) will be left unchanged`,
          'info',
        )
        unselectAll(resource)
        refresh()
      })
      .catch((error) =>
        notify(error?.body?.message || error?.message, 'warning'),
      )
      .finally(() => setSaving(false))
  }, [
    approve,
    decision,
    disabled,
    ids,
    notify,
    refresh,
    resource,
    saving,
    unselectAll,
  ])
  return (
    <Button
      label={approve ? 'Approve proposal' : 'Leave unchanged'}
      onClick={handleClick}
      disabled={!ids.length || saving || disabled}
    >
      {approve ? <CheckCircleOutlineIcon /> : <BlockIcon />}
    </Button>
  )
}

SetSilenceDecisionButton.propTypes = {
  selectedIds: PropTypes.array,
  resource: PropTypes.string.isRequired,
  decision: PropTypes.oneOf(['approve', 'skip']).isRequired,
  disabled: PropTypes.bool,
}

SetSilenceDecisionButton.defaultProps = {
  selectedIds: [],
  disabled: false,
}

export const ApplySilenceButton = ({
  selectedIds,
  libraryIds,
  resource,
  all,
  disabled,
  onStarted,
}) => {
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const [confirming, setConfirming] = useState(false)
  const [saving, setSaving] = useState(false)
  const ids = useMemo(() => selectedIds || [], [selectedIds])
  const libraries = useMemo(() => libraryIds || [], [libraryIds])
  const hasTargets = all ? libraries.length > 0 : ids.length > 0
  const handleConfirm = useCallback(() => {
    setConfirming(false)
    if (saving || disabled || !hasTargets) return
    setSaving(true)
    request(
      SILENCE_APPLY_URL,
      all ? { all: true, libraryIds: libraries } : { ids },
    )
      .then(({ json }) => {
        notify(json?.message || 'Silence trimming started', 'info')
        if (!all) unselectAll(resource)
        refresh()
        onStarted?.()
      })
      .catch((error) =>
        notify(error?.body?.message || error?.message, 'warning'),
      )
      .finally(() => setSaving(false))
  }, [
    all,
    disabled,
    hasTargets,
    ids,
    libraries,
    notify,
    onStarted,
    refresh,
    resource,
    saving,
    unselectAll,
  ])
  const countText = all
    ? `every safe or approved proposal in ${
        libraries.length === 1
          ? 'the selected library'
          : `${libraries.length} selected libraries`
      }`
    : `${ids.length} selected song(s)`
  return (
    <>
      <Button
        label={all ? 'Apply safe / approved trims' : 'Trim selected'}
        onClick={() => setConfirming(true)}
        disabled={saving || disabled || !hasTargets}
      >
        <CropIcon />
      </Button>
      <Confirm
        isOpen={confirming}
        loading={saving}
        title="Apply start/end silence trims?"
        content={`This will process ${countText}. A separate verified backup is created before each file is replaced. Review-only songs are skipped unless approved.`}
        onConfirm={handleConfirm}
        onClose={() => setConfirming(false)}
      />
    </>
  )
}

ApplySilenceButton.propTypes = AnalyzeSilenceButton.propTypes
ApplySilenceButton.defaultProps = AnalyzeSilenceButton.defaultProps

export const RestoreSilenceButton = ({ selectedIds, resource, disabled }) => {
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const [confirming, setConfirming] = useState(false)
  const [saving, setSaving] = useState(false)
  const ids = useMemo(() => selectedIds || [], [selectedIds])
  const handleConfirm = useCallback(() => {
    setConfirming(false)
    if (!ids.length || saving || disabled) return
    setSaving(true)
    request('/api/song/silence-trim/restore', { ids })
      .then(({ json }) => {
        notify(
          `Restored ${json?.restored?.length || 0}; skipped ${
            json?.skipped?.length || 0
          }; failed ${json?.failed?.length || 0}`,
          json?.failed?.length ? 'warning' : 'info',
        )
        unselectAll(resource)
        refresh()
      })
      .catch((error) =>
        notify(error?.body?.message || error?.message, 'warning'),
      )
      .finally(() => setSaving(false))
  }, [disabled, ids, notify, refresh, resource, saving, unselectAll])
  return (
    <>
      <Button
        label="Restore pre-trim"
        onClick={() => setConfirming(true)}
        disabled={!ids.length || saving || disabled}
      >
        <RestoreIcon />
      </Button>
      <Confirm
        isOpen={confirming}
        loading={saving}
        title="Restore exact pre-trim files?"
        content={`Restore ${ids.length} selected song(s) from the separate silence-trim backup? LUFS backups are not used.`}
        onConfirm={handleConfirm}
        onClose={() => setConfirming(false)}
      />
    </>
  )
}

RestoreSilenceButton.propTypes = {
  selectedIds: PropTypes.array,
  resource: PropTypes.string.isRequired,
  disabled: PropTypes.bool,
}

RestoreSilenceButton.defaultProps = {
  selectedIds: [],
  disabled: false,
}
