import React, { useCallback, useEffect, useState } from 'react'
import {
  Card,
  CardContent,
  CardHeader,
  Typography,
  List,
  ListItem,
  ListItemText,
  Button,
  CircularProgress,
  makeStyles,
} from '@material-ui/core'
import CloudDownloadIcon from '@material-ui/icons/CloudDownload'
import RefreshIcon from '@material-ui/icons/Refresh'
import { useNotify, useTranslate } from 'react-admin'
import { useParams } from 'react-router-dom'
import httpClient from '../dataProvider/httpClient'
import { REST_URL } from '../consts'

const useStyles = makeStyles((theme) => ({
  root: {
    maxWidth: 960,
    margin: '24px auto',
  },
  actions: {
    display: 'flex',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(2),
  },
  list: {
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius,
  },
}))

const DiscoveryShow = () => {
  const classes = useStyles()
  const notify = useNotify()
  const translate = useTranslate()
  const { id } = useParams()
  const [record, setRecord] = useState(null)
  const [loading, setLoading] = useState(false)

  const fetchRecord = useCallback(async () => {
    if (!id) return
    setLoading(true)
    try {
      const res = await httpClient(`${REST_URL}/discovery/${id}`)
      setRecord(res?.json || null)
    } catch (error) {
      notify('ra.page.error', 'warning')
    } finally {
      setLoading(false)
    }
  }, [id, notify])

  useEffect(() => {
    fetchRecord()
  }, [fetchRecord])

  const handleExport = () => {
    window.open(`${REST_URL}/discovery/${id}/export`, '_blank')
  }

  const handleRefresh = () => {
    fetchRecord()
  }

  if (loading && !record) {
    return (
      <Card className={classes.root}>
        <CardContent>
          <CircularProgress />
        </CardContent>
      </Card>
    )
  }

  if (!record) {
    return (
      <Card className={classes.root}>
        <CardHeader title={translate('menu.discovery')} />
        <CardContent>
          <Typography color="textSecondary">
            {translate('menu.discovery_empty', { _: translate('ra.message.no_results') })}
          </Typography>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className={classes.root}>
      <CardHeader title={record.name} subheader={translate('resources.playlist.fields.songCount', { smart_count: record.songCount || 0 })} />
      <CardContent>
        <div className={classes.actions}>
          <Button startIcon={<RefreshIcon />} onClick={handleRefresh} disabled={loading} variant="outlined">
            {translate('action.refresh', { _: translate('ra.action.refresh') })}
          </Button>
          <Button startIcon={<CloudDownloadIcon />} onClick={handleExport} variant="contained" color="primary">
            {translate('action.export', { _: translate('ra.action.export') })}
          </Button>
        </div>
        {loading ? <CircularProgress size={24} /> : null}
        <List className={classes.list} dense>
          {(record.tracks || []).map((track, index) => (
            <ListItem key={`${track}-${index}`}>
              <ListItemText primary={track} secondary={`#${index + 1}`} />
            </ListItem>
          ))}
        </List>
      </CardContent>
    </Card>
  )
}

export default DiscoveryShow
