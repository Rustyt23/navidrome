import React from 'react'
import TuneIcon from '@material-ui/icons/Tune'
import Lufs2List from './Lufs2List'

// `icon` must be a rendered element, not a component: the sidebar passes it to
// MenuItemLink, which calls cloneElement() on it.
export default {
  list: Lufs2List,
  icon: <TuneIcon />,
}
