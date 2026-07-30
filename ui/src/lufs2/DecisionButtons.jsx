import React, { useCallback, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button as RaButton,
  useNotify,
  useRefresh,
  useTranslate,
  useUnselectAll,
} from 'react-admin'
import GraphicEqIcon from '@material-ui/icons/GraphicEq'
import VolumeDownIcon from '@material-ui/icons/VolumeDown'
import BlockIcon from '@material-ui/icons/Block'
import { CircularProgress } from '@material-ui/core'
import { httpClient } from '../dataProvider'
import {
  DECISION_CEILING,
  DECISION_LIMIT,
  DECISION_SKIP,
} from './recommendation'

const DECISION_URL = '/api/song/loudness/decision'

const ICONS = {
  [DECISION_LIMIT]: <GraphicEqIcon />,
  [DECISION_CEILING]: <VolumeDownIcon />,
  [DECISION_SKIP]: <BlockIcon />,
}

// SetDecisionButton records what the client wants done with the selected
// tracks. It only stores the intent - nothing is applied to any file until a
// phase 2 run is started.
export const SetDecisionButton = ({ decision, selectedIds, resource }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const unselectAll = useUnselectAll()
  const [saving, setSaving] = useState(false)
  const count = selectedIds?.length || 0

  const handleClick = useCallback(() => {
    if (!count || saving) return
    setSaving(true)
    httpClient(DECISION_URL, {
      method: 'PUT',
      body: JSON.stringify({ ids: selectedIds, decision }),
    })
      .then(({ json }) => {
        notify('resources.lufs2.notifications.decisionSaved', 'info', {
          smart_count: json?.updated ?? count,
        })
        unselectAll(resource)
        refresh()
      })
      .catch((error) =>
        notify(error?.message || 'ra.notification.http_error', 'warning'),
      )
      .finally(() => setSaving(false))
  }, [
    count,
    decision,
    notify,
    refresh,
    resource,
    saving,
    selectedIds,
    unselectAll,
  ])

  return (
    <RaButton
      onClick={handleClick}
      disabled={!count || saving}
      label={translate(`resources.lufs2.actions.${decision}`)}
    >
      {saving ? <CircularProgress size={16} /> : ICONS[decision]}
    </RaButton>
  )
}

SetDecisionButton.propTypes = {
  decision: PropTypes.string.isRequired,
  selectedIds: PropTypes.array,
  resource: PropTypes.string,
}

SetDecisionButton.defaultProps = { selectedIds: [] }

export default SetDecisionButton
