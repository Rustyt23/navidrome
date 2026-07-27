import React from 'react'
import EqualizerIcon from '@material-ui/icons/Equalizer'
import LufsList from './LufsList'

// `icon` must be a rendered element, not a component: the sidebar passes it to
// MenuItemLink, which calls cloneElement() on it.
export default {
  list: LufsList,
  icon: <EqualizerIcon />,
}
