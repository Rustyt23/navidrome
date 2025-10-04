import { useMemo, useState, useCallback, useEffect } from 'react'
import {
  DateField,
  Filter,
  ReferenceInput,
  SearchInput,
  SelectInput,
  TextField,
  useUpdate,
  useNotify,
  useRecordContext,
  usePermissions,
  useResourceContext,
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import Switch from '@material-ui/core/Switch'

import {
  List,
  Writable,
  isWritable,
  useSelectedFields,
  useResourceRefresh,
} from '../common'
import DiscoveryListActions from './DiscoveryListActions'
import EmptyPlaylist from '../playlist_folder/EmptyPlaylist'
import TypeIconField from '../playlist_folder/TypeIconField'
import TypeAwareEditButton from '../playlist_folder/TypeAwareEditButton'
import PlaylistFolderBulkActions from '../playlist_folder/PlaylistFolderBulkActions'
import { PlaylistFolderDataGrid } from '../playlist_folder/PlaylistFolderDataGrid'

const DiscoveryFolderFilter = (props) => {
  const { permissions } = usePermissions()
  const resource = useResourceContext() || 'discoveryFolder'
  return (
    <Filter {...props} variant="outlined">
      <SearchInput source="q" alwaysOn />
      {permissions === 'admin' && (
        <ReferenceInput
          source="owner_id"
          label={`resources.${resource}.fields.ownerName`}
          reference="user"
          perPage={50}
          sort={{ field: 'name', order: 'ASC' }}
          alwaysOn
        >
          <SelectInput optionText="name" />
        </ReferenceInput>
      )}
    </Filter>
  )
}

const TogglePublicInput = ({ source }) => {
  const record = useRecordContext()
  const notify = useNotify()
  const [update, { isLoading }] = useUpdate()

  const serverValue = Boolean(record?.[source])

  const [checked, setChecked] = useState(serverValue)

  useEffect(() => {
    setChecked(serverValue)
  }, [serverValue])

  const handleChange = useCallback(
    (e) => {
      e.stopPropagation()
      if (!record?.id) return
      const resource =
        record.type === 'folder' ? 'discoveryFolder' : record.type
      const next = !checked
      setChecked(next)

      update(
        resource,
        record.id,
        { ...record, public: next },
        {
          onFailure: () => {
            setChecked(!next)
            notify('ra.page.error', 'warning')
          },
        },
      )
    },
    [checked, notify, record, update],
  )

  return (
    <Switch
      checked={checked}
      onChange={handleChange}
      onClick={(e) => e.stopPropagation()}
      disabled={isLoading || !isWritable(record?.ownerId)}
      color="primary"
      size="small"
      inputProps={{ 'aria-label': 'toggle-public' }}
    />
  )
}

const DiscoveryFolderList = (props) => {
  const resource = useResourceContext() || 'discoveryFolder'
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh(resource)

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      updatedAt: isDesktop && <DateField source="updatedAt" />,
      public: !isXsmall && <TogglePublicInput source="public" />,
    }),
    [isDesktop, isXsmall],
  )

  const columns = useSelectedFields({
    resource,
    columns: toggleableFields,
  })

  const rowClick = useCallback(
    (id, record) => {
      const folderPath = '/discovery/folder'
      const playlistPath = '/discovery'
      const recordType = record?.type
      return recordType === 'folder' || recordType === 'discoveryFolder'
        ? `${folderPath}/${id}/show`
        : `${playlistPath}/${id}/show`
    },
    [],
  )

  return (
    <List
      {...props}
      exporter={false}
      filters={<DiscoveryFolderFilter />}
      actions={<DiscoveryListActions />}
      bulkActionButtons={!isXsmall && <PlaylistFolderBulkActions resource={resource} />}
      empty={<EmptyPlaylist />}
      perPage={isXsmall ? 50 : 50}
    >
      <PlaylistFolderDataGrid rowClick={rowClick}>
        <TypeIconField label={false} />
        <TextField source="name" />
        {columns}
        <Writable>
          <TypeAwareEditButton />
        </Writable>
      </PlaylistFolderDataGrid>
    </List>
  )
}

export default DiscoveryFolderList
