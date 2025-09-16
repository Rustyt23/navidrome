import React from 'react'
import PropTypes from 'prop-types'
import List from '@material-ui/core/List'
import ListItem from '@material-ui/core/ListItem'
import ListItemText from '@material-ui/core/ListItemText'
import { makeStyles } from '@material-ui/core/styles'
import { sanitizeListRestProps } from 'react-admin'
import { setTrack } from '../actions'
import { useDispatch } from 'react-redux'

const useStyles = makeStyles(
  {
    link: {
      textDecoration: 'none',
      color: 'inherit',
    },
    listItem: {
      padding: '6px 12px',
    },
    primary: {
      display: 'flex',
      alignItems: 'center',
      width: '100%',
      minWidth: 0,
      gap: '12px',
    },
    title: {
      flex: '1 1 50%',
      minWidth: 0,
      fontWeight: 500,
      color: '#FFFFFF',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    artist: {
      flex: '1 1 50%',
      minWidth: 0,
      display: 'flex',
      justifyContent: 'flex-end',
      overflow: 'hidden',
    },
    artistText: {
      maxWidth: '100%',
      color: '#FFFFFF',
      fontWeight: 400,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      textAlign: 'right',
      direction: 'ltr',
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
                        <span className={classes.artist}>
                          <span
                            className={classes.artistText}
                            title={data[id].artist}
                          >
                            {data[id].artist}
                          </span>
                        </span>
                      </div>
                    }
                  />
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
