import React from 'react'
import PropTypes from 'prop-types'
import List from '@material-ui/core/List'
import ListItem from '@material-ui/core/ListItem'
import ListItemText from '@material-ui/core/ListItemText'
import ListSubheader from '@material-ui/core/ListSubheader'
import { makeStyles } from '@material-ui/core/styles'
import { sanitizeListRestProps, useTranslate } from 'react-admin'
import { setTrack } from '../actions'
import { useDispatch } from 'react-redux'

const useStyles = makeStyles(
  {
    link: {
      textDecoration: 'none',
      color: 'inherit',
    },
    subheader: {
      display: 'grid',
      gridTemplateColumns: '1fr 1fr',
      columnGap: '12px',
      padding: '6px 12px',
      color: '#D1D5DB',
      fontWeight: 600,
      backgroundColor: 'transparent',
    },
    subheaderText: {
      minWidth: 0,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      textAlign: 'left',
    },
    listItem: {
      padding: '6px 12px',
    },
    primary: {
      display: 'grid',
      gridTemplateColumns: '1fr 1fr',
      columnGap: '12px',
      alignItems: 'center',
      width: '100%',
      minWidth: 0,
    },
    title: {
      minWidth: 0,
      fontWeight: 500,
      color: '#FFFFFF',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      textAlign: 'left',
    },
    artist: {
      minWidth: 0,
      color: '#FFFFFF',
      fontWeight: 400,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      textAlign: 'left',
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
  const translate = useTranslate()
  return (
    (loading || total > 0) && (
      <List
        className={className}
        {...sanitizeListRestProps(rest)}
        subheader={
          <ListSubheader component="div" disableSticky className={classes.subheader}>
            <span className={classes.subheaderText}>
              {translate('resources.song.fields.title', { _: 'Title' })}
            </span>
            <span className={classes.subheaderText}>
              {translate('resources.song.fields.artist', { _: 'Artist' })}
            </span>
          </ListSubheader>
        }
      >
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
                        <span
                          className={classes.artist}
                          title={data[id].artist}
                        >
                          {data[id].artist}
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
