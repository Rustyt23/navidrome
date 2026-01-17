import React, { Fragment, useCallback } from 'react'
import { useDispatch } from 'react-redux'
import {
  BulkDeleteButton,
  Button,
  ResourceContextProvider,
  useListContext,
  useTranslate,
} from 'react-admin'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import ShuffleIcon from '@material-ui/icons/Shuffle'
import { MdOutlinePlaylistRemove } from 'react-icons/md'
import PropTypes from 'prop-types'

import { playTracks, shuffleTracks } from '../actions'

const DiscoverySongBulkActions = ({ discoveryId, onUnselectItems }) => {
  const dispatch = useDispatch()
  const translate = useTranslate()
  const { data, selectedIds } = useListContext()

  const handleAction = useCallback(
    (action, includeSelectedId = false) => () => {
      if (!selectedIds || selectedIds.length === 0) {
        return
      }
      const selectedId = includeSelectedId ? selectedIds[0] : undefined
      dispatch(action(data, selectedIds, selectedId))
      onUnselectItems?.()
    },
    [dispatch, data, selectedIds, onUnselectItems],
  )

  const mappedResource = `discovery/${discoveryId}/tracks`

  return (
    <ResourceContextProvider value={mappedResource}>
      <Fragment>
        <Button
          onClick={handleAction(playTracks, true)}
          label={translate('resources.album.actions.playAll')}
        >
          <PlayArrowIcon />
        </Button>
        <Button
          onClick={handleAction(shuffleTracks)}
          label={translate('resources.album.actions.shuffle')}
        >
          <ShuffleIcon />
        </Button>
        <BulkDeleteButton
          label={translate('ra.action.remove')}
          resource={mappedResource}
          icon={<MdOutlinePlaylistRemove />}
          onClick={onUnselectItems}
        />
      </Fragment>
    </ResourceContextProvider>
  )
}

DiscoverySongBulkActions.propTypes = {
  discoveryId: PropTypes.oneOfType([PropTypes.string, PropTypes.number])
    .isRequired,
  onUnselectItems: PropTypes.func,
}

DiscoverySongBulkActions.defaultProps = {
  onUnselectItems: undefined,
}

export default DiscoverySongBulkActions
