// ui/src/playlist_folder/PlaylistFolderList.jsx
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
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import Switch from '@material-ui/core/Switch'

// [ADDED] local styles to shrink the MUI Switch
import { makeStyles } from '@material-ui/core/styles'

import {
  List,
  Pagination,
  Writable,
  getStoredPageSize,
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

// [ADDED] compact switch styles (local to this file only)
const useSmallSwitchStyles = makeStyles({
  swRoot: {
    transform: 'scale(0.78)',
    transformOrigin: 'left center',
    margin: 0,
    padding: 0,
  },
  swBase: {
    padding: 6, // tighter than default to avoid inflating row height
  },
  swThumb: {},
  swTrack: {},
})

const PlaylistFolderFilter = (props) => {
  const { permissions } = usePermissions()
  return (
    <Filter {...props} variant="outlined">
      <SearchInput source="q" alwaysOn />
      {permissions === 'admin' && (
        <ReferenceInput
          source="owner_id"
          label="resources.playlist.fields.ownerName"
          reference="user"
          perPage={25}
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
  const s = useSmallSwitchStyles() // [ADDED] use compact styles

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
      const resource = record.type
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
    [checked, notify, record, update]
  )

  return (
    <Switch
      classes={{ root: s.swRoot, switchBase: s.swBase, thumb: s.swThumb, track: s.swTrack }} // [ADDED]
      checked={checked}
      onChange={handleChange}
      onClick={(e) => e.stopPropagation()}
      disabled={isLoading || !isWritable(record?.ownerId)}
      color="primary"
      inputProps={{ 'aria-label': 'toggle-public' }}
    />
  )
}

const rowClick = (id, record) =>
  record?.type === 'folder' ? `/folder/${id}/show` : `/playlist/${id}/show`

const PlaylistFolderList = (props) => {
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('folder')

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      updatedAt: isDesktop && <DateField source="updatedAt" />,
      public: !isXsmall && <TogglePublicInput source="public" />,
    }),
    [isDesktop, isXsmall],
  )

  const columns = useSelectedFields({
    resource: 'folder',
    columns: toggleableFields,
  })

  const initialPerPage = useMemo(() => getStoredPageSize('folders'), [])

  return (
    <List
      {...props}
      exporter={false}
      filters={<PlaylistFolderFilter />}
      actions={<PlaylistListActions />}
      bulkActionButtons={!isXsmall && <PlaylistFolderBulkActions />}
      empty={<EmptyPlaylist />}
      perPage={initialPerPage}
      pagination={<Pagination scope="folders" />}
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

