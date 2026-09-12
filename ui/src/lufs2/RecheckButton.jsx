import React, { useCallback, useEffect, useRef, useState } from 'react'
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
import { LIBRARY_URL } from '../lufs/useLibraryStatus'
import { useJobStatus } from '../lufs/useJobStatus'

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
  const { follow } = useJobStatus(LIBRARY_URL)
  const { follow: followAnalysis } = useJobStatus(ANALYZE_URL)
  const stopFollowing = useRef(null)

  useEffect(() => () => stopFollowing.current?.(), [])

  // The measuring pass runs in the background, so the retry has to wait for it
  // to finish rather than start on top of it.
  const untilAnalyzeSettles = useCallback(() => {
    return new Promise((resolve, reject) => {
      stopFollowing.current = followAnalysis((json) => {
        stopFollowing.current = null
        if (
          json.error ||
          json.failed ||
          json.cancelled ||
          json.processed < json.total
        ) {
          reject(
            new Error(
              json.error ||
                'Analysis did not finish successfully. Review the results before retrying.',
            ),
          )
        } else {
          resolve()
        }
      })
    })
  }, [followAnalysis])

  const handleClick = useCallback(async () => {
    if (!count || busy) return
    setBusy(true)
    try {
      await httpClient(ANALYZE_URL, {
        method: 'POST',
        body: JSON.stringify({ ids: selectedIds }),
      })
      await untilAnalyzeSettles()

      // The retry is a background run now, so its result arrives from the job
      // rather than from the response.
      await dataProvider.optimizeSongLoudness(selectedIds)
      stopFollowing.current = follow((json) => {
        stopFollowing.current = null
        setBusy(false)
        notify('resources.lufs2.notifications.rechecked', {
          type:
            json?.failed || json?.rejected || json?.error ? 'warning' : 'info',
          messageArgs: {
            changed: json?.normalized || 0,
            skipped: json?.skipped || 0,
            rejected: json?.rejected || 0,
            failed: json?.failed || 0,
          },
        })
        refresh()
        onDone?.()
      })
    } catch (error) {
      setBusy(false)
      notify(error?.message || 'ra.notification.http_error', {
        type: 'warning',
      })
    }
  }, [
    busy,
    count,
    dataProvider,
    follow,
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
