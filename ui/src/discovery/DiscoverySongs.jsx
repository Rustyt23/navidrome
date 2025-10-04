import React from 'react'
import { Datagrid, ListContextProvider, TextField, useListContext } from 'react-admin'
import { Card, CardContent } from '@material-ui/core'
import { DurationField } from '../common'

const DiscoverySongs = (props) => {
  const listContext = useListContext()
  return (
    <ListContextProvider value={listContext}>
      <Card variant="outlined">
        <CardContent>
          <Datagrid rowClick="show" {...props} bulkActionButtons={false}>
            <TextField source="title" label="resources.song.fields.title" />
            <TextField source="artist" label="resources.song.fields.artist" />
            <TextField source="album" label="resources.song.fields.album" />
            <DurationField source="duration" />
            <TextField source="path" label="resources.song.fields.path" />
          </Datagrid>
        </CardContent>
      </Card>
    </ListContextProvider>
  )
}

export default DiscoverySongs
