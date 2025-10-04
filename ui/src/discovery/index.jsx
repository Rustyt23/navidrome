import React from 'react'
import QueueMusicOutlinedIcon from '@material-ui/icons/QueueMusicOutlined'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import PlaylistList from '../playlist/PlaylistList'
import PlaylistEdit from '../playlist/PlaylistEdit'
import PlaylistCreate from '../playlist/PlaylistCreate'
import PlaylistShow from '../playlist/PlaylistShow'

export default {
  list: PlaylistList,
  create: PlaylistCreate,
  edit: PlaylistEdit,
  show: PlaylistShow,
  icon: (
    <DynamicMenuIcon
      path={'discovery'}
      icon={QueueMusicOutlinedIcon}
      activeIcon={QueueMusicIcon}
    />
  ),
}
