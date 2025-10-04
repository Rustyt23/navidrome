import React from 'react'
import ExploreOutlinedIcon from '@material-ui/icons/ExploreOutlined'
import ExploreIcon from '@material-ui/icons/Explore'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import DiscoveryList from './DiscoveryList'
import DiscoveryShow from './DiscoveryShow'

export default {
  list: DiscoveryList,
  show: DiscoveryShow,
  icon: (
    <DynamicMenuIcon
      path={'discovery'}
      icon={ExploreOutlinedIcon}
      activeIcon={ExploreIcon}
    />
  ),
}
