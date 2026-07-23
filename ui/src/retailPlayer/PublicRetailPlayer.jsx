import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ButtonBase, Typography } from '@material-ui/core'
import { alpha, makeStyles } from '@material-ui/core/styles'
import PauseIcon from '@material-ui/icons/Pause'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import { useParams } from 'react-router-dom'
import { baseUrl } from '../utils'
import { normalizeValue } from './deviceUtils'
import useRetailPlayerDeviceStatus from './useRetailPlayerDeviceStatus'

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

// The websocket delivers the active song name as a basename (possibly a path);
// reduce it to a display label as a fallback when metadata has no title.
const streamNameLabel = (songName) =>
  normalizeValue(songName)
    .replace(/\\/g, '/')
    .split('/')
    .pop()
    .replace(/\.[^./\\]+$/, '')

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
  // The song currently loaded for playback, and a second element used to
  // preload the upcoming song so playback never stops until the next one is
  // ready. wantPlayingRef tracks the user's intent across song swaps.
  const currentAudioRef = useRef(null)
  const preloadAudioRef = useRef(null)
  const wantPlayingRef = useRef(false)
  const [currentUrl, setCurrentUrl] = useState('')
  const [isPlaying, setIsPlaying] = useState(false)
  const [streamError, setStreamError] = useState(false)
  // Public stream info for the current song: { streamUrl, url (artwork), ... }.
  const [streamInfo, setStreamInfo] = useState(null)
  // True when the current song genuinely isn't in the library (endpoint 404),
  // as opposed to a transient empty name during a device song change.
  const [songUnavailable, setSongUnavailable] = useState(false)

  const deviceName = useMemo(() => {
    try {
      return decodeURIComponent(deviceSlug || '')
    } catch (error) {
      return deviceSlug || ''
    }
  }, [deviceSlug])

  // Reuse the retail-player device hook the dashboard uses. It owns the
  // remote-control websocket (device lookup, remoteControlId, subscription,
  // realtime merge), so `device.nowPlaying` updates the instant the device
  // changes song — no polling, no /status, no code duplicated here.
  const { device: liveDevice, isLoading: isDeviceLoading } =
    useRetailPlayerDeviceStatus(deviceSlug)
  const nowPlaying = liveDevice?.nowPlaying

  // The websocket's activeStreamName is the song basename (e.g.
  // "Ituana - Tape Loop.mp3"). An anonymous player can't mint a stream URL
  // itself, so hand that name to the public stream-url endpoint, which searches
  // the library and returns a signed /share URL the <audio> element can play.
  const songName = normalizeValue(nowPlaying?.streamName)

  useEffect(() => {
    // Empty name: a transient gap while the device switches songs. Leave the
    // current song and its state untouched so playback continues.
    if (!songName) {
      return undefined
    }

    const controller = new AbortController()
    setStreamError(false)
    fetch(
      baseUrl(
        `/api/retailplayer/stream-url?song=${encodeURIComponent(songName)}`,
      ),
      { signal: controller.signal },
    )
      .then(async (response) => {
        if (response.ok) {
          const data = await response.json()
          setStreamInfo(data?.artwork || null)
          setSongUnavailable(false)
        } else if (response.status === 404) {
          // The song genuinely isn't in the library — mark it so playback stops.
          setStreamInfo(null)
          setSongUnavailable(true)
        }
        // Other statuses (5xx, etc.): treat as transient, keep playing.
      })
      .catch((error) => {
        // Ignore aborts and network blips; keep the current song playing.
        if (error.name !== 'AbortError') {
          // no-op
        }
      })

    return () => {
      controller.abort()
    }
  }, [songName])

  const track = useMemo(
    () => ({
      title:
        normalizeValue(nowPlaying?.title) ||
        streamNameLabel(songName) ||
        'Now Playing',
      artist: normalizeValue(nowPlaying?.artist),
      artworkUrl:
        normalizeValue(nowPlaying?.artworkUrl) ||
        resolvePublicUrl(streamInfo?.url),
      streamUrl: resolvePublicUrl(streamInfo?.streamUrl),
    }),
    [nowPlaying, songName, streamInfo],
  )

  const isLoading = !liveDevice && isDeviceLoading
  const isPlayable = Boolean(currentUrl) && !streamError && !songUnavailable

  // Build an Audio element for a song and wire it to React state. Kept out of
  // the JSX so we can hold two at once (current + preloading) and hand off
  // between them without a DOM src swap.
  const makeAudio = useCallback((streamUrl) => {
    const audio = new Audio()
    audio.preload = 'auto'
    audio.src = streamUrl
    audio.onplay = () => setIsPlaying(true)
    audio.onpause = () => setIsPlaying(false)
    // Natural end: leave wantPlayingRef set so the next song (delivered over the
    // websocket) auto-plays. The small gap here is expected/acceptable.
    audio.onended = () => setIsPlaying(false)
    audio.onerror = () => setStreamError(true)
    return audio
  }, [])

  // Detach handlers before clearing the source: assigning an empty src fires an
  // `error` event, and a live onerror would wrongly flag the *next* song as
  // unavailable while we're just discarding the old element.
  const teardownAudio = useCallback((audio) => {
    if (!audio) {
      return
    }
    audio.onplay = null
    audio.onpause = null
    audio.onended = null
    audio.onerror = null
    audio.pause()
    audio.removeAttribute('src')
    audio.load()
  }, [])

  useEffect(() => {
    // The current song genuinely isn't in the library: stop playback right away
    // and show the unavailable state. This is distinct from a transient empty
    // name (handled below), which must NOT stop the current song.
    if (songUnavailable) {
      teardownAudio(currentAudioRef.current)
      currentAudioRef.current = null
      teardownAudio(preloadAudioRef.current)
      preloadAudioRef.current = null
      setCurrentUrl('')
      // teardownAudio detaches onpause, so reset the playing flag ourselves.
      setIsPlaying(false)
      return undefined
    }

    const desired = track.streamUrl

    // No resolvable stream URL yet but the name is non-empty resolving, or a
    // transient gap between songs (the device empties its active-stream name
    // during a change). Never stop the current song for this: keep it playing
    // so churn in the device state can't create silence. Only surface the empty
    // state when nothing has started playing yet.
    if (!desired) {
      if (!currentAudioRef.current) {
        setCurrentUrl('')
      }
      return undefined
    }

    if (desired === currentUrl) {
      return undefined
    }

    const activateNow = (audio) => {
      const previous = currentAudioRef.current
      if (previous !== audio) {
        teardownAudio(previous)
      }
      currentAudioRef.current = audio
      setCurrentUrl(desired)
      setStreamError(false)
      if (wantPlayingRef.current) {
        audio.play().catch(() => {})
      }
    }

    const current = currentAudioRef.current
    const isActive = current && !current.paused && !current.ended

    // Nothing playing (first load, paused, or the previous song ended): switch
    // right away — there is no ongoing playback to protect.
    if (!isActive) {
      activateNow(makeAudio(desired))
      return undefined
    }

    // A song is mid-playback: preload the next one and only hand off once it can
    // play through, so the current song is never force-stopped into a gap.
    const next = makeAudio(desired)
    preloadAudioRef.current = next
    let swapped = false
    const swap = () => {
      if (swapped) {
        return
      }
      swapped = true
      preloadAudioRef.current = null
      activateNow(next)
    }
    next.addEventListener('canplaythrough', swap, { once: true })
    // If preloading fails, hand off anyway so a bad preload can't wedge playback.
    next.addEventListener('error', swap, { once: true })
    // Safety net: never wait forever for canplaythrough.
    const timeoutId = window.setTimeout(swap, 10000)
    next.load()

    return () => {
      window.clearTimeout(timeoutId)
      if (!swapped) {
        // Guard so a teardown-triggered error event can't invoke swap().
        swapped = true
        teardownAudio(next)
        if (preloadAudioRef.current === next) {
          preloadAudioRef.current = null
        }
      }
    }
  }, [songUnavailable, track.streamUrl, currentUrl, makeAudio, teardownAudio])

  // Tear down audio elements on unmount.
  useEffect(
    () => () => {
      ;[currentAudioRef, preloadAudioRef].forEach((ref) => {
        teardownAudio(ref.current)
        ref.current = null
      })
    },
    [teardownAudio],
  )

  const togglePlayback = useCallback(async () => {
    const audio = currentAudioRef.current
    if (!audio) {
      return
    }

    if (audio.paused) {
      wantPlayingRef.current = true
      try {
        await audio.play()
      } catch (error) {
        setStreamError(true)
      }
      return
    }

    wantPlayingRef.current = false
    audio.pause()
  }, [])

  return (
    <main className={classes.root}>
      <header className={classes.header}>
        <Typography
          component="h1"
          variant="h6"
          className={classes.headerTitle}
          noWrap
        >
          {normalizeValue(liveDevice?.name) || deviceName || 'Retail Player'}
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
        </section>
      )}
    </main>
  )
}

export default PublicRetailPlayer
