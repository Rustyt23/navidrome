import React from 'react'
import PropTypes from 'prop-types'
import List from '@material-ui/core/List'
import ListItem from '@material-ui/core/ListItem'
import ListItemIcon from '@material-ui/core/ListItemIcon'
import ListItemSecondaryAction from '@material-ui/core/ListItemSecondaryAction'
import ListItemText from '@material-ui/core/ListItemText'
import { makeStyles } from '@material-ui/core/styles'
import { sanitizeListRestProps } from 'react-admin'
import { SongContextMenu } from './index'
import { setTrack } from '../actions'
import { useDispatch } from 'react-redux'

const useStyles = makeStyles(
  {
    link: {
      textDecoration: 'none',
      color: 'inherit',
    },
    listItem: {
      padding: '12px 16px',
    },
    primary: {
      display: 'flex',
      alignItems: 'center',
      width: '100%',
      minWidth: 0,
      gap: '6px',
    },
    title: {
      flex: '1 1 auto',
      minWidth: 0,
      fontWeight: 500,
      color: '#fff',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    separator: {
      flex: '0 0 auto',
      color: 'rgba(255, 255, 255, 0.6)',
    },
    artist: {
      flex: '1 1 auto',
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      color: 'rgba(255, 255, 255, 0.65)',
      fontWeight: 400,
    },
    rightIcon: {
      top: '50%',
      transform: 'translateY(-50%)',
    },
  },
  { name: 'RaSongSimpleList' },
)

export const SongSimpleList = ({
  basePath,
  className,
  classes: classesOverride,
  data,
  hasBulkActions,
  ids,
  loading,
  onToggleItem,
  selectedIds,
  total,
  ...rest
}) => {
  const dispatch = useDispatch()
  const classes = useStyles({ classes: classesOverride })
  return (
    (loading || total > 0) && (
      <List className={className} {...sanitizeListRestProps(rest)}>
        {ids.map(
          (id) =>
            data[id] && (
              <span key={id} onClick={() => dispatch(setTrack(data[id]))}>
                <ListItem className={classes.listItem} button={true}>
                  <ListItemText
                    disableTypography
                    primary={
                      <div className={classes.primary}>
                        <span
                          className={classes.title}
                          title={data[id].title}
                        >
                          {data[id].title}
                        </span>
                        <span className={classes.separator}>—</span>
                        <span
                          className={classes.artist}
                          title={data[id].artist}
                        >
                          {data[id].artist}
                        </span>
                      </div>
                    }
                  />
                  <ListItemSecondaryAction className={classes.rightIcon}>
                    <ListItemIcon>
                      <SongContextMenu record={data[id]} visible={true} />
                    </ListItemIcon>
                  </ListItemSecondaryAction>
                </ListItem>
              </span>
            ),
        )}
      </List>
    )
  )
}

SongSimpleList.propTypes = {
  basePath: PropTypes.string,
  className: PropTypes.string,
  classes: PropTypes.object,
  data: PropTypes.object,
  hasBulkActions: PropTypes.bool.isRequired,
  ids: PropTypes.array,
  onToggleItem: PropTypes.func,
  selectedIds: PropTypes.arrayOf(PropTypes.any).isRequired,
}

SongSimpleList.defaultProps = {
  hasBulkActions: false,
  selectedIds: [],
}
