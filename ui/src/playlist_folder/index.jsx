import PlaylisFolderList from './PlaylistFolderList'
import PlaylistFolderCreate from './PlaylistFolderCreate'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import PlaylistFolderEdit from './PlaylistFolderEdit'
import PlaylistFolderShow from './PlaylistFolderShow'

import LibraryMusicOutlinedIcon from '@material-ui/icons/LibraryMusicOutlined'
import LibraryMusicIcon from '@material-ui/icons/LibraryMusic'

export default {
  list: PlaylisFolderList,
  create: PlaylistFolderCreate,
  edit: PlaylistFolderEdit,
  show: PlaylistFolderShow,
  icon: (
    <DynamicMenuIcon
      path={'folders'}
      icon={LibraryMusicOutlinedIcon}
      activeIcon={LibraryMusicIcon}
    />
  ),
}
