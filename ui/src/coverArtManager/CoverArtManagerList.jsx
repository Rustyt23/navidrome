import React from 'react'
import { Datagrid, FunctionField, NumberField, TextField, useTranslate } from 'react-admin'
import { DurationField, List, PathField, QualityInfo, SongTitleField } from '../common'

const CoverArtStatusField = (props) => {
  const translate = useTranslate()

  return (
    <FunctionField
      {...props}
      render={(record) =>
        translate(
          record?.hasCoverArt
            ? 'resources.coverArtManager.fields.coverArtAvailable'
            : 'resources.coverArtManager.fields.coverArtMissing',
        )
      }
    />
  )
}

const CoverArtManagerList = (props) => (
  <List
    {...props}
    resource="song"
    sort={{ field: 'title', order: 'ASC' }}
    exporter={false}
    bulkActionButtons={false}
    perPage={50}
  >
    <Datagrid rowClick={false}>
      <SongTitleField source="title" showTrackNumbers={false} />
      <TextField source="artist" />
      <TextField source="album" />
      <NumberField source="trackNumber" />
      <FunctionField source="year" render={(record) => record?.year || ''} />
      <TextField source="genre" />
      <DurationField source="duration" />
      <PathField source="path" />
      <QualityInfo source="quality" sortable={false} />
      <NumberField source="channels" />
      <CoverArtStatusField
        source="coverArtStatus"
        label="resources.coverArtManager.fields.coverArtStatus"
        sortable={false}
      />
    </Datagrid>
  </List>
)

export default CoverArtManagerList
