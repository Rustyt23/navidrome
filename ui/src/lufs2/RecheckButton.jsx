import React, { useCallback, useState } from 'react'
import PropTypes from 'prop-types'
import {
  Button as RaButton,
  useDataProvider,
  useNotify,
  useRefresh,
  useTranslate,
} from 'react-admin'
import { CircularProgress } from '@material-ui/core'
import ReplayIcon from '@material-ui/icons/Replay'
import { httpClient } from '../dataProvider'
import { ANALYZE_URL } from '../lufs/useAnalyzeStatus'

// RecheckButton measures the selected songs again and then tries them again.
//
// The two halves have to go together. Measuring rebuilds the record, which
// clears the mark left by a run that built a file and refused it - that mark is
// what keeps later runs from rebuilding the same rejected file for ever, and it
// is only as good as the settings it was made under. But clearing it on its own
// just moves the song into the queue: it drops off this page immediately and
// nothing retries it, so a list of problems empties out while every problem is
// still there.
//
// Doing both means a song either comes back with a current reason or is
// genuinely fixed and gone.
export const RecheckButton = ({ selectedIds, onDone }) => {
  const translate = useTranslate()
  const notify = useNotify()
  const refresh = useRefresh()
  const dataProvider = useDataProvider()
  const [busy, setBusy] = useState(false)
  const count = selectedIds?.length || 0

  // The measuring pass runs in the background, so the retry has to wait for it
  // to finish rather than start on top of it.
  const untilAnalyzeSettles = useCallback(() => {
    return new Promise((resolve) => {
      let attempts = 0
      const check = () => {
        httpClient(ANALYZE_URL)
          .then(({ json }) => {
            attempts += 1
            if ((json && !json.running) || attempts >= 600) return resolve()
            setTimeout(check, 500)
          })
          .catch(() => resolve())
      }
      setTimeout(check, 400)
    })
  }, [])

  const handleClick = useCallback(async () => {
    if (!count || busy) return
    setBusy(true)
    try {
      await httpClient(ANALYZE_URL, {
        method: 'POST',
        body: JSON.stringify({ ids: selectedIds }),
      })
      await untilAnalyzeSettles()

      const response = await dataProvider.optimizeSongLoudness(selectedIds)
      const changed = response?.data?.normalized?.length || 0
      const skipped = response?.data?.skipped?.length || 0
      const failed = response?.data?.failed?.length || 0
      notify('resources.lufs2.notifications.rechecked', {
        type: failed ? 'warning' : 'info',
        messageArgs: { changed, skipped, failed },
      })
      refresh()
      onDone?.()
    } catch (error) {
      notify(error?.message || 'ra.notification.http_error', {
        type: 'warning',
      })
    } finally {
      setBusy(false)
    }
  }, [
    busy,
    count,
    dataProvider,
    notify,
    onDone,
    refresh,
    selectedIds,
    untilAnalyzeSettles,
  ])

  return (
    <RaButton
      onClick={handleClick}
      disabled={!count || busy}
      label={translate('resources.lufs2.actions.recheck')}
    >
      {busy ? <CircularProgress size={16} /> : <ReplayIcon />}
    </RaButton>
  )
}

RecheckButton.propTypes = {
  selectedIds: PropTypes.array,
  onDone: PropTypes.func,
}

RecheckButton.defaultProps = { selectedIds: [] }

export default RecheckButton
