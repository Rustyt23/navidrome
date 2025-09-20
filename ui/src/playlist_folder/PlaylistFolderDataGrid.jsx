import React, { isValidElement, useCallback, useEffect, useRef } from 'react'
import {
  Datagrid,
  PureDatagridBody,
  PureDatagridRow,
  useDataProvider,
  useNotify,
  useRefresh,
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
    // ↓ shrink row height by reducing vertical padding on all table cells
    '& td, & th, & .MuiTableCell-root': {
      paddingTop: 1,
      paddingBottom: 1,
    },
    '&:hover': { backgroundColor: '#f5f5f5' },
  },
  rowCell: {
    paddingTop: 1,
    paddingBottom: 1,
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

  const pathname = location?.pathname || '/'

  const sourceId = (() => {
    const m = matchPath(pathname, { path: '/folder/:id/show', exact: false })
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
        const isTargetFolder = record.type === 'folder'
        const isTargetPlaylist = record.type === 'playlist'

        if (isTargetPlaylist && item.type !== 'playlist' && item.type !== 'folder') {
          const res = await dataProvider.addToPlaylist(record.id, item)
          notify('message.songsAddedToPlaylist', 'info', { smart_count: res?.data?.added })
          refresh()
          return
        }

        if (item.id === record.id) return

        if (item.type === 'playlist') {
          const targetFolderId = isTargetFolder ? record.id : null
          await dataProvider.setPlaylistFolder({
            playlistId: item.id,
            targetFolderId: targetFolderId,
            sourceParentId: currentSourceId
          })
        } else if (item.type === 'folder') {
          const targetParentId = isTargetFolder ? record.id : null
          await dataProvider.moveFolder({
            folderId: item.id,
            targetParentId: targetParentId,
            sourceParentId: currentSourceId
          })
        }
        notify('message.movedSuccess', 'info')
        refresh()
      } catch (e) {
        notify('ra.page.error', 'warning')
      }
    },
    [dataProvider, notify, refresh, record.id, record.type]
  )

  const { dragDropRef, isDragging } = useDragAndDrop(
    record.type === 'playlist' ? DraggableTypes.PLAYLIST : DraggableTypes.FOLDER,
    { id: record.id, type: record.type },
    record.type === 'playlist' ? DraggableTypes.ALL : [DraggableTypes.PLAYLIST, DraggableTypes.FOLDER],
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

export const PlaylistFolderDataGrid = ({ classes: classesProp, ...props }) => {
  const classes = useStyles()
  const datagridClasses = {
    ...classesProp,
    rowCell: clsx(classes.rowCell, classesProp?.rowCell),
  }
  return (
    <Datagrid
      className={classes.headerStyle}
      isRowSelectable={(r) => !r?.missing}
      classes={datagridClasses}
      {...props}
      body={<PlaylistFolderBody />}
    />
  )
}

PlaylistFolderDataGrid.propTypes = {
  children: PropTypes.node,
  classes: PropTypes.object,
}
