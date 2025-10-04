import React, { isValidElement, useCallback, useEffect, useRef } from 'react'
import {
  Datagrid,
  PureDatagridBody,
  PureDatagridRow,
  useDataProvider,
  useNotify,
  useRefresh,
  useResourceContext,
} from 'react-admin'
import PropTypes from 'prop-types'
import clsx from 'clsx'
import { DraggableTypes } from '../consts'
import { makeStyles } from '@material-ui/core/styles'
import { useHistory, useLocation } from 'react-router-dom'
import useDragAndDrop from '../common/useDragAndDrop'
import { matchPath } from 'react-router'

const useStyles = makeStyles({
  row: {
    cursor: 'pointer',
    '&:hover': { backgroundColor: '#f5f5f5' },
    '& td': { paddingTop: 3, paddingBottom: 3 },
  },
  missingRow: {
    cursor: 'inherit',
    opacity: 0.3,
  },
  headerStyle: {
    '& thead': { boxShadow: '0px 3px 3px rgba(0,0,0,.15)' },
    '& th': { fontWeight: 'bold', padding: '15px' },
  },
})

const PlaylistFolderRow = ({ record, children, className, rowClick, ...rest }) => {
  const classes = useStyles()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const refresh = useRefresh()
  const history = useHistory()
  const location = useLocation()
  const listResource = useResourceContext() || 'folder'
  const isDiscovery = listResource === 'discoveryFolder' || record.type === 'discovery'

  const pathname = location?.pathname || '/'

  const sourceId = (() => {
    const pattern = isDiscovery ? '/discovery/folder/:id/show' : '/folder/:id/show'
    const m = matchPath(pathname, { path: pattern, exact: false })
    return m?.params?.id ?? ''
  })()

  const sourceIdRef = useRef('')
  useEffect(() => {
    sourceIdRef.current = sourceId
  }, [sourceId])

  const fields = React.Children.toArray(children).filter((c) => isValidElement(c))

  const handleDrop = useCallback(
    async (item) => {
      try {
        if (!item) return
        const currentSourceId = sourceIdRef.current ?? ''
        const playlistType = isDiscovery ? 'discovery' : 'playlist'
        const folderType = isDiscovery ? 'discoveryFolder' : 'folder'
        const isTargetFolder = record.type === 'folder'
        const isTargetPlaylist = record.type === 'playlist' || record.type === 'discovery'

        if (isTargetPlaylist && item.type !== playlistType && item.type !== folderType) {
          const addFn = isDiscovery ? dataProvider.addToDiscovery : dataProvider.addToPlaylist
          const res = await addFn(record.id, item)
          notify('message.songsAddedToPlaylist', 'info', { smart_count: res?.data?.added })
          refresh()
          return
        }

        if (item.id === record.id) return

        if (item.type === playlistType) {
          const targetFolderId = isTargetFolder ? record.id : null
          if (isDiscovery) {
            await dataProvider.setDiscoveryFolder({
              discoveryId: item.id,
              targetFolderId,
              sourceParentId: currentSourceId,
            })
          } else {
            await dataProvider.setPlaylistFolder({
              playlistId: item.id,
              targetFolderId,
              sourceParentId: currentSourceId,
            })
          }
        } else if (item.type === folderType) {
          const targetParentId = isTargetFolder ? record.id : null
          if (isDiscovery) {
            await dataProvider.moveDiscoveryFolder({
              folderId: item.id,
              targetParentId,
              sourceParentId: currentSourceId,
            })
          } else {
            await dataProvider.moveFolder({
              folderId: item.id,
              targetParentId,
              sourceParentId: currentSourceId,
            })
          }
        }
        notify('message.movedSuccess', 'info')
        refresh()
      } catch (e) {
        notify('ra.page.error', 'warning')
      }
    },
    [
      dataProvider,
      notify,
      refresh,
      record.id,
      record.type,
      isDiscovery,
    ]
  )

  const { dragDropRef, isDragging } = useDragAndDrop(
    record.type === 'playlist'
      ? DraggableTypes.PLAYLIST
      : record.type === 'discovery'
      ? DraggableTypes.DISCOVERY
      : isDiscovery
      ? DraggableTypes.DISCOVERY_FOLDER
      : DraggableTypes.FOLDER,
    { id: record.id, type: record.type === 'folder' && isDiscovery ? 'discoveryFolder' : record.type },
    record.type === 'playlist' || record.type === 'discovery'
      ? DraggableTypes.ALL
      : isDiscovery
      ? [DraggableTypes.DISCOVERY, DraggableTypes.DISCOVERY_FOLDER]
      : [DraggableTypes.PLAYLIST, DraggableTypes.FOLDER],
    handleDrop
  )

  const computedClasses = clsx(
    className,
    classes.row,
    record.missing && classes.missingRow
  )

  const handleRowClick = (event) => {
    event.preventDefault()
    if (typeof rowClick === 'function') {
      const target = rowClick(record.id, record)
      if (target) history.push(target)
    }
  }

  return (
    <PureDatagridRow
      ref={dragDropRef}
      record={record}
      {...rest}
      className={computedClasses}
      onClick={handleRowClick}
      style={{ opacity: isDragging ? 0.5 : 1 }}
    >
      {fields}
    </PureDatagridRow>
  )
}

PlaylistFolderRow.propTypes = {
  record: PropTypes.object,
  children: PropTypes.node,
  rowClick: PropTypes.func,
  className: PropTypes.string,
}

const PlaylistFolderBody = (props) => (
  <PureDatagridBody {...props} row={<PlaylistFolderRow />} />
)

export const PlaylistFolderDataGrid = (props) => {
  const classes = useStyles()
  return (
    <Datagrid
      className={classes.headerStyle}
      isRowSelectable={(r) => !r?.missing}
      {...props}
      body={<PlaylistFolderBody />}
    />
  )
}

PlaylistFolderDataGrid.propTypes = {
  children: PropTypes.node,
}
