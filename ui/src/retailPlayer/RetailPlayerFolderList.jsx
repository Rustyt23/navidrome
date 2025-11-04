import { useCallback } from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  List,
  SearchInput,
  TextField,
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'

const RetailPlayerFolderFilter = (props) => (
  <Filter {...props} variant="outlined">
    <SearchInput source="q" alwaysOn />
  </Filter>
)

const RetailPlayerFolderList = (props) => {
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  const rowClick = useCallback((id, record) => {
    const slug = record?.slug || record?.id
    if (!slug) {
      return false
    }
    return `/retailplayer/${encodeURIComponent(slug)}`
  }, [])

  return (
    <List
      {...props}
      resource="retailplayerDevice"
      basePath="/retailplayer/folder"
      title="RetailPlayer - Devices"
      exporter={false}
      filters={<RetailPlayerFolderFilter />}
      actions={false}
      bulkActionButtons={false}
      perPage={isXsmall ? 50 : 50}
      sort={{ field: 'name', order: 'ASC' }}
    >
      <Datagrid rowClick={rowClick}>
        <TextField source="name" label="Name" />
        <TextField source="channel" label="Channel" />
        <TextField source="channelList" label="Channel List" />
        {isDesktop && <TextField source="organization" label="Organization" />}
        {isDesktop ? (
          <TextField source="timeZone" label="Time Zone" />
        ) : (
          <FunctionField
            label="Details"
            render={(record) =>
              [record?.organization, record?.timeZone]
                .filter(Boolean)
                .join(' \u2022 ')
            }
          />
        )}
      </Datagrid>
    </List>
  )
}

export default RetailPlayerFolderList
