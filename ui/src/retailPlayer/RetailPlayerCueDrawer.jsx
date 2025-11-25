import React, { useCallback, useEffect, useMemo, useState } from 'react'
import Drawer from '@material-ui/core/Drawer'
import { Button, CircularProgress, Divider, Typography } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import httpClient from '../dataProvider/httpClient'

const useStyles = makeStyles((theme) => ({
  drawerPaper: {
    width: 360,
    maxWidth: '100vw',
    padding: theme.spacing(3),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
  },
  headerRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
  },
  cueButtons: {
    display: 'grid',
    gridTemplateColumns: '1fr',
    gap: theme.spacing(1),
  },
  stopButton: {
    alignSelf: 'flex-start',
  },
  helperText: {
    color: theme.palette.text.secondary,
  },
}))

const RetailPlayerCueDrawer = ({ open, onClose, deviceId, onActionComplete }) => {
  const classes = useStyles()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState('')
  const [triggers, setTriggers] = useState([])
  const [isSending, setIsSending] = useState(false)

  const normalizedDeviceId = useMemo(() => (deviceId || '').trim(), [deviceId])

  const loadTriggers = useCallback(() => {
    if (!open || !normalizedDeviceId) {
      return undefined
    }

    const abortController = new AbortController()
    setIsLoading(true)
    setError('')

    httpClient(`/api/retailplayer/triggers?device=${encodeURIComponent(normalizedDeviceId)}`, {
      signal: abortController.signal,
    })
      .then(({ json }) => {
        const loadedTriggers = Array.isArray(json?.triggers) ? json.triggers : []
        setTriggers(loadedTriggers)
      })
      .catch((err) => {
        if (err?.name !== 'AbortError') {
          // eslint-disable-next-line no-console
          console.error('Unable to fetch retail player triggers', err)
          setError('Unable to load cues')
        }
      })
      .finally(() => {
        setIsLoading(false)
      })

    return () => {
      abortController.abort()
    }
  }, [normalizedDeviceId, open])

  useEffect(() => loadTriggers(), [loadTriggers])

  const handleStop = useCallback(() => {
    if (!normalizedDeviceId) {
      return
    }

    setIsSending(true)
    httpClient('/api/retailplayer/stop', {
      method: 'POST',
      headers: new Headers({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ device: normalizedDeviceId }),
    })
      .then(() => {
        if (typeof onActionComplete === 'function') {
          onActionComplete()
        }
      })
      .catch((err) => {
        // eslint-disable-next-line no-console
        console.error('Unable to stop retail player device', err)
      })
      .finally(() => {
        setIsSending(false)
      })
  }, [normalizedDeviceId, onActionComplete])

  const handlePlayCue = useCallback(
    (cueId) => {
      if (!normalizedDeviceId || !cueId) {
        return
      }

      setIsSending(true)
      httpClient('/api/retailplayer/play', {
        method: 'POST',
        headers: new Headers({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({ device: normalizedDeviceId, cue: cueId }),
      })
        .then(() => {
          if (typeof onActionComplete === 'function') {
            onActionComplete()
          }
        })
        .catch((err) => {
          // eslint-disable-next-line no-console
          console.error('Unable to play retail player cue', err)
        })
        .finally(() => {
          setIsSending(false)
        })
    },
    [normalizedDeviceId, onActionComplete]
  )

  const renderBody = () => {
    if (isLoading) {
      return (
        <div aria-live="polite">
          <CircularProgress size={24} />
        </div>
      )
    }

    if (error) {
      return (
        <Typography variant="body2" className={classes.helperText} aria-live="polite">
          {error}
        </Typography>
      )
    }

    if (!triggers.length) {
      return (
        <Typography variant="body2" className={classes.helperText} aria-live="polite">
          No cues available for this device.
        </Typography>
      )
    }

    return (
      <div className={classes.cueButtons}>
        {triggers.map((trigger) => {
          const { id, name, label } = trigger || {}
          const displayName = name || label || id || 'Cue'
          return (
            <Button
              key={id || displayName}
              variant="outlined"
              color="primary"
              onClick={() => handlePlayCue(id)}
              disabled={isSending}
            >
              {displayName}
            </Button>
          )
        })}
      </div>
    )
  }

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      classes={{ paper: classes.drawerPaper }}
      ModalProps={{ keepMounted: true }}
    >
      <div className={classes.headerRow}>
        <Typography variant="h6" component="h2">
          Cues
        </Typography>
        <Button
          onClick={handleStop}
          color="secondary"
          variant="contained"
          className={classes.stopButton}
          disabled={isSending}
        >
          STOP
        </Button>
      </div>

      <Divider />

      {renderBody()}
    </Drawer>
  )
}

RetailPlayerCueDrawer.defaultProps = {
  onClose: undefined,
  onActionComplete: undefined,
}

export default RetailPlayerCueDrawer
