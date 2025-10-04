import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Card, CardContent, CardHeader, CircularProgress, List, ListItem, ListItemText, Typography, makeStyles } from '@material-ui/core'
import RefreshIcon from '@material-ui/icons/Refresh'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import { Button, RaRecordContext, RaTitle, TopToolbar, useNotify, useTranslate } from 'react-admin'
import { useParams } from 'react-router-dom'
import httpClient from '../dataProvider/httpClient'
import { M3U_MIME_TYPE, REST_URL } from '../consts'
import { Title } from '../common'

const useStyles = makeStyles((theme) => ({
  root: {
    maxWidth: 960,
    margin: '24px auto',
  },
  actionsToolbar: {
    display: 'flex',
    justifyContent: 'space-between',
    width: '100%',
    marginBottom: theme.spacing(2),
  },
  list: {
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius,
  },
}))

const DiscoveryActions = ({ onRefresh, onExport, refreshing, exporting }) => {
  const translate = useTranslate()
  const classes = useStyles()

  return (
    <TopToolbar className={classes.actionsToolbar}>
      <div>
        <Button
          onClick={onRefresh}
          label={translate('action.refresh', { _: translate('ra.action.refresh') })}
          disabled={refreshing || exporting}
        >
          <RefreshIcon />
        </Button>
        <Button
          onClick={onExport}
          label={translate('resources.playlist.actions.export')}
          disabled={refreshing || exporting}
        >
          <QueueMusicIcon />
        </Button>
      </div>
    </TopToolbar>
  )
}

const DiscoveryShow = () => {
  const classes = useStyles()
  const notify = useNotify()
  const translate = useTranslate()
  const { id } = useParams()
  const [record, setRecord] = useState(null)
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState(false)

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

  const playlistSubtitle = useMemo(() => {
    if (!record) {
      return null
    }
    return translate('resources.playlist.fields.songCount', {
      smart_count: record.songCount || 0,
    })
  }, [record, translate])

  const handleExport = useCallback(() => {
    if (!id) return
    setExporting(true)
    httpClient(`${REST_URL}/discovery/${id}/export`, {
      headers: new Headers({ Accept: M3U_MIME_TYPE }),
    })
      .then((res) => {
        const blob = new Blob([res.body], { type: M3U_MIME_TYPE })
        const url = window.URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `${record?.name || 'discovery'}.m3u`
        document.body.appendChild(link)
        link.click()
        link.parentNode.removeChild(link)
        window.URL.revokeObjectURL(url)
      })
      .catch(() => {
        notify('ra.page.error', 'warning')
      })
      .finally(() => {
        setExporting(false)
      })
  }, [id, notify, record])

  const handleRefresh = useCallback(() => {
    fetchRecord()
  }, [fetchRecord])

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
    <RaRecordContext.Provider value={record}>
      {record && <RaTitle title={<Title subTitle={record.name} />} />}
      <Card className={classes.root}>
        <CardHeader title={record.name} subheader={playlistSubtitle} />
        <CardContent>
          <DiscoveryActions
            onRefresh={handleRefresh}
            onExport={handleExport}
            refreshing={loading}
            exporting={exporting}
          />
          {loading ? <CircularProgress size={24} /> : null}
          <List className={classes.list} dense>
            {(record.tracks || []).map((track, index) => (
              <ListItem key={`${track}-${index}`} divider>
                <ListItemText primary={track} secondary={`#${index + 1}`} />
              </ListItem>
            ))}
          </List>
        </CardContent>
      </Card>
    </RaRecordContext.Provider>
  )
}

export default DiscoveryShow
