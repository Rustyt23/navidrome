import DiscoveryFolderList from '../discovery/DiscoveryFolderList'
import PlaylistFolderCreate from '../playlist_folder/PlaylistFolderCreate'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import PlaylistFolderEdit from '../playlist_folder/PlaylistFolderEdit'
import DiscoveryFolderShow from '../discovery/DiscoveryFolderShow'

import LibraryMusicOutlinedIcon from '@material-ui/icons/LibraryMusicOutlined'
import LibraryMusicIcon from '@material-ui/icons/LibraryMusic'

export default {
  list: DiscoveryFolderList,
  create: PlaylistFolderCreate,
  edit: PlaylistFolderEdit,
  show: DiscoveryFolderShow,
  icon: (
    <DynamicMenuIcon
      path={'discovery/folders'}
      icon={LibraryMusicOutlinedIcon}
      activeIcon={LibraryMusicIcon}
    />
  ),
}
