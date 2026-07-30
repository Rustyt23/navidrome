import React from 'react'
import CropIcon from '@material-ui/icons/Crop'
import SilenceTrimList from './SilenceTrimList'

// React Admin's menu clones this value, so it must be a rendered element.
export default {
  list: SilenceTrimList,
  icon: <CropIcon />,
}
