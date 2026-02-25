import React, { useState } from 'react'
import { useNotify, useUnselectAll } from 'react-admin'
import { Button } from '@material-ui/core'
import { BiDownload } from 'react-icons/bi'
import { httpClient } from '../dataProvider'

const CovertartBulkActions = ({ selectedIds = [] }) => {
  const notify = useNotify()
  const unselectAll = useUnselectAll()
  const [loading, setLoading] = useState(false)

  const handleFetchSelected = () => {
    if (!selectedIds.length) {
      return
    }

    setLoading(true)
    httpClient('/api/metadata/musicbrainz/fetch', {
      method: 'POST',
      body: JSON.stringify({ songIds: selectedIds }),
    })
      .then(({ status }) => {
        if (status === 202) {
          notify('activity.musicbrainz.started', 'info')
        } else {
          notify('activity.musicbrainz.alreadyRunning', 'warning')
        }
      })
      .catch(() => notify('activity.musicbrainz.failed', 'warning'))
      .finally(() => {
        setLoading(false)
        unselectAll('covertart')
      })
  }

  return (
    <Button
      color="primary"
      startIcon={<BiDownload />}
      onClick={handleFetchSelected}
      disabled={loading || selectedIds.length === 0}
      aria-label="Fetch metadata for selected songs"
    >
      Fetch Selected Metadata
    </Button>
  )
}

export default CovertartBulkActions
