import { useState } from 'react'
import { Menu, MenuItem, Button } from '@material-ui/core'
import AddIcon from '@material-ui/icons/Add'
import { useTranslate } from 'react-admin'
import { useHistory } from 'react-router-dom'

const PlaylistFolderCreateButton = ({ recordId = null }) => {
  const translate = useTranslate()
  const history = useHistory()
  const [anchorEl, setAnchorEl] = useState(null)

  const open = (e) => setAnchorEl(e.currentTarget)
  const close = () => setAnchorEl(null)

  const goFolder = () => {
    const state = recordId ? { parentId: recordId } : {}
    history.push({ pathname: '/folder/create', state })
    close()
  }
  const goPlaylist = () => {
    const state = recordId ? { playlistFolderId: recordId } : {}
    history.push({ pathname: '/playlist/create', state })
    close()
  }

  return (
    <>
      <Button color="primary" onClick={open} startIcon={<AddIcon />}>
        {translate('ra.action.create')}
      </Button>
      <Menu anchorEl={anchorEl} keepMounted open={Boolean(anchorEl)} onClose={close}>
        <MenuItem onClick={goFolder}>
          {translate('resources.playlist.actions.createFolder')}
        </MenuItem>
        <MenuItem onClick={goPlaylist}>
          {translate('resources.playlist.actions.create')}
        </MenuItem>
      </Menu>
    </>
  )
}

export default PlaylistFolderCreateButton
