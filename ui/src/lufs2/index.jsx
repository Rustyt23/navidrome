import React from 'react'
import TuneIcon from '@material-ui/icons/Tune'
import Lufs2List from './Lufs2List'

// `icon` must be a rendered element, not a component: the sidebar passes it to
// MenuItemLink, which calls cloneElement() on it.
// Kept out of the sidebar: the exception list only means anything alongside the
// LUFS page, and is reached by the button there. The menu filters on subMenu,
// so a value none of them look for leaves the resource routable but unlisted.
export default {
  list: Lufs2List,
  icon: <TuneIcon />,
  options: { subMenu: 'hidden' },
}
