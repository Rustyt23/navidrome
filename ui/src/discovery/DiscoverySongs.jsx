import React from 'react'
import {
  Datagrid,
  FunctionField,
  ListContextProvider,
  useListContext,
} from 'react-admin'
import { Card, CardContent } from '@material-ui/core'
import { DurationField, SongContextMenu } from '../common'

const DiscoverySongs = (props) => {
  const listContext = useListContext()
  return (
    <ListContextProvider value={listContext}>
      <Card variant="outlined">
        <CardContent>
          <Datagrid rowClick="show" {...props} bulkActionButtons={false}>
            <FunctionField
              label="resources.song.fields.title"
              render={(record) => record?.mediaFile?.title || ''}
            />
            <FunctionField
              label="resources.song.fields.artist"
              render={(record) => record?.mediaFile?.artist || ''}
            />
            <FunctionField
              label="resources.song.fields.album"
              render={(record) => record?.mediaFile?.album || ''}
            />
            <DurationField source="mediaFile.duration" />
            <FunctionField
              label="resources.song.fields.actions"
              render={(record) => (
                <SongContextMenu
                  resource="song"
                  record={{ ...record.mediaFile, playlistId: null }}
                />
              )}
            />
          </Datagrid>
        </CardContent>
      </Card>
    </ListContextProvider>
  )
}

export default DiscoverySongs
