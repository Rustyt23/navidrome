import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ButtonBase, Typography } from '@material-ui/core'
import { alpha, makeStyles } from '@material-ui/core/styles'
import PauseIcon from '@material-ui/icons/Pause'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import { useParams } from 'react-router-dom'
import { baseUrl } from '../utils'
import { normalizeValue } from './deviceUtils'

const isLoopbackHost = (hostname) => {
  const normalized = hostname.toLowerCase()
  return (
    normalized === 'localhost' ||
    normalized === '::1' ||
    normalized.startsWith('127.')
  )
}

const resolvePublicUrl = (value) => {
  const normalized = normalizeValue(value)
  if (!normalized || typeof window === 'undefined') {
    return normalized
  }

  try {
    const publicUrl = new URL(normalized, window.location.href)
    if (isLoopbackHost(publicUrl.hostname)) {
      publicUrl.hostname = window.location.hostname
    }
    return publicUrl.toString()
  } catch (error) {
    return normalized
  }
}

const getCurrentTrack = (payload) => {
  const status =
    payload?.status && typeof payload.status === 'object' ? payload.status : {}
  const metadata = Array.isArray(payload?.streamMetadata)
    ? payload.streamMetadata
    : []
  const activeResource = normalizeValue(status.activeResource).toLowerCase()
  const activeStreamName = normalizeValue(status.activeStreamName)
  const streamName = normalizeValue(
    status.activeStreamName || status.activeStream,
  )
  const currentMetadata =
    metadata.find((item) => {
      const itemResource = normalizeValue(item?.activeResource).toLowerCase()
      const itemChannel = normalizeValue(item?.channelName)
      const itemFilename = normalizeValue(item?.filename)

      return (
        (activeResource && itemResource === activeResource) ||
        (activeStreamName && itemChannel === activeStreamName) ||
        (streamName && itemFilename === streamName)
      )
    }) ||
    metadata.find((item) => item?.metadata?.title || item?.metadata?.artist)
  const details = currentMetadata?.metadata || {}
  const streamLabel = streamName.replace(/\\/g, '/').split('/').pop() || ''
  const title =
    normalizeValue(details.trackTitle || details.title) ||
    normalizeValue(status.trackTitle || status.streamTitle) ||
    streamLabel.replace(/\.[^./\\]+$/, '') ||
    'Now Playing'
  const artist =
    normalizeValue(details.trackArtist || details.artist) ||
    normalizeValue(status.trackArtist || status.artist)

  return {
    title,
    artist,
    artworkUrl: resolvePublicUrl(payload?.artwork?.url),
    streamUrl: resolvePublicUrl(payload?.artwork?.streamUrl),
  }
}

const useStyles = makeStyles((theme) => {
  const accent =
    (theme.palette.secondary && theme.palette.secondary.main) || '#ff6f9f'
  const paper =
    theme.palette.background.paper || theme.palette.background.default

  return {
    root: {
      minHeight: '100vh',
      padding: theme.spacing(3),
      boxSizing: 'border-box',
      background: theme.palette.background.default,
      color: theme.palette.text.primary,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      [theme.breakpoints.down('xs')]: { padding: theme.spacing(2) },
    },
    header: {
      width: '100%',
      maxWidth: 780,
      padding: `${theme.spacing(1.5)}px ${theme.spacing(2)}px`,
      boxSizing: 'border-box',
      textAlign: 'center',
      borderBottom: `1px solid ${alpha(theme.palette.common.white, 0.12)}`,
    },
    headerTitle: { fontWeight: theme.typography.fontWeightBold },
    card: {
      width: '100%',
      maxWidth: 780,
      marginTop: theme.spacing(5),
      padding: theme.spacing(4),
      boxSizing: 'border-box',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      borderRadius: theme.shape.borderRadius * 3,
      background: alpha(paper, 0.82),
      boxShadow: '0 16px 45px rgba(0, 0, 0, 0.28)',
      [theme.breakpoints.down('xs')]: {
        marginTop: theme.spacing(3),
        padding: theme.spacing(3),
      },
    },
    artwork: {
      width: 280,
      height: 280,
      borderRadius: '50%',
      overflow: 'hidden',
      background: `radial-gradient(circle at center, ${alpha(accent, 0.86)} 0 8%, #242424 9% 31%, #111 32% 100%)`,
      boxShadow: `0 0 0 9px ${alpha(theme.palette.common.white, 0.08)}, 0 18px 32px rgba(0, 0, 0, 0.38)`,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      [theme.breakpoints.down('xs')]: { width: 220, height: 220 },
    },
    artworkImage: { width: '100%', height: '100%', objectFit: 'cover' },
    trackInfo: {
      width: '100%',
      marginTop: theme.spacing(4),
      textAlign: 'center',
    },
    trackTitle: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(28),
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    trackArtist: {
      marginTop: theme.spacing(0.75),
      color: theme.palette.text.secondary,
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
    },
    playButton: {
      width: 68,
      height: 68,
      marginTop: theme.spacing(3),
      borderRadius: '50%',
      background: accent,
      color: theme.palette.getContrastText(accent),
      boxShadow: `0 8px 18px ${alpha(accent, 0.35)}`,
      '&:hover, &:focus-visible': { background: alpha(accent, 0.86) },
      '&.Mui-disabled': {
        color: theme.palette.action.disabled,
        background: theme.palette.action.disabledBackground,
        boxShadow: 'none',
      },
      '& > svg': { fontSize: theme.typography.pxToRem(38) },
    },
    unavailable: {
      marginTop: theme.spacing(2),
      color: theme.palette.text.secondary,
    },
    loading: {
      marginTop: theme.spacing(5),
      color: theme.palette.text.secondary,
    },
  }
})

const PublicRetailPlayer = () => {
  const classes = useStyles()
  const { deviceSlug } = useParams()
  const audioRef = useRef(null)
  const [payload, setPayload] = useState(null)
  const [isLoading, setIsLoading] = useState(true)
  const [isPlaying, setIsPlaying] = useState(false)
  const [streamError, setStreamError] = useState(false)

  const deviceName = useMemo(() => {
    try {
      return decodeURIComponent(deviceSlug || '')
    } catch (error) {
      return deviceSlug || ''
    }
  }, [deviceSlug])

  const loadStatus = useCallback(
    async (signal) => {
      if (!deviceName) {
        return
      }

      try {
        const response = await fetch(
          baseUrl(
            `/api/retailplayer/devices/${encodeURIComponent(deviceName)}/status`,
          ),
          { signal },
        )
        if (!response.ok) {
          throw new Error(`Unable to load player status (${response.status})`)
        }
        setPayload(await response.json())
        setStreamError(false)
      } catch (error) {
        if (error.name !== 'AbortError') {
          setPayload(null)
        }
      } finally {
        if (!signal.aborted) {
          setIsLoading(false)
        }
      }
    },
    [deviceName],
  )

  useEffect(() => {
    const controller = new AbortController()
    setIsLoading(true)
    loadStatus(controller.signal)
    const intervalId = window.setInterval(
      () => loadStatus(controller.signal),
      10000,
    )

    return () => {
      controller.abort()
      window.clearInterval(intervalId)
    }
  }, [loadStatus])

  const track = useMemo(() => getCurrentTrack(payload), [payload])
  const isPlayable = Boolean(track.streamUrl) && !streamError

  useEffect(() => {
    const audio = audioRef.current
    if (!audio) {
      return
    }

    audio.pause()
    setIsPlaying(false)
  }, [track.streamUrl])

  const togglePlayback = useCallback(async () => {
    const audio = audioRef.current
    if (!audio || !isPlayable) {
      return
    }

    if (audio.paused) {
      try {
        await audio.play()
        setIsPlaying(true)
      } catch (error) {
        setStreamError(true)
      }
      return
    }

    audio.pause()
    setIsPlaying(false)
  }, [isPlayable])

  return (
    <main className={classes.root}>
      <header className={classes.header}>
        <Typography
          component="h1"
          variant="h6"
          className={classes.headerTitle}
          noWrap
        >
          {payload?.device?.name || deviceName || 'Retail Player'}
        </Typography>
      </header>

      {isLoading ? (
        <Typography className={classes.loading}>
          Loading now playing…
        </Typography>
      ) : (
        <section className={classes.card} aria-label="Now playing">
          <div className={classes.artwork}>
            {track.artworkUrl ? (
              <img
                className={classes.artworkImage}
                src={track.artworkUrl}
                alt={`Artwork for ${track.title}`}
              />
            ) : null}
          </div>
          <div className={classes.trackInfo}>
            <Typography
              component="h2"
              className={classes.trackTitle}
              title={track.title}
            >
              {track.title}
            </Typography>
            {track.artist ? (
              <Typography className={classes.trackArtist} title={track.artist}>
                {track.artist}
              </Typography>
            ) : null}
          </div>
          <ButtonBase
            className={classes.playButton}
            aria-label={isPlaying ? 'Pause' : 'Play'}
            onClick={togglePlayback}
            disabled={!isPlayable}
            focusRipple
          >
            {isPlaying ? <PauseIcon /> : <PlayArrowIcon />}
          </ButtonBase>
          {!isPlayable ? (
            <Typography className={classes.unavailable} variant="body2">
              This song is not available in Musicmatters.
            </Typography>
          ) : null}
          <audio
            ref={audioRef}
            src={track.streamUrl || undefined}
            onPause={() => setIsPlaying(false)}
            onPlay={() => setIsPlaying(true)}
            onEnded={() => setIsPlaying(false)}
            onError={() => setStreamError(true)}
          />
        </section>
      )}
    </main>
  )
}

export default PublicRetailPlayer
