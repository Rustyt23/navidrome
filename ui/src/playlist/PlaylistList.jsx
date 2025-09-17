import React, { useMemo, useEffect } from 'react'
import {
  Datagrid,
  DateField,
  EditButton,
  Filter,
  NumberField,
  ReferenceInput,
  SearchInput,
  SelectInput,
  TextField,
  Pagination,
  useUpdate,
  useNotify,
  useRecordContext,
  BulkDeleteButton,
  usePermissions,
  useListContext,
} from 'react-admin'
import Switch from '@material-ui/core/Switch'
import { useMediaQuery } from '@material-ui/core'
import {
  DurationField,
  List,
  Writable,
  isWritable,
  useSelectedFields,
  useResourceRefresh,
} from '../common'
import PlaylistListActions from './PlaylistListActions'
import ChangePublicStatusButton from './ChangePublicStatusButton'
import { usePerPagePreference } from '../common/hooks/usePerPagePreference'
import {
  DEFAULT_PLAYLIST_PER_PAGE,
  PLAYLIST_LIST_PER_PAGE_STORAGE_KEY,
  PLAYLIST_LIST_ROWS_PER_PAGE_OPTIONS,
} from './playlistPaginationConfig'

const PlaylistFilter = (props) => {
  const { permissions } = usePermissions()
  return (
    <Filter {...props} variant={'outlined'}>
      <SearchInput source="q" alwaysOn />
      {permissions === 'admin' && (
        <ReferenceInput
          source="owner_id"
          label={'resources.playlist.fields.ownerName'}
          reference="user"
          perPage={0}
          sort={{ field: 'name', order: 'ASC' }}
          alwaysOn
        >
          <SelectInput optionText="name" />
        </ReferenceInput>
      )}
    </Filter>
  )
}

const TogglePublicInput = ({ resource, source }) => {
  const record = useRecordContext()
  const notify = useNotify()
  const [togglePublic] = useUpdate(
    resource,
    record.id,
    {
      ...record,
      public: !record.public,
    },
    {
      undoable: false,
      onFailure: (error) => {
        notify('ra.page.error', 'warning')
      },
    },
  )

  const handleClick = (e) => {
    togglePublic()
    e.stopPropagation()
  }

  return (
    <Switch
      checked={record[source]}
      onClick={handleClick}
      disabled={!isWritable(record.ownerId)}
    />
  )
}

const ToggleAutoImport = ({ resource, source }) => {
  const record = useRecordContext()
  const notify = useNotify()
  const [ToggleAutoImport] = useUpdate(
    resource,
    record.id,
    {
      ...record,
      sync: !record.sync,
    },
    {
      undoable: false,
      onFailure: (error) => {
        notify('ra.page.error', 'warning')
      },
    },
  )
  const handleClick = (e) => {
    ToggleAutoImport()
    e.stopPropagation()
  }

  return record.path ? (
    <Switch
      checked={record[source]}
      onClick={handleClick}
      disabled={!isWritable(record.ownerId)}
    />
  ) : null
}

const PlaylistListBulkActions = (props) => (
  <>
    <ChangePublicStatusButton public={true} {...props} />
    <ChangePublicStatusButton public={false} {...props} />
    <BulkDeleteButton {...props} />
  </>
)

const PlaylistListPerPageObserver = ({
  rowsPerPageOptions,
  onPerPageChange,
  storedPerPage,
}) => {
  const { perPage, setPerPage } = useListContext()

  useEffect(() => {
    const fallback = rowsPerPageOptions.includes(DEFAULT_PLAYLIST_PER_PAGE)
      ? DEFAULT_PLAYLIST_PER_PAGE
      : rowsPerPageOptions[0]

    if (!rowsPerPageOptions.includes(perPage)) {
      if (perPage !== fallback) {
        setPerPage(fallback)
      }
      if (onPerPageChange) {
        onPerPageChange(fallback)
      }
      return
    }

    if (onPerPageChange && perPage !== storedPerPage) {
      onPerPageChange(perPage)
    }
  }, [
    onPerPageChange,
    perPage,
    rowsPerPageOptions,
    setPerPage,
    storedPerPage,
  ])

  return null
}

const PlaylistList = (props) => {
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('playlist')
  const { perPage: storedPerPage, setPerPage: setStoredPerPage } =
    usePerPagePreference(
      PLAYLIST_LIST_PER_PAGE_STORAGE_KEY,
      DEFAULT_PLAYLIST_PER_PAGE,
    )

  const playlistsPerPage = useMemo(() => {
    if (PLAYLIST_LIST_ROWS_PER_PAGE_OPTIONS.includes(storedPerPage)) {
      return storedPerPage
    }
    return DEFAULT_PLAYLIST_PER_PAGE
  }, [storedPerPage])

  useEffect(() => {
    if (playlistsPerPage !== storedPerPage) {
      setStoredPerPage(playlistsPerPage)
    }
  }, [playlistsPerPage, setStoredPerPage, storedPerPage])

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      songCount: !isXsmall && <NumberField source="songCount" />,
      duration: <DurationField source="duration" />,
      updatedAt: isDesktop && (
        <DateField source="updatedAt" sortByOrder={'DESC'} />
      ),
      public: !isXsmall && (
        <TogglePublicInput source="public" sortByOrder={'DESC'} />
      ),
      comment: <TextField source="comment" />,
      sync: <ToggleAutoImport source="sync" sortByOrder={'DESC'} />,
    }),
    [isDesktop, isXsmall],
  )

  const columns = useSelectedFields({
    resource: 'playlist',
    columns: toggleableFields,
    defaultOff: ['comment'],
  })

  return (
    <List
      {...props}
      exporter={false}
      filters={<PlaylistFilter />}
      actions={<PlaylistListActions />}
      bulkActionButtons={!isXsmall && <PlaylistListBulkActions />}
      perPage={playlistsPerPage}
      pagination={
        <Pagination rowsPerPageOptions={PLAYLIST_LIST_ROWS_PER_PAGE_OPTIONS} />
      }
    >
      <PlaylistListPerPageObserver
        rowsPerPageOptions={PLAYLIST_LIST_ROWS_PER_PAGE_OPTIONS}
        onPerPageChange={setStoredPerPage}
        storedPerPage={storedPerPage}
      />
      <Datagrid rowClick="show" isRowSelectable={(r) => isWritable(r?.ownerId)}>
        <TextField source="name" />
        {columns}
        <Writable>
          <EditButton />
        </Writable>
      </Datagrid>
    </List>
  )
}

export default PlaylistList
