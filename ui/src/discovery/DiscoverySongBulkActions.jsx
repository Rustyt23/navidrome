import React, { Fragment, useEffect } from 'react'
import {
  BulkDeleteButton,
  useListContext,
  useUnselectAll,
  ResourceContextProvider,
} from 'react-admin'
import { MdOutlineDiscoveryRemove } from 'react-icons/md'
import PropTypes from 'prop-types'
import { AddToDiscoveryButton } from '../common/AddToDiscoveryButton'
import { makeStyles } from '@material-ui/core/styles'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

// Replace original resource with "fake" one for removing tracks from playlist
const DiscoverySongBulkActions = ({
  playlistId,
  resource,
  selectedIds,
  onUnselectItems,
  ...rest
}) => {
  const classes = useStyles()
  const unselectAll = useUnselectAll()
  const listContext = useListContext()
  const data = listContext?.data
  useEffect(() => {
    unselectAll('playlistTrack')
  }, [unselectAll])

  const mappedResource = `playlist/${playlistId}/tracks`
  const selectedMediaIds = selectedIds.map(
    (id) => data?.[id]?.mediaFileId ?? id,
  )
  return (
    <ResourceContextProvider value={mappedResource}>
      <Fragment>
        <BulkDeleteButton
          {...rest}
          label={'ra.action.remove'}
          icon={<MdOutlineDiscoveryRemove />}
          resource={mappedResource}
          onClick={onUnselectItems}
        />
        {/* Add the AddToDiscoveryButton */}
        <AddToDiscoveryButton
          resource={mappedResource} // Use the mapped resource for consistency
          selectedIds={selectedMediaIds} // Pass the mapped media IDs
          className={classes.button} // Apply custom styles
        />
      </Fragment>
    </ResourceContextProvider>
  )
}

DiscoverySongBulkActions.propTypes = {
  playlistId: PropTypes.string.isRequired,
  resource: PropTypes.string.isRequired,
  selectedIds: PropTypes.arrayOf(
    PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  ).isRequired,
  onUnselectItems: PropTypes.func,
}

export default DiscoverySongBulkActions
