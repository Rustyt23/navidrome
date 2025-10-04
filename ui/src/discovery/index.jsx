import QueueMusicOutlinedIcon from '@material-ui/icons/QueueMusicOutlined'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import DiscoveryList from './DiscoveryList'
import DiscoveryShow from './DiscoveryShow'

export default {
  list: DiscoveryList,
  show: DiscoveryShow,
  icon: (
    <DynamicMenuIcon
      path={'discovery'}
      icon={QueueMusicOutlinedIcon}
      activeIcon={QueueMusicIcon}
    />
  ),
}
