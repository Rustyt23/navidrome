import React, { Fragment, useEffect } from 'react'
import { useUnselectAll } from 'react-admin'
import { playTracks } from '../actions'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import { BatchPlayButton } from './index'
import { AddToPlaylistButton } from './AddToPlaylistButton'
import { makeStyles } from '@material-ui/core/styles'
import { EditSongCommentButton } from './EditSongCommentButton'

const useStyles = makeStyles((theme) => ({
  button: {
    color: theme.palette.type === 'dark' ? 'white' : undefined,
  },
}))

export const SongBulkActions = (props) => {
  const classes = useStyles()
  const unselectAll = useUnselectAll()
  useEffect(() => {
    unselectAll(props.resource)
  }, [unselectAll, props.resource])
  return (
    <Fragment>
      <BatchPlayButton
        {...props}
        action={playTracks}
        label={'resources.song.actions.playNow'}
        icon={<PlayArrowIcon />}
        className={classes.button}
      />
      <AddToPlaylistButton {...props} className={classes.button} />
      <EditSongCommentButton {...props} className={classes.button} />
    </Fragment>
  )
}
