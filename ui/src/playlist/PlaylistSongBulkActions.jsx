import React, { Fragment, useEffect } from 'react'
import {
  BulkDeleteButton,
  useUnselectAll,
  ResourceContextProvider,
  useListContext,
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
const PlaylistSongBulkActions = ({ playlistId, onUnselectItems, ...rest }) => {
  const classes = useStyles()
  const { resource, selectedIds } = useListContext()
  const unselectAll = useUnselectAll()
  useEffect(() => {
    unselectAll(resource)
  }, [unselectAll, resource])

  const mappedResource = `playlist/${playlistId}/tracks`
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
        <AddToPlaylistButton
          resource={resource}
          selectedIds={selectedIds}
          className={classes.button}
        />
      </Fragment>
    </ResourceContextProvider>
  )
}

PlaylistSongBulkActions.propTypes = {
  playlistId: PropTypes.string.isRequired,
  onUnselectItems: PropTypes.func,
}

export default PlaylistSongBulkActions
