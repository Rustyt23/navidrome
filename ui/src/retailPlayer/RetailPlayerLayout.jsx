import React, { useState } from 'react'
import PropTypes from 'prop-types'
import clsx from 'clsx'
import {
  Button,
  Collapse,
  Paper,
  Typography,
  makeStyles,
  useMediaQuery,
  useTheme,
} from '@material-ui/core'
import ExpandLessIcon from '@material-ui/icons/ExpandLess'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import { NavLink } from 'react-router-dom'
import { Title } from 'react-admin'

const useStyles = makeStyles((theme) => ({
  root: {
    display: 'flex',
    flexDirection: 'column',
    width: '100%',
    minHeight: '100%',
    backgroundColor: theme.palette.background.default,
  },
  layout: {
    display: 'flex',
    flexGrow: 1,
    width: '100%',
  },
  navDesktop: {
    width: 240,
    flexShrink: 0,
    backgroundColor: theme.palette.background.paper,
    borderRight: `1px solid ${theme.palette.divider}`,
    padding: theme.spacing(2, 0),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
  },
  sectionLabel: {
    padding: theme.spacing(0, 3),
    textTransform: 'uppercase',
    letterSpacing: 1,
    fontSize: 12,
    color: theme.palette.text.secondary,
  },
  navList: {
    listStyle: 'none',
    padding: 0,
    margin: 0,
    display: 'flex',
    flexDirection: 'column',
  },
  navLink: {
    position: 'relative',
    display: 'flex',
    alignItems: 'center',
    padding: theme.spacing(1.5, 3),
    textDecoration: 'none',
    color: theme.palette.text.secondary,
    fontWeight: 500,
    borderLeft: '4px solid transparent',
    transition: 'background-color 0.2s ease, color 0.2s ease, border-left-color 0.2s ease',
    '&:hover': {
      color: theme.palette.text.primary,
      backgroundColor: theme.palette.action.hover,
    },
  },
  navLinkActive: {
    color: theme.palette.text.primary,
    fontWeight: 600,
    borderLeftColor: theme.palette.primary.main,
    backgroundColor: theme.palette.action.selected,
  },
  mobileNavWrapper: {
    padding: theme.spacing(0, 2, 2),
  },
  mobileToggle: {
    width: '100%',
    justifyContent: 'space-between',
  },
  mobileNav: {
    marginTop: theme.spacing(1),
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius,
    border: `1px solid ${theme.palette.divider}`,
  },
  mobileNavList: {
    padding: theme.spacing(1, 0),
  },
  content: {
    flexGrow: 1,
    padding: theme.spacing(3),
    minWidth: 0,
  },
}))

const RetailPlayerLayout = ({ menuItems, children }) => {
  const classes = useStyles()
  const theme = useTheme()
  const isDesktop = useMediaQuery(theme.breakpoints.up('md'))
  const [mobileOpen, setMobileOpen] = useState(false)

  const handleToggle = () => {
    setMobileOpen((open) => !open)
  }

  const handleNavigate = () => {
    setMobileOpen(false)
  }

  const renderNavItems = (itemClassName) => (
    <ul className={clsx(classes.navList, itemClassName)}>
      {menuItems.map((item) => (
        <li key={item.to}>
          <NavLink
            to={item.to}
            exact={item.exact}
            className={classes.navLink}
            activeClassName={classes.navLinkActive}
            onClick={handleNavigate}
          >
            <Typography variant="body1" component="span" noWrap>
              {item.label}
            </Typography>
          </NavLink>
        </li>
      ))}
    </ul>
  )

  return (
    <div className={classes.root}>
      <Title title="Retail Player" />
      <div className={classes.layout}>
        {isDesktop ? (
          <nav className={classes.navDesktop} aria-label="Retail player sections">
            <Typography variant="overline" className={classes.sectionLabel}>
              Retail Player
            </Typography>
            {renderNavItems()}
          </nav>
        ) : (
          <div className={classes.mobileNavWrapper}>
            <Button
              onClick={handleToggle}
              variant="outlined"
              color="default"
              className={classes.mobileToggle}
              endIcon={mobileOpen ? <ExpandLessIcon /> : <ExpandMoreIcon />}
              aria-expanded={mobileOpen}
              aria-controls="retail-player-mobile-nav"
            >
              Retail Player Menu
            </Button>
            <Collapse in={mobileOpen} timeout="auto" unmountOnExit>
              <Paper id="retail-player-mobile-nav" className={classes.mobileNav} elevation={1}>
                {renderNavItems(classes.mobileNavList)}
              </Paper>
            </Collapse>
          </div>
        )}
        <main className={classes.content}>{children}</main>
      </div>
    </div>
  )
}

RetailPlayerLayout.propTypes = {
  menuItems: PropTypes.arrayOf(
    PropTypes.shape({
      label: PropTypes.string.isRequired,
      to: PropTypes.string.isRequired,
      exact: PropTypes.bool,
    }),
  ).isRequired,
  children: PropTypes.node,
}

RetailPlayerLayout.defaultProps = {
  children: null,
}

export default RetailPlayerLayout
