import React from 'react'
import TravelExploreOutlinedIcon from '@material-ui/icons/TravelExplore'
import TravelExploreIcon from '@material-ui/icons/TravelExplore'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import DiscoveryList from './DiscoveryList'
import DiscoveryShow from './DiscoveryShow'

export default {
  list: DiscoveryList,
  show: DiscoveryShow,
  icon: (
    <DynamicMenuIcon
      path={'discovery'}
      icon={TravelExploreOutlinedIcon}
      activeIcon={TravelExploreIcon}
    />
  ),
}
