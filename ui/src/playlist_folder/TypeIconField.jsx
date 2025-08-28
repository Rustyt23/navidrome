import { useRecordContext } from 'react-admin'
import { RiFolder3Fill, RiPlayListFill } from 'react-icons/ri'
import { Box, useTheme } from '@material-ui/core'

const TypeIconField = ({ sx }) => {
  const record = useRecordContext()
  const theme = useTheme()
  if (!record) return null

  const isFolder = record.type === 'folder'
  const Icon = isFolder ? RiFolder3Fill : RiPlayListFill
  const color = isFolder ? theme.palette.primary.main : theme.palette.secondary.main

  return (
    <Box
      style={{
        display: 'flex', justifyContent: 'center', alignItems: 'center', width: '100%',
        ...(sx || {}),
      }}
      aria-label={isFolder ? 'Folder' : 'Playlist'}
    >
      <Icon style={{ fontSize: 18, color }} />
    </Box>
  )
}

export default TypeIconField
