import React from 'react'
import PropTypes from 'prop-types'
import { BulkDeleteButton, ResourceContextProvider } from 'react-admin'
import { MdOutlinePlaylistRemove } from 'react-icons/md'

const DiscoverySongBulkActions = ({ discoveryId, onUnselectItems, ...rest }) => {
  const resource = `discovery/${discoveryId}/tracks`
  return (
    <ResourceContextProvider value={resource}>
      <BulkDeleteButton
        {...rest}
        resource={resource}
        label={'ra.action.remove'}
        icon={<MdOutlinePlaylistRemove />}
        onClick={onUnselectItems}
      />
    </ResourceContextProvider>
  )
}

DiscoverySongBulkActions.propTypes = {
  discoveryId: PropTypes.string.isRequired,
  onUnselectItems: PropTypes.func,
}

export default DiscoverySongBulkActions
