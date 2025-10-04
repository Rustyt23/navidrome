import React, { Fragment, useEffect } from 'react'
import {
  BulkDeleteButton,
  useListContext,
  useUnselectAll,
  ResourceContextProvider,
} from 'react-admin'
import { MdOutlinePlaylistRemove } from 'react-icons/md'
import PropTypes from 'prop-types'
import { AddToPlaylistButton } from '../common/AddToPlaylistButton'
import { makeStyles } from '@material-ui/core/styles'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

// Replace original resource with "fake" one for removing tracks from playlist
const PlaylistSongBulkActions = ({
  playlistId,
  resource,
  selectedIds,
  onUnselectItems,
  parentResource = 'playlist',
  readOnly,
  ...rest
}) => {
  const classes = useStyles()
  const unselectAll = useUnselectAll()
  const listContext = useListContext()
  const data = listContext?.data
  useEffect(() => {
    unselectAll(resource)
  }, [unselectAll, resource])

  const mappedResource =
    parentResource === 'discovery'
      ? `discovery/${playlistId}/songs`
      : `playlist/${playlistId}/tracks`
  const selectedMediaIds = selectedIds.map(
    (id) => data?.[id]?.mediaFileId ?? id,
  )
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
        {!readOnly && (
          <AddToPlaylistButton
            resource={mappedResource}
            selectedIds={selectedMediaIds}
            className={classes.button}
          />
        )}
      </Fragment>
    </ResourceContextProvider>
  )
}

PlaylistSongBulkActions.propTypes = {
  playlistId: PropTypes.string.isRequired,
  resource: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ).isRequired,
  onUnselectItems: PropTypes.func,
  parentResource: PropTypes.string,
  readOnly: PropTypes.bool,
}

export default PlaylistSongBulkActions
