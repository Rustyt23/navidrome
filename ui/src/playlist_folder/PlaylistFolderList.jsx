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
import PlaylistListActions from './PlaylistListActions'
import EmptyPlaylist from './EmptyPlaylist'
import TypeIconField from './TypeIconField'
import TypeAwareEditButton from './TypeAwareEditButton'
import PlaylistFolderBulkActions from './PlaylistFolderBulkActions'
import { PlaylistFolderDataGrid } from './PlaylistFolderDataGrid'

const PlaylistFolderFilter = (props) => {
  const { permissions } = usePermissions()
  const resource = useResourceContext() || 'folder'
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
  const parentResource = useResourceContext() || 'folder'

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
        record.type === 'folder'
          ? parentResource === 'discoveryFolder'
            ? 'discoveryFolder'
            : 'folder'
          : record.type
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
        }
      )
      },
      [checked, notify, parentResource, record, update]
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

const PlaylistFolderList = (props) => {
  const resource = useResourceContext() || 'folder'
  const isDiscovery = resource === 'discoveryFolder'
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
      const folderPath = isDiscovery ? '/discovery/folder' : '/folder'
      const playlistPath = isDiscovery ? '/discovery' : '/playlist'
      return record?.type === 'folder'
        ? `${folderPath}/${id}/show`
        : `${playlistPath}/${id}/show`
    },
    [isDiscovery],
  )

  return (
    <List
      {...props}
      exporter={false}
      filters={<PlaylistFolderFilter />}
      actions={<PlaylistListActions />}
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

export default PlaylistFolderList
