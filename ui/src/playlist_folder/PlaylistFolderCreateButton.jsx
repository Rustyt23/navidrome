import { useState } from 'react'
import { Menu, MenuItem, Button } from '@material-ui/core'
import AddIcon from '@material-ui/icons/Add'
import { useTranslate } from 'react-admin'
import { useHistory } from 'react-router-dom'

const PlaylistFolderCreateButton = ({ recordId = null, resource = 'folder' }) => {
  const translate = useTranslate()
  const history = useHistory()
  const [anchorEl, setAnchorEl] = useState(null)
  const isDiscovery = resource === 'discoveryFolder'

  const open = (e) => setAnchorEl(e.currentTarget)
  const close = () => setAnchorEl(null)

  const goFolder = () => {
    const state = recordId ? { parentId: recordId } : {}
    const pathname = isDiscovery ? '/discoveryFolder/create' : '/folder/create'
    history.push({ pathname, state })
    close()
  }
  const goPlaylist = () => {
    const state = recordId
      ? { [isDiscovery ? 'discoveryFolderId' : 'playlistFolderId']: recordId }
      : {}
    const pathname = isDiscovery ? '/discovery/create' : '/playlist/create'
    history.push({ pathname, state })
    close()
  }

  return (
    <>
      <Button color="primary" onClick={open} startIcon={<AddIcon />}>
        {translate('ra.action.create')}
      </Button>
      <Menu anchorEl={anchorEl} keepMounted open={Boolean(anchorEl)} onClose={close}>
        <MenuItem onClick={goFolder}>
          {translate(`resources.${isDiscovery ? 'discovery' : 'playlist'}.actions.createFolder`)}
        </MenuItem>
        <MenuItem onClick={goPlaylist}>
          {translate(`resources.${isDiscovery ? 'discovery' : 'playlist'}.actions.create`)}
        </MenuItem>
      </Menu>
    </>
  )
}

export default PlaylistFolderCreateButton
