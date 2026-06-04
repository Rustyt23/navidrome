import React, { Fragment, useEffect, useState } from 'react'
import {
  BulkDeleteButton,
  useListContext,
  useUnselectAll,
  ResourceContextProvider,
  useDataProvider,
} from 'react-admin'
import { MdOutlinePlaylistRemove } from 'react-icons/md'
import PropTypes from 'prop-types'
import { AddToPlaylistButton } from '../common/AddToPlaylistButton'
import { EditSongCommentButton } from '../common/EditSongCommentButton'
import { OptimizeLufsButton } from '../common/OptimizeLufsButton'
import { makeStyles } from '@material-ui/core/styles'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

export const resolveSelectedMediaIds = async ({
  selectedIds,
  data = {},
  playlistId,
  dataProvider,
}) => {
  const resolveFromRecords = (records) => {
    const lookup = { ...records }
    return selectedIds.map((id) => lookup[id]?.mediaFileId ?? id)
  }

  const allIdsAreKnown = selectedIds.every((id) => data?.[id])
  if (allIdsAreKnown) {
    return resolveFromRecords(data)
  }

  try {
    const { data: records } = await dataProvider.getList('playlistTrack', {
      filter: { playlist_id: playlistId },
      pagination: { page: 1, perPage: 0 },
      sort: { field: 'id', order: 'ASC' },
    })

    const lookup = records.reduce(
      (acc, record) => ({ ...acc, [record.id]: record }),
      { ...data },
    )

    return resolveFromRecords(lookup)
  } catch {
    return selectedIds
  }
}

// Replace original resource with "fake" one for removing tracks from playlist
const PlaylistSongBulkActions = ({
  playlistId,
  selectedIds,
  onUnselectItems,
  ...rest
}) => {
  const classes = useStyles()
  const unselectAll = useUnselectAll()
  const listContext = useListContext()
  const dataProvider = useDataProvider()
  const data = listContext?.data
  const [selectedMediaIds, setSelectedMediaIds] = useState([])
  useEffect(() => {
    unselectAll('playlistTrack')
  }, [unselectAll])

  const mappedResource = `playlist/${playlistId}/tracks`
  useEffect(() => {
    let isActive = true

    resolveSelectedMediaIds({
      selectedIds: selectedIds || [],
      data,
      playlistId,
      dataProvider,
    }).then((mediaIds) => {
      if (isActive) {
        setSelectedMediaIds(mediaIds)
      }
    })

    return () => {
      isActive = false
    }
  }, [selectedIds, data, playlistId, dataProvider])
  return (
    <ResourceContextProvider value={mappedResource}>
      <Fragment>
        <BulkDeleteButton
          {...rest}
          label={'ra.action.remove'}
          icon={<MdOutlinePlaylistRemove />}
          resource={mappedResource}
          onClick={onUnselectItems}
        />
        {/* Add the AddToPlaylistButton */}
        <AddToPlaylistButton
          resource={mappedResource} // Use the mapped resource for consistency
          selectedIds={selectedMediaIds} // Pass the mapped media IDs
          className={classes.button} // Apply custom styles
        />
        <EditSongCommentButton
          resource={'song'}
          selectedIds={selectedMediaIds}
          recordIds={selectedIds}
          unselectResource={'playlistTrack'}
          className={classes.button}
        />
        <OptimizeLufsButton
          resource={'playlistTrack'}
          selectedIds={selectedMediaIds}
          className={classes.button}
        />
      </Fragment>
    </ResourceContextProvider>
  )
}

PlaylistSongBulkActions.propTypes = {
  playlistId: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ).isRequired,
  onUnselectItems: PropTypes.func,
}

export default PlaylistSongBulkActions
