import React, { useCallback, useState } from 'react'
import PropTypes from 'prop-types'
import { Button as RaButton, useNotify, useTranslate } from 'react-admin'
import AssessmentIcon from '@material-ui/icons/Assessment'
import { httpClient } from '../dataProvider'
import { ANALYZE_URL } from './useAnalyzeStatus'

// AnalyzeLufsButton starts a measuring pass. `mode="original"` records only
// each song's own loudness - one pass per file, skipping the comparison
// against stored originals - and skips tracks already measured, so it can be
// stopped and resumed freely.
export const AnalyzeLufsButton = ({
  selectedIds,
  all,
  onStarted,
  disabled,
  mode,
  label,
  icon,
}) => {
  const translate = useTranslate()
  const notify = useNotify()
  const [saving, setSaving] = useState(false)
  const count = selectedIds?.length || 0

  const handleClick = useCallback(() => {
    if (saving || disabled || (!all && !count)) return
    setSaving(true)
    httpClient(ANALYZE_URL, {
      method: 'POST',
      body: JSON.stringify({
        ...(all ? { all: true } : { ids: selectedIds }),
        ...(mode ? { mode } : {}),
      }),
    })
      .then(({ json }) => {
        notify(json?.message || 'Analysis started', 'info')
        onStarted?.()
      })
      .catch((error) => {
        notify(
          error?.body?.message ||
            error?.message ||
            'ra.notification.http_error',
          'warning',
        )
      })
      .finally(() => setSaving(false))
  }, [all, count, disabled, mode, notify, onStarted, saving, selectedIds])

  return (
    <RaButton
      onClick={handleClick}
      label={translate(
        label ||
          (all
            ? 'resources.lufs.actions.analyzeAll'
            : 'resources.lufs.actions.analyze'),
      )}
      disabled={saving || disabled || (!all && !count)}
    >
      {icon || <AssessmentIcon />}
    </RaButton>
  )
}

AnalyzeLufsButton.propTypes = {
  selectedIds: PropTypes.array,
  all: PropTypes.bool,
  onStarted: PropTypes.func,
  disabled: PropTypes.bool,
  mode: PropTypes.string,
  label: PropTypes.string,
  icon: PropTypes.node,
}

AnalyzeLufsButton.defaultProps = {
  selectedIds: [],
  all: false,
  disabled: false,
}

export default AnalyzeLufsButton
