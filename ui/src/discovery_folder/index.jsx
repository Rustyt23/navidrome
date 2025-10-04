import PlaylistFolderList from '../playlist_folder/PlaylistFolderList'
import PlaylistFolderCreate from '../playlist_folder/PlaylistFolderCreate'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import PlaylistFolderEdit from '../playlist_folder/PlaylistFolderEdit'
import PlaylistFolderShow from '../playlist_folder/PlaylistFolderShow'

import LibraryMusicOutlinedIcon from '@material-ui/icons/LibraryMusicOutlined'
import LibraryMusicIcon from '@material-ui/icons/LibraryMusic'

export default {
  list: PlaylistFolderList,
  create: PlaylistFolderCreate,
  edit: PlaylistFolderEdit,
  show: PlaylistFolderShow,
  icon: (
    <DynamicMenuIcon
      path={'discovery/folders'}
      icon={LibraryMusicOutlinedIcon}
      activeIcon={LibraryMusicIcon}
    />
  ),
}
