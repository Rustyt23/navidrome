// ui/src/playlist_folder/PlaylistFolderShow.jsx
import { useMemo, useState, useCallback, useEffect } from 'react'
import {
  ShowBase,
  List,
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
import { makeStyles } from '@material-ui/core/styles'

import {
  Writable,
  isWritable,
  useSelectedFields,
  useResourceRefresh,
  Title,
  Pagination,
  getStoredPageSize,
} from '../common'
import PlaylistListActions from './PlaylistListActions'
import EmptyPlaylist from './EmptyPlaylist'
import TypeIconField from './TypeIconField'
import TypeAwareEditButton from './TypeAwareEditButton'
import PlaylistFolderBulkActions from './PlaylistFolderBulkActions'
import { PlaylistFolderDataGrid } from './PlaylistFolderDataGrid'

/* ========= Compact, smooth Switch (keeps native look/colors) ========= */
const useCompactSwitchStyles = makeStyles((theme) => ({
  // Track area
  swRoot: {
    width: 32,         // default ~34
    height: 18,        // default ~20
    padding: 0,
    margin: 0,
    display: 'inline-flex',
    alignItems: 'center',
  },
  // The draggable base (controls thumb transform)
  swBase: {
    padding: 2,        // ensures 14px thumb fits: 2 + 14 + 2 = 18
    transition: theme.transitions.create(['transform'], { duration: 120 }),
    '&$swChecked': {
      // travel = track(32) - thumb(14) - 2*padding(2+2) = 14
      transform: 'translateX(14px)',
    },
  },
  // Thumb (circle)
  swThumb: {
    width: 14,
    height: 14,
    boxShadow: 'none',
  },
  // Track (pill) — let theme handle colors, just smooth transition
  swTrack: {
    borderRadius: 9,
    opacity: 1,
    transition: theme.transitions.create(['background-color', 'border'], { duration: 120 }),
  },
  swChecked: {},       // required so &$swChecked selectors work
  swFocusVisible: {},  // optional hook if you want focus styles later
}))
/* ==================================================================== */

const PlaylistFolderFilter = (props) => {
  const { permissions } = usePermissions()
  return (
    <Filter {...props} variant="outlined">
      <SearchInput source="q" alwaysOn resettable />
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
  const s = useCompactSwitchStyles()
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
      disableRipple
      classes={{
        root: s.swRoot,
        switchBase: s.swBase,
        thumb: s.swThumb,
        track: s.swTrack,
        checked: s.swChecked,
      }}
      checked={checked}
      onChange={handleChange}
      onClick={(e) => e.stopPropagation()}
      disabled={isLoading || !isWritable(record?.ownerId)}
      color="primary" // keep native theme palette
      inputProps={{ 'aria-label': 'toggle-public' }}
    />
  )
}

const rowClick = (id, record) =>
  record?.type === 'folder' ? `/folder/${id}/show` : `/playlist/${id}/show`

const FolderChildrenList = (props) => {
  const record = useRecordContext()
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('folder')

  const toggleableFields = useMemo(
    () => ({
      ownerName: isDesktop && <TextField source="ownerName" />,
      updatedAt: isDesktop && <DateField source="updatedAt" />,
      public: !isXsmall && <TogglePublicInput source="public" />,
    }),
    [isDesktop, isXsmall]
  )

  const columns = useSelectedFields({
    resource: 'folder',
    columns: toggleableFields,
  })

  const parentId = record?.id ?? ''

  const initialPerPage = useMemo(
    () => getStoredPageSize('folderChildren'),
    [],
  )

  return (
    <List
      {...props}
      resource="folder"
      exporter={false}
      title={<Title subTitle={record?.name} />}
      filters={<PlaylistFolderFilter />}
      actions={<PlaylistListActions />}
      bulkActionButtons={!isXsmall && <PlaylistFolderBulkActions />}
      empty={<EmptyPlaylist />}
      perPage={initialPerPage}
      filter={{ parent_id: parentId }}
      filterDefaultValues={{ parent_id: parentId }}
      pagination={<Pagination scope="folderChildren" />}
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

const PlaylistFolderShow = (props) => (
  <ShowBase {...props}>
    <FolderChildrenList {...props} />
  </ShowBase>
)

export default PlaylistFolderShow

