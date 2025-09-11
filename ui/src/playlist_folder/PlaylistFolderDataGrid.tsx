// @ts-nocheck
import React, { isValidElement, useCallback, useEffect, useRef, useState } from 'react'
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
import { DraggableTypes, REST_URL } from '../consts'
import { makeStyles } from '@material-ui/core/styles'
import { useHistory, useLocation } from 'react-router-dom'
import useDragAndDrop from '../common/useDragAndDrop'
import { matchPath } from 'react-router'
import DuplicateSongDialog from '../dialogs/DuplicateSongDialog'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles({
  row: {
    cursor: 'pointer',
    '&:hover': { backgroundColor: '#f5f5f5' },
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
  const [dupDialogOpen, setDupDialogOpen] = useState(false)
  const [pendingItem, setPendingItem] = useState(null)
  const [duplicateIds, setDuplicateIds] = useState([])

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
          if (item.ids?.length) {
            const res = await httpClient(`${REST_URL}/playlist/${record.id}/tracks`)
            const existing = res.json?.map((t) => t.mediaFileId) || []
            const dups = item.ids.filter((id) => existing.includes(id))
            if (dups.length) {
              setPendingItem(item)
              setDuplicateIds(dups)
              setDupDialogOpen(true)
              return
            }
          }
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

  const handleDuplicate = useCallback(async () => {
    setDupDialogOpen(false)
    if (pendingItem) {
      try {
        const res = await dataProvider.addToPlaylist(record.id, pendingItem)
        notify('message.songsAddedToPlaylist', 'info', { smart_count: res?.data?.added })
        refresh()
      } catch {
        notify('ra.page.error', 'warning')
      }
    }
  }, [dataProvider, notify, refresh, record.id, pendingItem])

  const handleSkip = useCallback(async () => {
    setDupDialogOpen(false)
    if (!pendingItem) return
    const ids = pendingItem.ids.filter((id) => !duplicateIds.includes(id))
    if (!ids.length) return
    try {
      const res = await dataProvider.addToPlaylist(record.id, { ...pendingItem, ids })
      notify('message.songsAddedToPlaylist', 'info', { smart_count: res?.data?.added })
      refresh()
    } catch {
      notify('ra.page.error', 'warning')
    }
  }, [dataProvider, notify, refresh, record.id, pendingItem, duplicateIds])

  const handleNativeDrop = useCallback(
    async (event) => {
      try {
        const data = event?.dataTransfer?.getData('application/json')
        if (!data) return
        const payload = JSON.parse(data)
        if (payload?.type !== 'songs' || !payload.ids?.length) return
        event.preventDefault()
        await handleDrop(payload)
      } catch {
        // ignore parse errors
      }
    },
    [handleDrop],
  )

  const handleDragOver = useCallback((event) => {
    if (event?.dataTransfer?.types?.includes('application/json')) {
      event.preventDefault()
    }
  }, [])

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
    <>
      <PureDatagridRow
        ref={dragDropRef}
        record={record}
        {...rest}
        className={computedClasses}
        onClick={handleRowClick}
        style={{ opacity: isDragging ? 0.5 : 1 }}
        onDragOver={handleDragOver}
        onDrop={handleNativeDrop}
      >
        {fields}
      </PureDatagridRow>
      <DuplicateSongDialog
        open={dupDialogOpen}
        handleSkip={handleSkip}
        handleDuplicate={handleDuplicate}
        messageKey="resources.playlist.message.songs_exist"
      />
    </>
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
