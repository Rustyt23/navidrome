import { Box, Button, makeStyles } from '@material-ui/core'
import ExtensionIcon from '@material-ui/icons/Extension'
import DashboardOutlinedIcon from '@material-ui/icons/DashboardOutlined'
import PlaylistPlayIcon from '@material-ui/icons/PlaylistPlay'
import { useHistory, useLocation } from 'react-router-dom'

const useStyles = makeStyles((theme) => ({
  tabs: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(2),
  },
  tab: {
    padding: theme.spacing(0.5, 1.75),
    borderRadius: 999,
    border: '1px solid rgba(255, 42, 142, 0.35)',
    color: theme.palette.text.secondary,
    fontSize: 13,
    fontWeight: 600,
    textTransform: 'none',
    '& .MuiButton-startIcon svg': {
      fontSize: 18,
    },
    '&:hover': {
      borderColor: '#ff2a8e',
      background: 'rgba(255, 42, 142, 0.08)',
    },
  },
  activeTab: {
    color: '#ff2a8e',
    borderColor: '#ff2a8e',
    background: 'rgba(255, 42, 142, 0.14)',
  },
}))

const tabs = [
  { to: '/ai-tool', label: 'Ai-Matters', icon: <ExtensionIcon /> },
  {
    to: '/ai-dashboard',
    label: 'AI Dashboard',
    icon: <DashboardOutlinedIcon />,
  },
  {
    to: '/playlist-ai-tool',
    label: 'Playlist AI Tool',
    icon: <PlaylistPlayIcon />,
  },
]

const AiToolNavTabs = () => {
  const classes = useStyles()
  const history = useHistory()
  const location = useLocation()

  return (
    <Box className={classes.tabs}>
      {tabs.map((tab) => {
        const active = location.pathname === tab.to
        return (
          <Button
            key={tab.to}
            size="small"
            className={`${classes.tab} ${active ? classes.activeTab : ''}`}
            startIcon={tab.icon}
            aria-current={active ? 'page' : undefined}
            onClick={() => history.push(tab.to)}
          >
            {tab.label}
          </Button>
        )
      })}
    </Box>
  )
}

export default AiToolNavTabs
