import React, { useCallback } from 'react'
import PropTypes from 'prop-types'
import clsx from 'clsx'
import { Button as RaButton, useNotify, useTranslate } from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import GetAppIcon from '@material-ui/icons/GetApp'
import config from '../config'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

// The server caps a single archive at this many songs. Checked here as well so
// an over-large selection is refused with a sentence rather than by a download
// that starts and turns out to be an error page named .zip.
const MAX_SELECTION = 500

const DOWNLOAD_URL = '/api/song/download'

// DownloadSongsButton archives the selected songs and hands them to the browser.
//
// A navigation rather than a fetch, and that is deliberate: the browser streams
// the response straight to disk, so a selection of a few hundred megabytes
// costs nothing in memory. Fetching it instead would mean holding the whole
// archive in the tab as a blob before the save dialog ever appeared.
//
// The token rides in the query string because a navigation cannot carry
// headers. The auth chain already accepts it there - it is how the event stream
// authenticates - so this needs nothing new on the server side.
export const DownloadSongsButton = ({ selectedIds, className, disabled }) => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const selectedCount = selectedIds?.length || 0

  const handleClick = useCallback(() => {
    if (!selectedCount) return
    if (selectedCount > MAX_SELECTION) {
      notify('resources.song.notifications.downloadTooMany', {
        type: 'warning',
        messageArgs: { max: MAX_SELECTION, count: selectedCount },
      })
      return
    }

    const params = new URLSearchParams()
    selectedIds.forEach((id) => params.append('id', id))
    // raw: these pages exist to show what normalization did to the actual
    // files, and handing back a transcode would answer a question nobody asked.
    params.append('format', 'raw')
    const token = localStorage.getItem('token')
    if (token) params.append('jwt', token)

    // Assigned rather than opened in a tab: an attachment response never
    // replaces the page, and window.open is what popup blockers stop.
    window.location.assign(
      `${config.baseURL || ''}${DOWNLOAD_URL}?${params.toString()}`,
    )
  }, [notify, selectedCount, selectedIds])

  return (
    <RaButton
      onClick={handleClick}
      className={clsx(classes.button, className)}
      disabled={disabled || !selectedCount}
      label={translate('resources.song.actions.download')}
    >
      <GetAppIcon />
    </RaButton>
  )
}

DownloadSongsButton.propTypes = {
  selectedIds: PropTypes.arrayOf(PropTypes.string),
  className: PropTypes.string,
  disabled: PropTypes.bool,
}

export default DownloadSongsButton
