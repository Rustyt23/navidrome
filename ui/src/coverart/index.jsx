import React from 'react'
import ImageOutlinedIcon from '@material-ui/icons/ImageOutlined'
import ImageIcon from '@material-ui/icons/Image'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import CoverArtPage from './CoverArtPage'

export default {
  list: CoverArtPage,
  icon: (
    <DynamicMenuIcon
      path={'coverart'}
      icon={ImageOutlinedIcon}
      activeIcon={ImageIcon}
    />
  ),
}
