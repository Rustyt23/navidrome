import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { useMediaQuery } from '@material-ui/core'
import { ThemeProvider } from '@material-ui/core/styles'
import {
  createMuiTheme,
  useAuthState,
  useDataProvider,
  useTranslate,
} from 'react-admin'
import ReactGA from 'react-ga'
import { GlobalHotKeys } from 'react-hotkeys'
import ReactJkMusicPlayer from 'navidrome-music-player'
import 'navidrome-music-player/assets/index.css'
import { MdSkipNext, MdSkipPrevious } from 'react-icons/md'
import useCurrentTheme from '../themes/useCurrentTheme'
import config from '../config'
import useStyle from './styles'
import AudioTitle from './AudioTitle'
import {
  addTracks,
  clearQueue,
  currentPlaying,
  setPlayMode,
  setVolume,
  syncQueue,
} from '../actions'
import PlayerToolbar from './PlayerToolbar'
import TransportControlButton from './TransportControlButton'
import { sendNotification } from '../utils'
import subsonic from '../subsonic'
import locale from './locale'
import { keyMap } from '../hotkeys'
import keyHandlers from './keyHandlers'
import { calculateGain } from '../utils/calculateReplayGain'

const HOTKEY_SUPPRESS_DURATION = 400
const PREFETCH_THRESHOLD = 5

const Player = () => {
  const theme = useCurrentTheme()
  const translate = useTranslate()
  const playerTheme = theme.player?.theme || 'dark'
  const dataProvider = useDataProvider()
  const playerState = useSelector((state) => state.player)
  const dispatch = useDispatch()
  const [startTime, setStartTime] = useState(null)
  const [scrobbled, setScrobbled] = useState(false)
  const [preloaded, setPreload] = useState(false)
  const [audioInstance, setAudioInstance] = useState(null)
  const isDesktop = useMediaQuery('(min-width:810px)')
  const isMobilePlayer =
    /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
      navigator.userAgent,
    )

  const { authenticated } = useAuthState()
  const visible = authenticated && playerState.queue.length > 0
  const isRadio = playerState.current?.isRadio || false
  const classes = useStyle({
    isRadio,
    visible,
    enableCoverAnimation: config.enableCoverAnimation,
  })
  const showNotifications = useSelector(
    (state) => state.settings.notifications || false,
  )
  const gainInfo = useSelector((state) => state.replayGain)
  const [context, setContext] = useState(null)
  const [gainNode, setGainNode] = useState(null)
  const hotkeyTimeoutRef = useRef()
  const [suppressHotkeys, setSuppressHotkeys] = useState(false)
  const latestPlayerStateRef = useRef(playerState)
  const queuePrefetchRef = useRef({
    sessionId: null,
    pendingPromise: null,
    loadingPage: null,
  })
  const initialPrefetchSessionRef = useRef(null)

  const suppressHotkeysTemporarily = useCallback(() => {
    setSuppressHotkeys(true)
    if (hotkeyTimeoutRef.current) {
      clearTimeout(hotkeyTimeoutRef.current)
    }
    const timeoutId = setTimeout(() => {
      setSuppressHotkeys(false)
      hotkeyTimeoutRef.current = undefined
    }, HOTKEY_SUPPRESS_DURATION)
    hotkeyTimeoutRef.current = timeoutId
  }, [])

  useEffect(() => {
    return () => {
      if (hotkeyTimeoutRef.current) {
        clearTimeout(hotkeyTimeoutRef.current)
      }
    }
  }, [])

  useEffect(() => {
    latestPlayerStateRef.current = playerState
  }, [playerState])

  useEffect(() => {
    if (
      context === null &&
      audioInstance &&
      config.enableReplayGain &&
      'AudioContext' in window &&
      (gainInfo.gainMode === 'album' || gainInfo.gainMode === 'track')
    ) {
      const ctx = new AudioContext()
      // we need this to support radios in firefox
      audioInstance.crossOrigin = 'anonymous'
      const source = ctx.createMediaElementSource(audioInstance)
      const gain = ctx.createGain()

      source.connect(gain)
      gain.connect(ctx.destination)

      setContext(ctx)
      setGainNode(gain)
    }
  }, [audioInstance, context, gainInfo.gainMode])

  useEffect(() => {
    if (gainNode) {
      const current = playerState.current || {}
      const song = current.song || {}

      const numericGain = calculateGain(gainInfo, song)
      gainNode.gain.setValueAtTime(numericGain, context.currentTime)
    }
  }, [audioInstance, context, gainNode, playerState, gainInfo])

  const computeNextPage = useCallback((meta) => {
    if (!meta || meta.endReached) {
      return null
    }
    const perPage = meta.pagination?.perPage || 1
    const loadedPages = meta.pagination?.loadedPages || []
    let maxLoaded = loadedPages.length
      ? Math.max(...loadedPages)
      : meta.pagination?.page || 1
    if (typeof meta.pagination?.total === 'number') {
      const totalPages = Math.ceil(meta.pagination.total / perPage)
      if (maxLoaded >= totalPages) {
        return null
      }
    }
    if (!loadedPages.length && meta.pagination?.page) {
      maxLoaded = meta.pagination.page
    }
    return maxLoaded + 1
  }, [])

  const appendPrefetchedPage = useCallback(
    (page, response, metaAtRequest) => {
      const latest = latestPlayerStateRef.current
      const meta = latest.queueMeta
      if (!meta || meta.context !== 'songsList') {
        return
      }
      if (meta.sessionId !== metaAtRequest.sessionId) {
        return
      }
      const existingLoaded = meta.pagination?.loadedPages || []
      if (existingLoaded.includes(page)) {
        return
      }
      const queue = latest.queue || []
      const existingTrackIds = new Set(queue.map((item) => item.trackId))
      const rawRecords = (response && response.data) || []
      const entries = {}
      const ids = []
      rawRecords.forEach((record) => {
        if (!record || record.missing) {
          return
        }
        const recordId = record.id
        const trackId = record.mediaFileId || record.id
        if (!recordId || !trackId || existingTrackIds.has(trackId)) {
          return
        }
        entries[recordId] = { ...record }
        ids.push(recordId)
        existingTrackIds.add(trackId)
      })

      const perPage =
        meta.pagination?.perPage ||
        metaAtRequest.pagination?.perPage ||
        (rawRecords.length > 0 ? rawRecords.length : 1)
      const responseTotal =
        typeof response?.total === 'number' ? response.total : null
      const newTotal =
        responseTotal ??
        meta.pagination?.total ??
        metaAtRequest.pagination?.total ??
        null
      const updatedLoadedPages = Array.from(
        new Set([...(meta.pagination?.loadedPages || []), page]),
      ).sort((a, b) => a - b)
      const maxLoaded =
        updatedLoadedPages[updatedLoadedPages.length - 1] || page
      let endReached = meta.endReached || false
      if (newTotal !== null) {
        const totalPages = Math.ceil(newTotal / perPage)
        endReached = maxLoaded >= totalPages
      } else if (rawRecords.length < perPage) {
        endReached = true
      }

      const newMeta = {
        ...meta,
        pagination: {
          ...meta.pagination,
          loadedPages: updatedLoadedPages,
          total: newTotal,
        },
        endReached,
      }

      dispatch(addTracks(entries, ids, newMeta))
    },
    [dispatch],
  )

  const maybePrefetchNextPage = useCallback(
    async ({ force = false, meta: metaOverride } = {}) => {
      const latest = latestPlayerStateRef.current
      const meta = metaOverride || latest.queueMeta
      if (!meta || meta.context !== 'songsList' || meta.endReached) {
        return null
      }

      const sessionId = meta.sessionId
      if (queuePrefetchRef.current.sessionId !== sessionId) {
        queuePrefetchRef.current.sessionId = sessionId
        queuePrefetchRef.current.pendingPromise = null
        queuePrefetchRef.current.loadingPage = null
      }

      if (queuePrefetchRef.current.pendingPromise) {
        return queuePrefetchRef.current.pendingPromise
      }

      const queue = latest.queue || []
      const currentUuid = latest.current?.uuid
      const currentIndex = queue.findIndex((item) => item.uuid === currentUuid)
      const remaining =
        currentIndex === -1 ? queue.length : queue.length - currentIndex - 1
      const perPage = meta.pagination?.perPage || PREFETCH_THRESHOLD
      const threshold = force
        ? Number.POSITIVE_INFINITY
        : Math.max(1, Math.min(PREFETCH_THRESHOLD, perPage - 1))

      if (!force && remaining > threshold) {
        return null
      }

      const nextPage = computeNextPage(meta)
      if (!nextPage) {
        return null
      }

      const fetchPromise = dataProvider
        .getList(meta.resource || 'song', {
          pagination: { page: nextPage, perPage },
          sort: meta.sort || { field: 'title', order: 'ASC' },
          filter: meta.filter || {},
        })
        .then((response) => {
          appendPrefetchedPage(nextPage, response, meta)
          return response
        })
        .catch((error) => {
          if (process.env.NODE_ENV !== 'production') {
            // eslint-disable-next-line no-console
            console.warn('Failed to prefetch songs queue page', error)
          }
          throw error
        })
        .finally(() => {
          if (queuePrefetchRef.current.sessionId === sessionId) {
            queuePrefetchRef.current.pendingPromise = null
            queuePrefetchRef.current.loadingPage = null
          }
        })

      queuePrefetchRef.current.pendingPromise = fetchPromise
      queuePrefetchRef.current.loadingPage = nextPage

      return fetchPromise
    },
    [appendPrefetchedPage, computeNextPage, dataProvider],
  )

  const queueMeta = playerState.queueMeta
  const queueLength = playerState.queue.length
  const currentTrackUuid = playerState.current?.uuid

  useEffect(() => {
    if (!queueMeta || queueMeta.context !== 'songsList') {
      return
    }
    if (initialPrefetchSessionRef.current === queueMeta.sessionId) {
      return
    }
    initialPrefetchSessionRef.current = queueMeta.sessionId
    const promise = maybePrefetchNextPage({
      force: true,
      meta: queueMeta,
    })
    if (promise && typeof promise.catch === 'function') {
      promise.catch(() => {})
    }
  }, [maybePrefetchNextPage, queueMeta])

  useEffect(() => {
    if (
      !queueMeta ||
      queueMeta.context !== 'songsList' ||
      (!currentTrackUuid && queueLength === 0)
    ) {
      return
    }
    const promise = maybePrefetchNextPage()
    if (promise && typeof promise.catch === 'function') {
      promise.catch(() => {})
    }
  }, [maybePrefetchNextPage, queueMeta, queueLength, currentTrackUuid])

  useEffect(() => {
    if (!playerState.queueMeta) {
      queuePrefetchRef.current = {
        sessionId: null,
        pendingPromise: null,
        loadingPage: null,
      }
      initialPrefetchSessionRef.current = null
    }
  }, [playerState.queueMeta])

  const ensureQueueReady = useCallback(
    async (direction) => {
      if (direction !== 'next') {
        return
      }
      const latest = latestPlayerStateRef.current
      const meta = latest.queueMeta
      if (!meta || meta.context !== 'songsList' || meta.endReached) {
        return
      }
      const queue = latest.queue || []
      const currentUuid = latest.current?.uuid
      const currentIndex = queue.findIndex((item) => item.uuid === currentUuid)
      if (currentIndex === -1 || currentIndex < queue.length - 1) {
        return
      }
      let pending = queuePrefetchRef.current.pendingPromise
      if (!pending) {
        pending = maybePrefetchNextPage({ force: true, meta })
      }
      if (pending && typeof pending.then === 'function') {
        try {
          await pending
        } catch (error) {
          if (process.env.NODE_ENV !== 'production') {
            // eslint-disable-next-line no-console
            console.warn('Unable to extend songs queue before skipping', error)
          }
        }
      }
    },
    [maybePrefetchNextPage],
  )

  const getTelemetrySnapshot = useCallback(() => {
    const latest = latestPlayerStateRef.current
    const queue = latest.queue || []
    const currentUuid = latest.current?.uuid
    const currentIndex = queue.findIndex((item) => item.uuid === currentUuid)
    return {
      context: latest.queueMeta?.context || 'unknown',
      queueLength: queue.length,
      currentIndex,
      mode: latest.mode,
      isPlaying: audioInstance ? !audioInstance.paused : false,
    }
  }, [audioInstance])

  const defaultOptions = useMemo(
    () => ({
      theme: playerTheme,
      bounds: 'body',
      playMode: playerState.mode,
      mode: 'full',
      loadAudioErrorPlayNext: false,
      autoPlayInitLoadPlayList: true,
      clearPriorAudioLists: false,
      showDestroy: true,
      showDownload: false,
      showLyric: true,
      showReload: false,
      toggleMode: !isDesktop,
      glassBg: false,
      showThemeSwitch: false,
      showMediaSession: true,
      restartCurrentOnPrev: true,
      quietUpdate: true,
      defaultPosition: {
        top: 300,
        left: 120,
      },
      volumeFade: { fadeIn: 200, fadeOut: 200 },
      renderAudioTitle: (audioInfo, isMobile) => (
        <AudioTitle
          audioInfo={audioInfo}
          gainInfo={gainInfo}
          isMobile={isMobile}
        />
      ),
      locale: locale(translate),
    }),
    [gainInfo, isDesktop, playerTheme, translate, playerState.mode],
  )

  const transportIcons = useMemo(() => {
    const nextLabel = translate('player.nextTrackText')
    const prevLabel = translate('player.previousTrackText')

    return {
      next: (
        <TransportControlButton
          audioInstance={audioInstance}
          direction="next"
          icon={<MdSkipNext size={28} />}
          label={nextLabel}
          onPointerInteraction={suppressHotkeysTemporarily}
          ensureQueueReady={ensureQueueReady}
          getTelemetrySnapshot={getTelemetrySnapshot}
        />
      ),
      prev: (
        <TransportControlButton
          audioInstance={audioInstance}
          direction="prev"
          icon={<MdSkipPrevious size={28} />}
          label={prevLabel}
          onPointerInteraction={suppressHotkeysTemporarily}
          ensureQueueReady={ensureQueueReady}
          getTelemetrySnapshot={getTelemetrySnapshot}
        />
      ),
    }
  }, [
    audioInstance,
    ensureQueueReady,
    getTelemetrySnapshot,
    suppressHotkeysTemporarily,
    translate,
  ])

  const options = useMemo(() => {
    const current = playerState.current || {}
    return {
      ...defaultOptions,
      audioLists: playerState.queue.map((item) => item),
      playIndex: playerState.playIndex,
      autoPlay: playerState.clear || playerState.playIndex === 0,
      clearPriorAudioLists: playerState.clear,
      extendsContent: (
        <PlayerToolbar id={current.trackId} isRadio={current.isRadio} />
      ),
      defaultVolume: isMobilePlayer ? 1 : playerState.volume,
      showMediaSession: !current.isRadio,
      icon: transportIcons,
    }
  }, [playerState, defaultOptions, isMobilePlayer, transportIcons])

  const onAudioListsChange = useCallback(
    (_, audioLists, audioInfo) => dispatch(syncQueue(audioInfo, audioLists)),
    [dispatch],
  )

  const nextSong = useCallback(() => {
    const idx = playerState.queue.findIndex(
      (item) => item.uuid === playerState.current.uuid,
    )
    return idx !== null ? playerState.queue[idx + 1] : null
  }, [playerState])

  const onAudioProgress = useCallback(
    (info) => {
      if (info.ended) {
        document.title = 'MusicMatters'
      }

      const progress = (info.currentTime / info.duration) * 100
      if (isNaN(info.duration) || (progress < 50 && info.currentTime < 240)) {
        return
      }

      if (info.isRadio) {
        return
      }

      if (!preloaded) {
        const next = nextSong()
        if (next != null) {
          const audio = new Audio()
          audio.src = next.musicSrc
        }
        setPreload(true)
        return
      }

      if (!scrobbled) {
        info.trackId && subsonic.scrobble(info.trackId, startTime)
        setScrobbled(true)
      }
    },
    [startTime, scrobbled, nextSong, preloaded],
  )

  const onAudioVolumeChange = useCallback(
    // sqrt to compensate for the logarithmic volume
    (volume) => dispatch(setVolume(Math.sqrt(volume))),
    [dispatch],
  )

  const onAudioPlay = useCallback(
    (info) => {
      // Do this to start the context; on chrome-based browsers, the context
      // will start paused since it is created prior to user interaction
      if (context && context.state !== 'running') {
        context.resume()
      }

      dispatch(currentPlaying(info))
      if (startTime === null) {
        setStartTime(Date.now())
      }
      if (info.duration) {
        const song = info.song
        document.title = `${song.title} - ${song.artist} - MusicMatters`
        if (!info.isRadio) {
          const pos = startTime === null ? null : Math.floor(info.currentTime)
          subsonic.nowPlaying(info.trackId, pos)
        }
        setPreload(false)
        if (config.gaTrackingId) {
          ReactGA.event({
            category: 'Player',
            action: 'Play song',
            label: `${song.title} - ${song.artist}`,
          })
        }
        if (showNotifications) {
          sendNotification(
            song.title,
            `${song.artist} - ${song.album}`,
            info.cover,
          )
        }
      }
    },
    [context, dispatch, showNotifications, startTime],
  )

  const onAudioPlayTrackChange = useCallback(() => {
    if (scrobbled) {
      setScrobbled(false)
    }
    if (startTime !== null) {
      setStartTime(null)
    }
  }, [scrobbled, startTime])

  const onAudioPause = useCallback(
    (info) => dispatch(currentPlaying(info)),
    [dispatch],
  )

  const onAudioEnded = useCallback(
    (currentPlayId, audioLists, info) => {
      setScrobbled(false)
      setStartTime(null)
      dispatch(currentPlaying(info))
      dataProvider
        .getOne('keepalive', { id: info.trackId })
        // eslint-disable-next-line no-console
        .catch((e) => console.log('Keepalive error:', e))
    },
    [dispatch, dataProvider],
  )

  const onCoverClick = useCallback((mode, audioLists, audioInfo) => {
    if (mode === 'full' && audioInfo?.song?.albumId) {
      window.location.href = `#/album/${audioInfo.song.albumId}/show`
    }
  }, [])

  const onBeforeDestroy = useCallback(() => {
    return new Promise((resolve, reject) => {
      dispatch(clearQueue())
      reject()
    })
  }, [dispatch])

  if (!visible) {
    document.title = 'MusicMatters'
  }

  const baseHandlers = useMemo(
    () => keyHandlers(audioInstance, playerState),
    [audioInstance, playerState],
  )

  const handlers = useMemo(
    () => (suppressHotkeys ? {} : baseHandlers),
    [baseHandlers, suppressHotkeys],
  )

  useEffect(() => {
    if (isMobilePlayer && audioInstance) {
      audioInstance.volume = 1
    }
  }, [isMobilePlayer, audioInstance])

  return (
    <ThemeProvider theme={createMuiTheme(theme)}>
      <ReactJkMusicPlayer
        {...options}
        className={classes.player}
        onAudioListsChange={onAudioListsChange}
        onAudioVolumeChange={onAudioVolumeChange}
        onAudioProgress={onAudioProgress}
        onAudioPlay={onAudioPlay}
        onAudioPlayTrackChange={onAudioPlayTrackChange}
        onAudioPause={onAudioPause}
        onPlayModeChange={(mode) => dispatch(setPlayMode(mode))}
        onAudioEnded={onAudioEnded}
        onCoverClick={onCoverClick}
        onBeforeDestroy={onBeforeDestroy}
        getAudioInstance={setAudioInstance}
      />
      <GlobalHotKeys handlers={handlers} keyMap={keyMap} allowChanges />
    </ThemeProvider>
  )
}

export { Player }
