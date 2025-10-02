import { useRecordContext } from 'react-admin'
import { Box, useTheme } from '@material-ui/core'
import { RiFolder3Fill, RiMusic2Fill } from 'react-icons/ri'

const DiscoveryTypeIconField = ({ sx }) => {
  const record = useRecordContext()
  const theme = useTheme()

  if (!record) return null

  const isFolder = record.type === 'folder'
  const Icon = isFolder ? RiFolder3Fill : RiMusic2Fill
  const color = isFolder
    ? theme.palette.primary.main
    : theme.palette.secondary.main

  return (
    <Box
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        width: '100%',
        ...(sx || {}),
      }}
      aria-label={isFolder ? 'Folder' : 'File'}
    >
      <Icon style={{ fontSize: 18, color }} />
    </Box>
  )
}

export default DiscoveryTypeIconField
