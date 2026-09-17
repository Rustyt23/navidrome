import React from 'react'
import ContentCutIcon from '@material-ui/icons/Crop'
import SilenceList from './SilenceList'

// `icon` must be a rendered element, not a component: the sidebar passes it to
// MenuItemLink, which calls cloneElement() on it.
export default {
  list: SilenceList,
  icon: <ContentCutIcon />,
}
