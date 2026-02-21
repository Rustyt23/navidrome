import React from 'react'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import PhotoLibraryOutlinedIcon from '@material-ui/icons/PhotoLibraryOutlined'
import PhotoLibraryIcon from '@material-ui/icons/PhotoLibrary'
import CoverArtManagerList from './CoverArtManagerList'

export default {
  list: CoverArtManagerList,
  icon: (
    <DynamicMenuIcon
      path={'coverArtManager'}
      icon={PhotoLibraryOutlinedIcon}
      activeIcon={PhotoLibraryIcon}
    />
  ),
}
