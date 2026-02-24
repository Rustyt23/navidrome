import React from 'react'
import ImageOutlinedIcon from '@material-ui/icons/ImageOutlined'
import ImageIcon from '@material-ui/icons/Image'
import DynamicMenuIcon from '../layout/DynamicMenuIcon'
import CovertartList from './CovertartList'

export default {
  list: CovertartList,
  icon: (
    <DynamicMenuIcon
      path={'covertart'}
      icon={ImageOutlinedIcon}
      activeIcon={ImageIcon}
    />
  ),
}
