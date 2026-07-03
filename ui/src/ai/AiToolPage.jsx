import { useEffect, useMemo, useRef, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  CircularProgress,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  Menu,
  MenuItem,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
  makeStyles,
} from '@material-ui/core'
import MoreVertIcon from '@material-ui/icons/MoreVert'
import AspectRatioIcon from '@material-ui/icons/AspectRatio'
import StopIcon from '@material-ui/icons/Stop'
import ViewColumnIcon from '@material-ui/icons/ViewColumn'
import { Title, useDataProvider, useTranslate } from 'react-admin'
import { httpClient } from '../dataProvider'

const ADDED_SONGS_STORAGE_KEY = 'aiToolAddedSongs'
const DEFAULT_AI_PROVIDER_STORAGE_KEY = 'aiToolDefaultProvider'
const AI_TOOL_COLUMNS_STORAGE_KEY = 'aiToolVisibleColumns'

const AI_TOOL_COLUMNS = [
  { id: 'title', label: 'Title' },
  { id: 'album', label: 'Album' },
  { id: 'albumConfidence', label: 'Album Confidence' },
  { id: 'artist', label: 'Artist' },
  { id: 'year', label: 'Year' },
  { id: 'yearConfidence', label: 'Year Confidence' },
  { id: 'explicit', label: 'Explicit' },
  { id: 'lyrics', label: 'Lyrics' },
  { id: 'duration', label: 'Time' },
  { id: 'genre', label: 'Genre' },
  { id: 'aiGenre', label: 'AI Genre' },
  { id: 'genreConfidence', label: 'Genre Confidence' },
]

const CONFIDENCE_COLUMN_IDS = ['albumConfidence', 'yearConfidence', 'genreConfidence']
const DEFAULT_AI_TOOL_COLUMN_VISIBILITY = Object.fromEntries(
  AI_TOOL_COLUMNS.map((column) => [column.id, true]),
)

const AI_PROVIDERS = [
  { id: 'gemini-2.5', label: 'Gemini 2.5' },
  { id: 'gemini-3.5', label: 'Gemini 3.5' },
  { id: 'gemma-26b', label: 'Gemma 26B' },
]

const AI_SERVICES = [
  { id: 'gemma-26b', label: 'Gemma 26B' },
  { id: 'whisper', label: 'Whisper' },
  { id: 'gemini-2.5', label: 'Gemini 2.5' },
  { id: 'gemini-3.5', label: 'Gemini 3.5' },
]

const CHAT_MIN_WIDTH = 320
const CHAT_MIN_HEIGHT = 420
const CHAT_MARGIN = 16
const CHAT_DEFAULT_WIDTH = 360
const CHAT_DEFAULT_HEIGHT = 520

const defaultChatFrame = () => {
  if (typeof window === 'undefined') {
    return {
      left: CHAT_MARGIN,
      top: CHAT_MARGIN,
      width: CHAT_DEFAULT_WIDTH,
      height: CHAT_DEFAULT_HEIGHT,
    }
  }

  return {
    left: Math.max(window.innerWidth - CHAT_DEFAULT_WIDTH - 24, CHAT_MARGIN),
    top: Math.max(window.innerHeight - CHAT_DEFAULT_HEIGHT - 24, CHAT_MARGIN),
    width: CHAT_DEFAULT_WIDTH,
    height: CHAT_DEFAULT_HEIGHT,
  }
}

const expandedChatFrame = () => {
  if (typeof window === 'undefined') {
    return {
      left: CHAT_MARGIN,
      top: CHAT_MARGIN,
      width: 720,
      height: CHAT_DEFAULT_HEIGHT,
    }
  }

  const width = Math.min(720, window.innerWidth - CHAT_MARGIN * 2)
  return {
    left: Math.max(window.innerWidth - width - 24, CHAT_MARGIN),
    top: 48,
    width,
    height: Math.max(window.innerHeight - 96, CHAT_MIN_HEIGHT),
  }
}

const normalizeAIProvider = (provider) => {
  switch (provider) {
    case 'gemini-3.5':
    case 'gemini-3.5-flash':
      return 'gemini-3.5'
    case 'gemma-26b':
    case 'gemma-26':
    case 'gemma-4':
    case 'gemma-3':
    case 'gemma3':
    case 'gemma3:4b':
      return 'gemma-26b'
    case 'gemini-2.5':
    case 'gemini-2.5-flash':
    default:
      return 'gemini-2.5'
  }
}

const aiProviderLabel = (provider) =>
  AI_PROVIDERS.find((option) => option.id === normalizeAIProvider(provider))?.label || 'AI'

const normalizeAddedSongs = (songs = []) => {
  const byId = new Map()
  songs.forEach((song) => {
    if (!song?.id) return
    byId.set(song.id, { ...(byId.get(song.id) || {}), ...song })
  })
  return [...byId.values()]
}

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(2),
  },
  section: {
    marginBottom: theme.spacing(2),
  },
  tableWrap: {
    marginTop: theme.spacing(2),
    overflowX: 'auto',
  },
  tableActions: {
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  providerToolbar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    flexWrap: 'wrap',
    gap: theme.spacing(1.5),
    marginBottom: theme.spacing(2),
  },
  defaultProviderSelect: {
    minWidth: 220,
    '& .MuiOutlinedInput-root': {
      color: '#ffffff',
      background: '#151f2d',
      borderRadius: 8,
    },
    '& .MuiInputLabel-root': {
      color: '#ff8fc6',
    },
    '& .MuiOutlinedInput-notchedOutline': {
      borderColor: 'rgba(255, 42, 142, 0.45)',
    },
    '& .MuiSelect-icon': {
      color: '#ff8fc6',
    },
  },
  serviceStatusPanel: {
    width: '100%',
    borderRadius: 8,
    border: '1px solid rgba(255, 255, 255, 0.12)',
    background: '#151f2d',
    overflow: 'hidden',
  },
  serviceStatusHeader: {
    width: '100%',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: theme.spacing(1.25, 1.5),
    color: '#ffffff',
    cursor: 'pointer',
    background: 'transparent',
    border: 0,
    textAlign: 'left',
    font: 'inherit',
  },
  serviceStatusSummary: {
    color: '#c9d1dc',
    fontSize: 13,
  },
  serviceStatusList: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))',
    gap: theme.spacing(1),
    padding: theme.spacing(0, 1.5, 1.5),
  },
  serviceStatusItem: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    padding: theme.spacing(1),
    borderRadius: 6,
    background: '#0f1722',
  },
  serviceStatusName: {
    color: '#f7f8fb',
  },
  serviceStatusValue: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 12,
  },
  statusDot: {
    width: 8,
    height: 8,
    borderRadius: '50%',
    background: '#8a94a3',
  },
  statusDotOnline: {
    background: '#3ddc84',
    boxShadow: '0 0 8px rgba(61, 220, 132, 0.6)',
  },
  statusDotOffline: {
    background: '#ff5c8a',
    boxShadow: '0 0 8px rgba(255, 92, 138, 0.45)',
  },
  buttonProgress: {
    marginRight: theme.spacing(1),
  },
  progressPanel: {
    marginTop: theme.spacing(1.5),
    padding: theme.spacing(1.25, 1.5),
    borderRadius: 8,
    border: '1px solid rgba(255, 42, 142, 0.28)',
    background: '#151f2d',
  },
  progressHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(0.75),
  },
  progressText: {
    color: '#f7f8fb',
  },
  progressMeta: {
    color: '#ff8fc6',
    whiteSpace: 'nowrap',
  },
  progressActions: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  stopJobButton: {
    minWidth: 86,
  },
  progressBar: {
    height: 6,
    borderRadius: 6,
    backgroundColor: 'rgba(255, 255, 255, 0.12)',
    '& .MuiLinearProgress-bar': {
      backgroundColor: '#ff2a8e',
    },
  },
  valueExisting: {
    color: '#f7f8fb',
  },
  valueAI: {
    color: '#ff2a8e',
  },
  confidenceBadge: {
    padding: '1px 6px',
    borderRadius: 10,
    fontSize: 11,
    lineHeight: 1.5,
    background: 'rgba(255, 255, 255, 0.08)',
    whiteSpace: 'nowrap',
  },
  confidenceColumn: {
    minWidth: 130,
    whiteSpace: 'nowrap',
  },
  columnMenuTitle: {
    padding: theme.spacing(1.5, 2, 0.75),
    color: '#c9d1dc',
  },
  columnMenuItems: {
    maxHeight: 420,
    overflowY: 'auto',
  },
  chatWidget: {
    position: 'fixed',
    minWidth: CHAT_MIN_WIDTH,
    maxWidth: 'calc(100vw - 32px)',
    minHeight: CHAT_MIN_HEIGHT,
    maxHeight: 'calc(100vh - 96px)',
    zIndex: theme.zIndex.drawer + 2,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    borderRadius: 8,
    border: '1px solid rgba(255, 42, 142, 0.36)',
    background: '#0f1722',
    boxShadow: '0 24px 60px rgba(0, 0, 0, 0.45)',
  },
  chatResizeHandle: {
    position: 'absolute',
    width: 18,
    height: 18,
    zIndex: 2,
    '&::after': {
      content: '""',
      position: 'absolute',
      inset: 5,
      borderRadius: '50%',
      border: '1px solid rgba(255, 42, 142, 0.55)',
      background: 'rgba(255, 42, 142, 0.18)',
    },
  },
  chatResizeTopLeft: {
    top: 0,
    left: 0,
    cursor: 'nwse-resize',
  },
  chatResizeTopRight: {
    top: 0,
    right: 0,
    cursor: 'nesw-resize',
  },
  chatResizeBottomLeft: {
    bottom: 0,
    left: 0,
    cursor: 'nesw-resize',
  },
  chatResizeBottomRight: {
    bottom: 0,
    right: 0,
    cursor: 'nwse-resize',
  },
  chatHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    padding: theme.spacing(1.5, 1.75),
    background: '#111b28',
    borderBottom: '1px solid rgba(255, 255, 255, 0.08)',
  },
  chatAvatar: {
    width: 44,
    height: 44,
    borderRadius: '50%',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontWeight: 700,
    color: '#ffffff',
    background: 'linear-gradient(135deg, #ff2a8e, #161b24)',
    border: '2px solid rgba(255, 255, 255, 0.12)',
  },
  chatTitleWrap: {
    flex: 1,
    minWidth: 0,
  },
  chatHeaderActions: {
    display: 'flex',
    gap: theme.spacing(0.75),
  },
  chatTitle: {
    color: '#ffffff',
    fontWeight: 700,
    lineHeight: 1.15,
  },
  chatStatus: {
    color: '#ff8fc6',
    fontSize: 13,
    marginTop: 2,
  },
  chatStatusOnline: {
    color: '#3ddc84',
  },
  chatStatusOffline: {
    color: '#ff8fc6',
  },
  chatClose: {
    minWidth: 32,
    width: 32,
    height: 32,
    padding: 0,
    borderRadius: '50%',
    color: '#ffffff',
    borderColor: 'rgba(255, 255, 255, 0.18)',
  },
  chatMessages: {
    flex: 1,
    overflowY: 'auto',
    padding: theme.spacing(2),
    background: '#172231',
  },
  chatMessageRow: {
    display: 'flex',
    marginBottom: theme.spacing(1.5),
  },
  chatMessageRowUser: {
    justifyContent: 'flex-end',
  },
  chatMessageLabel: {
    color: '#f7f8fb',
    fontSize: 12,
    marginBottom: theme.spacing(0.5),
  },
  chatBubble: {
    maxWidth: '82%',
    borderRadius: 18,
    padding: theme.spacing(1.2, 1.5),
    lineHeight: 1.45,
    wordBreak: 'break-word',
  },
  chatBubbleAssistant: {
    color: '#111827',
    background: '#ffffff',
    borderTopLeftRadius: 6,
  },
  chatBubbleUser: {
    color: '#ffffff',
    background: '#ff2a8e',
    borderTopRightRadius: 6,
  },
  chatThinking: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 4,
    '& span': {
      width: 6,
      height: 6,
      borderRadius: '50%',
      background: '#ff2a8e',
      animation: '$thinkingDot 1s infinite ease-in-out',
    },
    '& span:nth-child(2)': {
      animationDelay: '0.15s',
    },
    '& span:nth-child(3)': {
      animationDelay: '0.3s',
    },
  },
  '@keyframes thinkingDot': {
    '0%, 80%, 100%': {
      opacity: 0.35,
      transform: 'translateY(0)',
    },
    '40%': {
      opacity: 1,
      transform: 'translateY(-3px)',
    },
  },
  chatInputBar: {
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: theme.spacing(1.25),
    background: '#0f1722',
    borderTop: '1px solid rgba(255, 255, 255, 0.08)',
  },
  chatInput: {
    '& .MuiOutlinedInput-root': {
      color: '#ffffff',
      background: '#151f2d',
      borderRadius: 8,
    },
    '& .MuiOutlinedInput-notchedOutline': {
      borderColor: 'rgba(255, 255, 255, 0.16)',
    },
    '& .MuiOutlinedInput-root:hover .MuiOutlinedInput-notchedOutline': {
      borderColor: 'rgba(255, 42, 142, 0.75)',
    },
    '& input::placeholder': {
      color: '#c9d1dc',
      opacity: 1,
    },
  },
  chatModelSelect: {
    width: 136,
    flexShrink: 0,
    '& .MuiOutlinedInput-root': {
      color: '#ffffff',
      background: '#151f2d',
      borderRadius: 8,
    },
    '& .MuiOutlinedInput-notchedOutline': {
      borderColor: 'rgba(255, 255, 255, 0.16)',
    },
    '& .MuiOutlinedInput-root:hover .MuiOutlinedInput-notchedOutline': {
      borderColor: 'rgba(255, 42, 142, 0.75)',
    },
    '& .MuiSelect-icon': {
      color: '#ff8fc6',
    },
  },
  chatSendButton: {
    minWidth: 48,
    width: 48,
    height: 44,
    borderRadius: 8,
    color: '#ffffff',
    background: '#ff2a8e',
    '&:hover': {
      background: '#e9197c',
    },
  },
  chatStopButton: {
    minWidth: 54,
    width: 54,
    height: 54,
    borderRadius: '50%',
    color: '#ffffff',
    background: '#3b3b3b',
    boxShadow: 'none',
    '&:hover': {
      background: '#4a4a4a',
    },
  },
  chatLauncher: {
    position: 'fixed',
    right: theme.spacing(3),
    bottom: theme.spacing(3),
    zIndex: theme.zIndex.drawer + 2,
    width: 58,
    height: 58,
    minWidth: 58,
    borderRadius: '50%',
    color: '#ffffff',
    background: '#ff2a8e',
    boxShadow: '0 16px 36px rgba(0, 0, 0, 0.35)',
    '&:hover': {
      background: '#e9197c',
    },
  },
  chatError: {
    color: '#ff8fc6',
    padding: theme.spacing(0, 1.5, 1.25),
    background: '#0f1722',
  },
}))

const formatDuration = (seconds) => {
  const total = Number(seconds)
  if (!Number.isFinite(total) || total <= 0) return ''
  const mins = Math.floor(total / 60)
  const secs = Math.floor(total % 60)
  return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
}

const formatExplicitStatus = (status) => {
  const normalized = String(status || '').trim().toLowerCase()
  if (normalized === 'e' || normalized === 'explicit') return 'Explicit'
  if (normalized === 'c' || normalized === 'clean') return 'Clean'
  return ''
}

const isUnknownValue = (value) => {
  const normalized = String(value || '').trim().toLowerCase()
  return normalized === '' || normalized === 'unknown' || normalized === 'unknown album' || normalized === '[unknown album]'
}

const hasSavedLyrics = (song) => {
  const lyrics = String(song?.lyrics || song?.lyricsText || '').trim()
  return lyrics !== '' && lyrics !== '[]'
}

const normalizeMetadataConfidence = (value) => {
  if (value === null || value === undefined || value === '') return null
  const confidence = Number(value)
  if (!Number.isFinite(confidence)) return null
  return Math.max(0, Math.min(100, Math.round(confidence)))
}

const AiToolPage = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const dataProvider = useDataProvider()
  const chatMessagesRef = useRef(null)
  const chatAbortControllerRef = useRef(null)
  const jobAbortControllerRef = useRef(null)
  const [messages, setMessages] = useState([])
  const [prompt, setPrompt] = useState('')
  const [songDialogOpen, setSongDialogOpen] = useState(false)
  const [songsLoading, setSongsLoading] = useState(false)
  const [availableSongs, setAvailableSongs] = useState([])
  const [selectedSongIds, setSelectedSongIds] = useState([])
  const [selectedAddedSongIds, setSelectedAddedSongIds] = useState([])
  const [addedSongs, setAddedSongs] = useState(() => {
    try {
      const raw = localStorage.getItem(ADDED_SONGS_STORAGE_KEY)
      const parsed = raw ? JSON.parse(raw) : []
      return Array.isArray(parsed) ? normalizeAddedSongs(parsed) : []
    } catch {
      return []
    }
  })

  const [isSending, setIsSending] = useState(false)
  const [defaultProvider, setDefaultProvider] = useState(() => {
    try {
      return normalizeAIProvider(localStorage.getItem(DEFAULT_AI_PROVIDER_STORAGE_KEY))
    } catch {
      return 'gemini-2.5'
    }
  })
  const [chatProvider, setChatProvider] = useState(defaultProvider)
  const [chatProviderOverridden, setChatProviderOverridden] = useState(false)
  const [chatError, setChatError] = useState('')
  const [toolError, setToolError] = useState('')
  const [isChatOpen, setIsChatOpen] = useState(true)
  const [isChatExpanded, setIsChatExpanded] = useState(false)
  const [chatFrame, setChatFrame] = useState(defaultChatFrame)
  const [lyricsLoadingId, setLyricsLoadingId] = useState('')
  const [lyricsDialogOpen, setLyricsDialogOpen] = useState(false)
  const [lyricsDialogTitle, setLyricsDialogTitle] = useState('')
  const [lyricsText, setLyricsText] = useState('')
  const [isClassifyingExplicit, setIsClassifyingExplicit] = useState(false)
  const [isFetchingMetadata, setIsFetchingMetadata] = useState(false)
  const [isClearingMetadata, setIsClearingMetadata] = useState(false)
  const [modelDialogAction, setModelDialogAction] = useState('')
  const [modelDialogSongs, setModelDialogSongs] = useState([])
  const [modelDialogProvider, setModelDialogProvider] = useState(defaultProvider)
  const [columnMenuAnchorEl, setColumnMenuAnchorEl] = useState(null)
  const [visibleColumns, setVisibleColumns] = useState(() => {
    try {
      const saved = JSON.parse(localStorage.getItem(AI_TOOL_COLUMNS_STORAGE_KEY) || '{}')
      return { ...DEFAULT_AI_TOOL_COLUMN_VISIBILITY, ...saved }
    } catch {
      return DEFAULT_AI_TOOL_COLUMN_VISIBILITY
    }
  })
  const [rowActionAnchorEl, setRowActionAnchorEl] = useState(null)
  const [rowActionSong, setRowActionSong] = useState(null)
  const [jobProgress, setJobProgress] = useState(null)
  const [isStatusOpen, setIsStatusOpen] = useState(true)
  const [modelStatuses, setModelStatuses] = useState(() =>
    AI_SERVICES.map((service) => ({ ...service, online: null })),
  )

  const selectedSongs = useMemo(
    () => availableSongs.filter((song) => selectedSongIds.includes(song.id)),
    [availableSongs, selectedSongIds],
  )

  const selectedAddedSongIdSet = useMemo(
    () => new Set(selectedAddedSongIds),
    [selectedAddedSongIds],
  )

  const selectedAddedSongs = useMemo(
    () => addedSongs.filter((song) => selectedAddedSongIdSet.has(song.id)),
    [addedSongs, selectedAddedSongIdSet],
  )

  const selectedAddedIds = useMemo(
    () => selectedAddedSongs.map((song) => song.id),
    [selectedAddedSongs],
  )

  const jobProgressValue = jobProgress?.total
    ? Math.round((jobProgress.done / jobProgress.total) * 100)
    : 0
  const isFetchJobRunning = Boolean(lyricsLoadingId) || isFetchingMetadata

  const selectedModelStatus = modelStatuses.find(
    (service) => service.id === normalizeAIProvider(chatProvider),
  )
  const selectedModelOnline = selectedModelStatus?.online
  const onlineServiceCount = modelStatuses.filter((service) => service.online === true).length
  const areStatusesChecking = modelStatuses.some((service) => service.online === null)

  const isAIValue = (song, field) => {
    if (song.aiFields?.[field]) return true
    if (field === 'aiGenre') return !isUnknownValue(song.aiGenre)
    if (field === 'lyrics') return hasSavedLyrics(song)
    if (field === 'explicitStatus') return Boolean(formatExplicitStatus(song.explicitStatus))
    if ((field === 'album' || field === 'year') && !isUnknownValue(song[field]) && !isUnknownValue(song.aiGenre)) {
      return true
    }
    return false
  }

  const valueClass = (song, field) =>
    isAIValue(song, field) ? classes.valueAI : classes.valueExisting

  const renderMetadataConfidence = (value) => {
    const confidence = normalizeMetadataConfidence(value)
    if (confidence === null) return null
    const color = confidence >= 80 ? '#3ddc84' : confidence >= 50 ? '#ffcb6b' : '#ff8fc6'
    return (
      <Typography
        component="span"
        className={classes.confidenceBadge}
        style={{ color }}
      >
        {confidence}% confidence
      </Typography>
    )
  }

  const isColumnVisible = (column) => visibleColumns[column] !== false
  const allConfidenceColumnsVisible = CONFIDENCE_COLUMN_IDS.every(isColumnVisible)
  const someConfidenceColumnsVisible = CONFIDENCE_COLUMN_IDS.some(isColumnVisible)

  const toggleColumn = (column) => {
    setVisibleColumns((current) => ({
      ...current,
      [column]: current[column] === false,
    }))
  }

  const toggleAllConfidenceColumns = () => {
    const visible = !allConfidenceColumnsVisible
    setVisibleColumns((current) => ({
      ...current,
      ...Object.fromEntries(CONFIDENCE_COLUMN_IDS.map((column) => [column, visible])),
    }))
  }

  const addedSongIds = useMemo(
    () => addedSongs.map((song) => song.id).join('|'),
    [addedSongs],
  )

  useEffect(() => {
    const normalized = normalizeAddedSongs(addedSongs)
    if (normalized.length !== addedSongs.length) {
      setAddedSongs(normalized)
      return
    }
    localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(normalized))
  }, [addedSongs])

  useEffect(() => {
    localStorage.setItem(DEFAULT_AI_PROVIDER_STORAGE_KEY, defaultProvider)
    if (!chatProviderOverridden) {
      setChatProvider(defaultProvider)
    }
  }, [defaultProvider, chatProviderOverridden])

  useEffect(() => {
    localStorage.setItem(AI_TOOL_COLUMNS_STORAGE_KEY, JSON.stringify(visibleColumns))
  }, [visibleColumns])

  useEffect(() => {
    let active = true

    const refreshStatuses = async () => {
      try {
        const { json } = await httpClient('/api/ai/status')
        if (!active) return
        const byId = new Map((json?.services || []).map((service) => [service.id, service]))
        setModelStatuses(
          AI_SERVICES.map((service) => ({
            ...service,
            online: byId.get(service.id)?.online === true,
          })),
        )
      } catch {
        if (active) {
          setModelStatuses(AI_SERVICES.map((service) => ({ ...service, online: false })))
        }
      }
    }

    refreshStatuses()
    const interval = window.setInterval(refreshStatuses, 30000)
    return () => {
      active = false
      window.clearInterval(interval)
    }
  }, [])

  useEffect(() => {
    if (!isChatOpen || !chatMessagesRef.current) return

    const messagesEl = chatMessagesRef.current
    messagesEl.scrollTop = messagesEl.scrollHeight
  }, [messages, isSending, isChatOpen, isChatExpanded])

  useEffect(() => () => {
    chatAbortControllerRef.current?.abort()
    jobAbortControllerRef.current?.abort()
  }, [])

  useEffect(() => {
    const visibleIds = new Set(addedSongs.map((song) => song.id))
    setSelectedAddedSongIds((prev) => prev.filter((id) => visibleIds.has(id)))
  }, [addedSongs])

  useEffect(() => {
    if (!addedSongIds) return undefined

    let active = true
    const reconcileAddedSongs = async () => {
      try {
        const query = addedSongs.map((song) => `id=${encodeURIComponent(song.id)}`).join('&')
        const { json } = await httpClient(`/api/song?${query}`)
        const currentSongs = new Map((json || []).map((song) => [song.id, song]))
        const nextSongs = addedSongs
          .filter((song) => currentSongs.has(song.id))
          .map((song) => {
            const current = currentSongs.get(song.id)
            return {
              ...song,
              ...current,
              aiGenre: song.aiGenre || '',
            }
          })

        if (active && JSON.stringify(nextSongs) !== JSON.stringify(addedSongs)) {
          setAddedSongs(nextSongs)
          localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
        }
      } catch {
        // Keep the local queue if the API is temporarily unavailable.
      }
    }

    reconcileAddedSongs()
    return () => {
      active = false
    }
  }, [addedSongIds])


  const sendMessage = async () => {
    const trimmed = prompt.trim()
    if (!trimmed || isSending) return

    const provider = normalizeAIProvider(chatProvider)
    const abortController = new AbortController()
    chatAbortControllerRef.current = abortController
    setChatError('')
    setMessages((prev) => [...prev, { role: 'user', text: trimmed, provider }])
    setPrompt('')
    setIsSending(true)

    try {
      const { json: payload } = await httpClient('/api/ai/chat', {
        method: 'POST',
        signal: abortController.signal,
        body: JSON.stringify({ message: trimmed, provider }),
      })

      setMessages((prev) => [
        ...prev,
        {
          role: 'assistant',
          text: payload.response || '',
          provider: payload.provider || provider,
          model: payload.model || '',
        },
      ])
    } catch (err) {
      if (abortController.signal.aborted || err?.name === 'AbortError') {
        setChatError('')
      } else {
        setChatError(err?.message || 'Could not get response from AI')
      }
    } finally {
      if (chatAbortControllerRef.current === abortController) {
        chatAbortControllerRef.current = null
      }
      setIsSending(false)
    }
  }

  const stopMessage = () => {
    chatAbortControllerRef.current?.abort()
  }

  const startChatResize = (corner, event) => {
    event.preventDefault()
    event.stopPropagation()
    setIsChatExpanded(false)

    const startX = event.clientX
    const startY = event.clientY
    const startFrame = isChatExpanded ? expandedChatFrame() : chatFrame

    const onMouseMove = (moveEvent) => {
      const dx = moveEvent.clientX - startX
      const dy = moveEvent.clientY - startY
      const maxRight = window.innerWidth - CHAT_MARGIN
      const maxBottom = window.innerHeight - CHAT_MARGIN

      let { left, top, width, height } = startFrame

      if (corner.includes('left')) {
        left = startFrame.left + dx
        width = startFrame.width - dx
        if (left < CHAT_MARGIN) {
          width += left - CHAT_MARGIN
          left = CHAT_MARGIN
        }
        if (width < CHAT_MIN_WIDTH) {
          left = startFrame.left + startFrame.width - CHAT_MIN_WIDTH
          width = CHAT_MIN_WIDTH
        }
      }

      if (corner.includes('right')) {
        width = startFrame.width + dx
        width = Math.max(CHAT_MIN_WIDTH, Math.min(width, maxRight - left))
      }

      if (corner.includes('top')) {
        top = startFrame.top + dy
        height = startFrame.height - dy
        if (top < CHAT_MARGIN) {
          height += top - CHAT_MARGIN
          top = CHAT_MARGIN
        }
        if (height < CHAT_MIN_HEIGHT) {
          top = startFrame.top + startFrame.height - CHAT_MIN_HEIGHT
          height = CHAT_MIN_HEIGHT
        }
      }

      if (corner.includes('bottom')) {
        height = startFrame.height + dy
        height = Math.max(CHAT_MIN_HEIGHT, Math.min(height, maxBottom - top))
      }

      setChatFrame({ left, top, width, height })
    }

    const onMouseUp = () => {
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }

    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
  }

  const openAddSongsDialog = async () => {
    setSongDialogOpen(true)
    if (availableSongs.length) return

    setSongsLoading(true)
    try {
      const { data } = await dataProvider.getList('song', {
        pagination: { page: 1, perPage: 200 },
        sort: { field: 'title', order: 'ASC' },
        filter: {},
      })
      setAvailableSongs(data || [])
    } finally {
      setSongsLoading(false)
    }
  }

  const toggleSong = (songId) => {
    setSelectedSongIds((prev) =>
      prev.includes(songId) ? prev.filter((id) => id !== songId) : [...prev, songId],
    )
  }

  const addSelectedSongs = () => {
    setAddedSongs((prev) => {
      const existing = new Set(prev.map((song) => song.id))
      const uniqueNew = selectedSongs.filter((song) => !existing.has(song.id))
      const nextSongs = normalizeAddedSongs([...prev, ...uniqueNew])
      localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
      return nextSongs
    })
    setSongDialogOpen(false)
  }

  const toggleAddedSong = (songId) => {
    setSelectedAddedSongIds((prev) =>
      prev.includes(songId) ? prev.filter((id) => id !== songId) : [...prev, songId],
    )
  }

  const toggleAllAddedSongs = () => {
    setSelectedAddedSongIds(
      selectedAddedIds.length === addedSongs.length ? [] : addedSongs.map((song) => song.id),
    )
  }

  const removeSong = (songId) => {
    setAddedSongs((prev) => {
      const nextSongs = prev.filter((song) => song.id !== songId)
      localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
      return nextSongs
    })
    setSelectedAddedSongIds((prev) => prev.filter((id) => id !== songId))
  }

  const removeSelectedSongs = () => {
    if (!selectedAddedIds.length) return

    const selected = new Set(selectedAddedIds)
    setAddedSongs((prev) => {
      const nextSongs = prev.filter((song) => !selected.has(song.id))
      localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
      return nextSongs
    })
    setSelectedAddedSongIds([])
  }

  const removeStaleSongOnNotFound = (err, song) => {
    if (!song?.id || !String(err?.message || '').toLowerCase().includes('not found')) return
    removeSong(song.id)
  }

  const startProgress = (type, songs) => {
    setJobProgress({
      type,
      currentTitle: songs[0]?.title || '',
      done: 0,
      total: songs.length,
      status: 'running',
    })
  }

  const updateProgress = (type, song, done, total) => {
    setJobProgress({
      type,
      currentTitle: song?.title || '',
      done,
      total,
      status: 'running',
    })
  }

  const finishProgress = (type, total) => {
    setJobProgress({
      type,
      currentTitle: 'Complete',
      done: total,
      total,
      status: 'complete',
    })
    window.setTimeout(() => {
      setJobProgress((prev) =>
        prev?.type === type && prev?.status === 'complete' ? null : prev,
      )
    }, 1500)
  }

  const stopProgress = (type) => {
    setJobProgress((prev) =>
      prev?.type === type ? { ...prev, currentTitle: 'Stopped', status: 'stopped' } : prev,
    )
    window.setTimeout(() => {
      setJobProgress((prev) =>
        prev?.type === type && prev?.status === 'stopped' ? null : prev,
      )
    }, 1500)
  }

  const stopFetchJob = () => {
    if (!jobAbortControllerRef.current) return

    jobAbortControllerRef.current.abort()
    setJobProgress((prev) =>
      prev ? { ...prev, currentTitle: 'Stopping…', status: 'stopping' } : prev,
    )
  }

  const openRowActions = (event, song) => {
    setRowActionAnchorEl(event.currentTarget)
    setRowActionSong(song)
  }

  const closeRowActions = () => {
    setRowActionAnchorEl(null)
    setRowActionSong(null)
  }

  const fetchLyrics = async (song) => {
    if (!song?.id || lyricsLoadingId || jobAbortControllerRef.current) return

    const abortController = new AbortController()
    jobAbortControllerRef.current = abortController
    setToolError('')
    setLyricsLoadingId(song.id)
    startProgress('lyrics', [song])
    try {
      await httpClient(`/api/ai/songs/${song.id}/lyrics/fetch`, {
        method: 'POST',
        signal: abortController.signal,
      })
      setAddedSongs((prev) => {
        const nextSongs = prev.map((item) =>
          item.id === song.id ? { ...item, lyrics: 'saved' } : item,
        )
        localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
        return nextSongs
      })
      updateProgress('lyrics', song, 1, 1)
      finishProgress('lyrics', 1)
    } catch (err) {
      if (abortController.signal.aborted || err?.name === 'AbortError') {
        stopProgress('lyrics')
      } else {
        removeStaleSongOnNotFound(err, song)
        setToolError(err?.message || 'Could not fetch lyrics')
        setJobProgress(null)
      }
    } finally {
      if (jobAbortControllerRef.current === abortController) {
        jobAbortControllerRef.current = null
      }
      setLyricsLoadingId('')
    }
  }

  const fetchSelectedLyrics = async () => {
    if (!selectedAddedIds.length || lyricsLoadingId || jobAbortControllerRef.current) return

    const abortController = new AbortController()
    jobAbortControllerRef.current = abortController
    setToolError('')
    setLyricsLoadingId('bulk')
    startProgress('lyrics', selectedAddedSongs)
    try {
      for (const [index, song] of selectedAddedSongs.entries()) {
        updateProgress('lyrics', song, index, selectedAddedSongs.length)
        await httpClient(`/api/ai/songs/${song.id}/lyrics/fetch`, {
          method: 'POST',
          signal: abortController.signal,
        })
        setAddedSongs((prev) => {
          const nextSongs = prev.map((item) =>
            item.id === song.id ? { ...item, lyrics: 'saved' } : item,
          )
          localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
          return nextSongs
        })
        updateProgress('lyrics', song, index + 1, selectedAddedSongs.length)
      }
      finishProgress('lyrics', selectedAddedSongs.length)
    } catch (err) {
      if (abortController.signal.aborted || err?.name === 'AbortError') {
        stopProgress('lyrics')
      } else {
        setToolError(err?.message || 'Could not fetch lyrics')
        setJobProgress(null)
      }
    } finally {
      if (jobAbortControllerRef.current === abortController) {
        jobAbortControllerRef.current = null
      }
      setLyricsLoadingId('')
    }
  }

  const showLyrics = async (song) => {
    if (!song?.id) return

    setToolError('')
    try {
      const { json: payload } = await httpClient(`/api/ai/songs/${song.id}/lyrics`)
      setLyricsDialogTitle(song.title || translate('menu.aiTool.lyrics', { _: 'Lyrics' }))
      setLyricsText(payload.text || '')
      setLyricsDialogOpen(true)
    } catch (err) {
      removeStaleSongOnNotFound(err, song)
      setToolError(err?.message || 'Could not load lyrics')
    }
  }

  const openModelDialog = (action, songs) => {
    if (!songs.length) return

    setModelDialogAction(action)
    setModelDialogSongs(songs)
    setModelDialogProvider(defaultProvider)
  }

  const closeModelDialog = () => {
    setModelDialogAction('')
    setModelDialogSongs([])
  }

  const classifyExplicit = async (songs, provider) => {
    const songIds = songs.map((song) => song.id)
    if (!songIds.length || isClassifyingExplicit) return

    setToolError('')
    setIsClassifyingExplicit(true)
    try {
      const { json: payload } = await httpClient('/api/ai/classify-explicit', {
        method: 'POST',
        body: JSON.stringify({
          songIds,
          provider: normalizeAIProvider(provider),
        }),
      })
      const statuses = new Map((payload.songs || []).map((song) => [song.id, song.explicitStatus]))
      setAddedSongs((prev) => {
        const nextSongs = prev.map((song) =>
          statuses.has(song.id)
            ? {
                ...song,
                explicitStatus: statuses.get(song.id) || '',
                aiFields: {
                  ...(song.aiFields || {}),
                  explicitStatus: Boolean(statuses.get(song.id)),
                },
              }
            : song,
        )
        localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
        return nextSongs
      })
    } catch (err) {
      setToolError(err?.message || 'Could not classify explicit content')
    } finally {
      setIsClassifyingExplicit(false)
    }
  }

  const fetchAIMetadataForSongs = async (songs, provider) => {
    if (!songs.length || isFetchingMetadata || jobAbortControllerRef.current) return

    const abortController = new AbortController()
    jobAbortControllerRef.current = abortController
    setToolError('')
    setIsFetchingMetadata(true)
    startProgress('metadata', songs)
    try {
      for (const [index, song] of songs.entries()) {
        updateProgress('metadata', song, index, songs.length)
        const { json: payload } = await httpClient('/api/ai/fetch-metadata', {
          method: 'POST',
          signal: abortController.signal,
          body: JSON.stringify({
            songIds: [song.id],
            provider: normalizeAIProvider(provider),
          }),
        })
        const metadata = new Map((payload.songs || []).map((item) => [item.id, item]))
        setAddedSongs((prev) => {
          const nextSongs = prev.map((item) => {
            const update = metadata.get(item.id)
            if (!update) return item
            return {
              ...item,
              album: update.album || item.album,
              year: update.year || item.year,
              aiGenre: update.aiGenre || item.aiGenre || '',
              metadataConfidence: {
                ...(item.metadataConfidence || {}),
                album: normalizeMetadataConfidence(update.albumConfidence) ?? 0,
                year: normalizeMetadataConfidence(update.yearConfidence) ?? 0,
                genre: normalizeMetadataConfidence(update.genreConfidence) ?? 0,
              },
              aiFields: {
                ...(item.aiFields || {}),
                album: Boolean(update.album) || Boolean(item.aiFields?.album),
                year: Boolean(update.year) || Boolean(item.aiFields?.year),
                aiGenre: Boolean(update.aiGenre) || Boolean(item.aiFields?.aiGenre),
              },
            }
          })
          localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
          return nextSongs
        })
        updateProgress('metadata', song, index + 1, songs.length)
      }
      finishProgress('metadata', songs.length)
    } catch (err) {
      if (abortController.signal.aborted || err?.name === 'AbortError') {
        stopProgress('metadata')
      } else {
        setToolError(err?.message || 'Could not fetch AI metadata')
        setJobProgress(null)
      }
    } finally {
      if (jobAbortControllerRef.current === abortController) {
        jobAbortControllerRef.current = null
      }
      setIsFetchingMetadata(false)
    }
  }

  const clearFetchedMetadata = async () => {
    if (!selectedAddedSongs.length || isClearingMetadata) return

    setToolError('')
    setIsClearingMetadata(true)
    try {
      const { json: payload } = await httpClient('/api/ai/clear-metadata', {
        method: 'POST',
        body: JSON.stringify({
          songs: selectedAddedSongs.map((song) => ({
            id: song.id,
            album: Boolean(song.aiFields?.album),
            year: Boolean(song.aiFields?.year),
          })),
        }),
      })
      const cleared = new Set(payload.songIds || [])
      setAddedSongs((prev) => {
        const nextSongs = prev.map((song) =>
          cleared.has(song.id)
            ? {
                ...song,
                album: song.aiFields?.album ? '[Unknown Album]' : song.album,
                year: song.aiFields?.year ? 0 : song.year,
                aiGenre: '',
                metadataConfidence: {},
                aiFields: {
                  ...(song.aiFields || {}),
                  album: false,
                  year: false,
                  aiGenre: false,
                },
              }
            : song,
        )
        localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
        return nextSongs
      })
    } catch (err) {
      setToolError(err?.message || 'Could not clear fetched metadata')
    } finally {
      setIsClearingMetadata(false)
    }
  }

  const runModelAction = async () => {
    const action = modelDialogAction
    const songs = modelDialogSongs
    const provider = modelDialogProvider
    closeModelDialog()

    if (action === 'classifyExplicit') {
      await classifyExplicit(songs, provider)
    } else if (action === 'fetchMetadata') {
      await fetchAIMetadataForSongs(songs, provider)
    }
  }

  const runRowAction = async (action) => {
    const song = rowActionSong
    closeRowActions()
    if (!song) return

    if (action === 'fetchLyrics') {
      await fetchLyrics(song)
    } else if (action === 'showLyrics') {
      await showLyrics(song)
    } else if (action === 'fetchMetadata') {
      openModelDialog('fetchMetadata', [song])
    } else if (action === 'removeSong') {
      removeSong(song.id)
    }
  }

  return (
    <Box className={classes.root}>
      <Title title={translate('menu.aiTool.name', { _: 'AI Tool' })} />

      <Card>
        <CardContent>
          <Box className={classes.providerToolbar}>
            <Typography variant="h6">
              {translate('menu.aiTool.importSong', { _: 'Import song' })}
            </Typography>
            <TextField
              select
              className={classes.defaultProviderSelect}
              label={translate('menu.aiTool.defaultAIProvider', {
                _: 'Default AI Provider',
              })}
              variant="outlined"
              size="small"
              value={defaultProvider}
              onChange={(event) => setDefaultProvider(normalizeAIProvider(event.target.value))}
            >
              {AI_PROVIDERS.map((provider) => (
                <MenuItem key={provider.id} value={provider.id}>
                  {provider.label}
                </MenuItem>
              ))}
            </TextField>
            <Box className={classes.serviceStatusPanel}>
              <button
                type="button"
                className={classes.serviceStatusHeader}
                onClick={() => setIsStatusOpen((open) => !open)}
                aria-expanded={isStatusOpen}
              >
                <Typography component="span">AI model status</Typography>
                <Typography component="span" className={classes.serviceStatusSummary}>
                  {areStatusesChecking
                    ? 'Checking…'
                    : `${onlineServiceCount}/${AI_SERVICES.length} online`}{' '}
                  {isStatusOpen ? '−' : '+'}
                </Typography>
              </button>
              <Collapse in={isStatusOpen}>
                <Box className={classes.serviceStatusList}>
                  {modelStatuses.map((service) => {
                    const statusLabel =
                      service.online === null ? 'Checking…' : service.online ? 'Online' : 'Offline'
                    return (
                      <Box className={classes.serviceStatusItem} key={service.id}>
                        <Typography className={classes.serviceStatusName} variant="body2">
                          {service.label}
                        </Typography>
                        <Typography
                          component="span"
                          className={classes.serviceStatusValue}
                          style={{
                            color:
                              service.online === true
                                ? '#3ddc84'
                                : service.online === false
                                  ? '#ff8fc6'
                                  : '#c9d1dc',
                          }}
                        >
                          <span
                            className={`${classes.statusDot} ${
                              service.online === true
                                ? classes.statusDotOnline
                                : service.online === false
                                  ? classes.statusDotOffline
                                  : ''
                            }`}
                          />
                          {statusLabel}
                        </Typography>
                      </Box>
                    )
                  })}
                </Box>
              </Collapse>
            </Box>
          </Box>
          <Box className={classes.tableActions}>
            <Button variant="outlined" color="primary" onClick={openAddSongsDialog}>
              {translate('menu.aiTool.addSongs', { _: 'Add songs' })}
            </Button>
            <Button
              variant="outlined"
              color="primary"
              onClick={() => openModelDialog('classifyExplicit', selectedAddedSongs)}
              disabled={!selectedAddedIds.length || isClassifyingExplicit || isFetchJobRunning}
            >
              {isClassifyingExplicit
                ? translate('menu.aiTool.classifyingExplicit', { _: 'Classifying...' })
                : translate('menu.aiTool.classifyExplicit', { _: 'Classify Explicit' })}
            </Button>
            <Button
              variant="outlined"
              color="primary"
              onClick={fetchSelectedLyrics}
              disabled={!selectedAddedIds.length || isFetchJobRunning}
            >
              {lyricsLoadingId ? (
                <CircularProgress size={14} color="inherit" className={classes.buttonProgress} />
              ) : null}
              {lyricsLoadingId === 'bulk'
                ? translate('menu.aiTool.fetchingLyrics', { _: 'Fetching...' })
                : translate('menu.aiTool.fetchLyrics', { _: 'Fetch Lyrics' })}
            </Button>
            <Button
              variant="outlined"
              color="primary"
              onClick={() => openModelDialog('fetchMetadata', selectedAddedSongs)}
              disabled={!selectedAddedIds.length || isFetchJobRunning || isClassifyingExplicit}
            >
              {isFetchingMetadata ? (
                <CircularProgress size={14} color="inherit" className={classes.buttonProgress} />
              ) : null}
              {isFetchingMetadata
                ? translate('menu.aiTool.fetchingMetadata', { _: 'Fetching...' })
                : translate('menu.aiTool.fetchAIMetadata', { _: 'Fetch AI Metadata' })}
            </Button>
            <Button
              variant="outlined"
              color="primary"
              onClick={(event) => setColumnMenuAnchorEl(event.currentTarget)}
              startIcon={<ViewColumnIcon />}
            >
              {translate('ra.toggleFieldsMenu.columnsToDisplay', { _: 'Columns' })}
            </Button>
            <Button
              variant="outlined"
              color="primary"
              onClick={clearFetchedMetadata}
              disabled={
                !selectedAddedIds.length ||
                isClearingMetadata ||
                isFetchJobRunning ||
                isClassifyingExplicit
              }
            >
              {isClearingMetadata ? (
                <CircularProgress size={14} color="inherit" className={classes.buttonProgress} />
              ) : null}
              {isClearingMetadata
                ? translate('menu.aiTool.clearingMetadata', { _: 'Clearing...' })
                : translate('menu.aiTool.clearFetchedMetadata', { _: 'Clear Fetched Metadata' })}
            </Button>
            <Button
              variant="outlined"
              color="primary"
              onClick={removeSelectedSongs}
              disabled={!selectedAddedIds.length}
            >
              {translate('ra.action.remove', { _: 'Remove' })}
            </Button>
            {selectedAddedIds.length ? (
              <Typography variant="body2">
                {selectedAddedIds.length} selected
              </Typography>
            ) : null}
          </Box>

          {jobProgress ? (
            <Box className={classes.progressPanel}>
              <Box className={classes.progressHeader}>
                <Typography variant="body2" className={classes.progressText}>
                  {jobProgress.type === 'lyrics' ? 'Fetching lyrics' : 'Fetching AI metadata'}:
                  {' '}
                  {jobProgress.currentTitle}
                </Typography>
                <Box className={classes.progressActions}>
                  <Typography variant="body2" className={classes.progressMeta}>
                    {jobProgress.done}/{jobProgress.total} done,{' '}
                    {Math.max(jobProgress.total - jobProgress.done, 0)} left
                  </Typography>
                  {isFetchJobRunning ? (
                    <Button
                      className={classes.stopJobButton}
                      variant="outlined"
                      color="secondary"
                      size="small"
                      onClick={stopFetchJob}
                      startIcon={<StopIcon fontSize="small" />}
                    >
                      {translate('ra.action.stop', { _: 'Stop' })}
                    </Button>
                  ) : null}
                </Box>
              </Box>
              <LinearProgress
                className={classes.progressBar}
                variant="determinate"
                value={jobProgressValue}
              />
            </Box>
          ) : null}

          <Box className={classes.tableWrap}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell padding="checkbox">
                    <Checkbox
                      checked={addedSongs.length > 0 && selectedAddedIds.length === addedSongs.length}
                      indeterminate={
                        selectedAddedIds.length > 0 && selectedAddedIds.length < addedSongs.length
                      }
                      onChange={toggleAllAddedSongs}
                    />
                  </TableCell>
                  {isColumnVisible('title') ? (
                    <TableCell>{translate('resources.song.fields.title', { _: 'Title' })}</TableCell>
                  ) : null}
                  {isColumnVisible('album') ? (
                    <TableCell>{translate('resources.song.fields.album', { _: 'Album' })}</TableCell>
                  ) : null}
                  {isColumnVisible('albumConfidence') ? (
                    <TableCell className={classes.confidenceColumn}>Album Confidence</TableCell>
                  ) : null}
                  {isColumnVisible('artist') ? (
                    <TableCell>{translate('resources.song.fields.artist', { _: 'Artist' })}</TableCell>
                  ) : null}
                  {isColumnVisible('year') ? (
                    <TableCell>{translate('resources.song.fields.year', { _: 'Year' })}</TableCell>
                  ) : null}
                  {isColumnVisible('yearConfidence') ? (
                    <TableCell className={classes.confidenceColumn}>Year Confidence</TableCell>
                  ) : null}
                  {isColumnVisible('explicit') ? (
                    <TableCell>{translate('resources.song.fields.explicitStatus', { _: 'Explicit' })}</TableCell>
                  ) : null}
                  {isColumnVisible('lyrics') ? (
                    <TableCell>{translate('menu.aiTool.lyrics', { _: 'Lyrics' })}</TableCell>
                  ) : null}
                  {isColumnVisible('duration') ? (
                    <TableCell>{translate('resources.song.fields.duration', { _: 'Time' })}</TableCell>
                  ) : null}
                  {isColumnVisible('genre') ? (
                    <TableCell>{translate('resources.song.fields.genre', { _: 'Genre' })}</TableCell>
                  ) : null}
                  {isColumnVisible('aiGenre') ? (
                    <TableCell>{translate('menu.aiTool.aiGenre', { _: 'AI Genre' })}</TableCell>
                  ) : null}
                  {isColumnVisible('genreConfidence') ? (
                    <TableCell className={classes.confidenceColumn}>Genre Confidence</TableCell>
                  ) : null}
                  <TableCell>{translate('ra.action.actions', { _: 'Actions' })}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {addedSongs.map((song) => (
                  <TableRow key={song.id}>
                    <TableCell padding="checkbox">
                      <Checkbox
                        checked={selectedAddedSongIdSet.has(song.id)}
                        onChange={() => toggleAddedSong(song.id)}
                      />
                    </TableCell>
                    {isColumnVisible('title') ? (
                      <TableCell className={classes.valueExisting}>{song.title || ''}</TableCell>
                    ) : null}
                    {isColumnVisible('album') ? (
                      <TableCell className={valueClass(song, 'album')}>{song.album || ''}</TableCell>
                    ) : null}
                    {isColumnVisible('albumConfidence') ? (
                      <TableCell className={classes.confidenceColumn}>
                        {renderMetadataConfidence(song.metadataConfidence?.album) || '—'}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('artist') ? (
                      <TableCell className={classes.valueExisting}>{song.artist || ''}</TableCell>
                    ) : null}
                    {isColumnVisible('year') ? (
                      <TableCell className={valueClass(song, 'year')}>{song.year || ''}</TableCell>
                    ) : null}
                    {isColumnVisible('yearConfidence') ? (
                      <TableCell className={classes.confidenceColumn}>
                        {renderMetadataConfidence(song.metadataConfidence?.year) || '—'}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('explicit') ? (
                      <TableCell className={valueClass(song, 'explicitStatus')}>
                        {formatExplicitStatus(song.explicitStatus)}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('lyrics') ? (
                      <TableCell className={valueClass(song, 'lyrics')}>
                        {hasSavedLyrics(song)
                          ? translate('menu.aiTool.lyricsAvailable', { _: 'Available' })
                          : translate('menu.aiTool.lyricsMissing', { _: 'Missing' })}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('duration') ? (
                      <TableCell className={classes.valueExisting}>{formatDuration(song.duration)}</TableCell>
                    ) : null}
                    {isColumnVisible('genre') ? (
                      <TableCell className={classes.valueExisting}>{song.genre || ''}</TableCell>
                    ) : null}
                    {isColumnVisible('aiGenre') ? (
                      <TableCell className={valueClass(song, 'aiGenre')}>{song.aiGenre || '-'}</TableCell>
                    ) : null}
                    {isColumnVisible('genreConfidence') ? (
                      <TableCell className={classes.confidenceColumn}>
                        {renderMetadataConfidence(song.metadataConfidence?.genre) || '—'}
                      </TableCell>
                    ) : null}
                    <TableCell>
                      <IconButton
                        size="small"
                        onClick={(event) => openRowActions(event, song)}
                        aria-label={translate('ra.action.actions', { _: 'Actions' })}
                      >
                        <MoreVertIcon />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Box>
          {toolError ? (
            <Typography color="error" variant="body2">
              {toolError}
            </Typography>
          ) : null}
        </CardContent>
      </Card>

      <Menu
        anchorEl={columnMenuAnchorEl}
        keepMounted
        open={Boolean(columnMenuAnchorEl)}
        onClose={() => setColumnMenuAnchorEl(null)}
      >
        <Typography className={classes.columnMenuTitle} variant="body2">
          {translate('ra.toggleFieldsMenu.columnsToDisplay', { _: 'Columns to display' })}
        </Typography>
        <Box className={classes.columnMenuItems}>
          <MenuItem onClick={toggleAllConfidenceColumns}>
            <Checkbox
              checked={allConfidenceColumnsVisible}
              indeterminate={someConfidenceColumnsVisible && !allConfidenceColumnsVisible}
            />
            All Confidence Columns
          </MenuItem>
          {AI_TOOL_COLUMNS.map((column) => (
            <MenuItem key={column.id} onClick={() => toggleColumn(column.id)}>
              <Checkbox checked={isColumnVisible(column.id)} />
              {column.label}
            </MenuItem>
          ))}
        </Box>
      </Menu>

      <Menu
        anchorEl={rowActionAnchorEl}
        keepMounted
        open={Boolean(rowActionAnchorEl)}
        onClose={closeRowActions}
      >
        <MenuItem onClick={() => runRowAction('fetchLyrics')} disabled={isFetchJobRunning}>
          {lyricsLoadingId === rowActionSong?.id
            ? translate('menu.aiTool.fetchingLyrics', { _: 'Fetching...' })
            : translate('menu.aiTool.fetchLyrics', { _: 'Fetch Lyrics' })}
        </MenuItem>
        <MenuItem onClick={() => runRowAction('showLyrics')}>
          {translate('menu.aiTool.showLyrics', { _: 'Show Lyrics' })}
        </MenuItem>
        <MenuItem
          onClick={() => runRowAction('fetchMetadata')}
          disabled={isFetchJobRunning || isClassifyingExplicit}
        >
          {isFetchingMetadata
            ? translate('menu.aiTool.fetchingMetadata', { _: 'Fetching...' })
            : translate('menu.aiTool.fetchAIMetadata', { _: 'Fetch AI Metadata' })}
        </MenuItem>
        <MenuItem onClick={() => runRowAction('removeSong')}>
          {translate('menu.aiTool.removeSong', { _: 'Remove Song' })}
        </MenuItem>
      </Menu>

      <Dialog
        open={Boolean(modelDialogAction)}
        onClose={closeModelDialog}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {modelDialogAction === 'classifyExplicit'
            ? translate('menu.aiTool.classifyExplicit', { _: 'Classify Explicit' })
            : translate('menu.aiTool.fetchAIMetadata', { _: 'Fetch AI Metadata' })}
        </DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Choose the AI model for {modelDialogSongs.length}{' '}
            {modelDialogSongs.length === 1 ? 'song' : 'songs'}.
          </Typography>
          <TextField
            select
            fullWidth
            margin="normal"
            variant="outlined"
            label={translate('menu.aiTool.aiModel', { _: 'AI Model' })}
            value={modelDialogProvider}
            onChange={(event) => setModelDialogProvider(normalizeAIProvider(event.target.value))}
          >
            {AI_PROVIDERS.map((provider) => (
              <MenuItem key={provider.id} value={provider.id}>
                {provider.label}
              </MenuItem>
            ))}
          </TextField>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeModelDialog}>
            {translate('ra.action.cancel', { _: 'Cancel' })}
          </Button>
          <Button
            color="primary"
            variant="contained"
            onClick={runModelAction}
            disabled={isClassifyingExplicit || isFetchJobRunning}
          >
            {modelDialogAction === 'classifyExplicit'
              ? translate('menu.aiTool.classify', { _: 'Classify' })
              : translate('menu.aiTool.fetchMetadata', { _: 'Fetch Metadata' })}
          </Button>
        </DialogActions>
      </Dialog>

      {isChatOpen ? (
        <Box
          className={classes.chatWidget}
          style={isChatExpanded ? expandedChatFrame() : chatFrame}
        >
          <Box
            className={`${classes.chatResizeHandle} ${classes.chatResizeTopLeft}`}
            onMouseDown={(event) => startChatResize('top-left', event)}
            title="Resize chat"
          />
          <Box
            className={`${classes.chatResizeHandle} ${classes.chatResizeTopRight}`}
            onMouseDown={(event) => startChatResize('top-right', event)}
            title="Resize chat"
          />
          <Box
            className={`${classes.chatResizeHandle} ${classes.chatResizeBottomLeft}`}
            onMouseDown={(event) => startChatResize('bottom-left', event)}
            title="Resize chat"
          />
          <Box
            className={`${classes.chatResizeHandle} ${classes.chatResizeBottomRight}`}
            onMouseDown={(event) => startChatResize('bottom-right', event)}
            title="Resize chat"
          />
          <Box className={classes.chatHeader}>
            <Box className={classes.chatAvatar}>AI</Box>
            <Box className={classes.chatTitleWrap}>
              <Typography className={classes.chatTitle}>AI Chat</Typography>
              <Typography
                className={`${classes.chatStatus} ${
                  selectedModelOnline === true
                    ? classes.chatStatusOnline
                    : selectedModelOnline === false
                      ? classes.chatStatusOffline
                      : ''
                }`}
              >
                {aiProviderLabel(chatProvider)} -{' '}
                {selectedModelOnline === null || selectedModelOnline === undefined
                  ? 'Checking…'
                  : selectedModelOnline
                    ? 'Online'
                    : 'Offline'}
              </Typography>
            </Box>
            <Box className={classes.chatHeaderActions}>
              <Button
                className={classes.chatClose}
                variant="outlined"
                onClick={() => setIsChatExpanded((prev) => !prev)}
                title={isChatExpanded ? 'Collapse chat' : 'Expand chat'}
              >
                <AspectRatioIcon fontSize="small" />
              </Button>
              <Button
                className={classes.chatClose}
                variant="outlined"
                onClick={() => setIsChatOpen(false)}
                title="Close chat"
              >
                x
              </Button>
            </Box>
          </Box>
          <Box className={classes.chatMessages} ref={chatMessagesRef}>
            {messages.length === 0 ? (
              <Box className={classes.chatMessageRow}>
                <Box className={`${classes.chatBubble} ${classes.chatBubbleAssistant}`}>
                  Hi. What can I help with today?
                </Box>
              </Box>
            ) : (
              messages.map((message, index) => (
                <Box
                  key={`${message.role}-${index}`}
                  className={`${classes.chatMessageRow} ${
                    message.role === 'user' ? classes.chatMessageRowUser : ''
                  }`}
                >
                  <Box>
                    <Typography
                      className={classes.chatMessageLabel}
                      align={message.role === 'user' ? 'right' : 'left'}
                    >
                      {message.role === 'user' ? 'You' : aiProviderLabel(message.provider)}
                    </Typography>
                    <Box
                      className={`${classes.chatBubble} ${
                        message.role === 'user'
                          ? classes.chatBubbleUser
                          : classes.chatBubbleAssistant
                      }`}
                    >
                      {message.text}
                    </Box>
                  </Box>
                </Box>
              ))
            )}
            {isSending ? (
              <Box className={classes.chatMessageRow}>
                <Box>
                  <Typography className={classes.chatMessageLabel}>
                    {aiProviderLabel(chatProvider)}
                  </Typography>
                  <Box className={`${classes.chatBubble} ${classes.chatBubbleAssistant}`}>
                    <Box className={classes.chatThinking} component="span" title="Thinking">
                      <span />
                      <span />
                      <span />
                    </Box>
                  </Box>
                </Box>
              </Box>
            ) : null}
          </Box>
          <Box className={classes.chatInputBar}>
            <TextField
              select
              className={classes.chatModelSelect}
              variant="outlined"
              size="small"
              value={chatProvider}
              onChange={(event) => {
                setChatProvider(normalizeAIProvider(event.target.value))
                setChatProviderOverridden(true)
              }}
            >
              {AI_PROVIDERS.map((provider) => (
                <MenuItem key={provider.id} value={provider.id}>
                  {provider.label}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              fullWidth
              className={classes.chatInput}
              variant="outlined"
              size="small"
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              onKeyPress={(event) => {
                if (event.key === 'Enter') sendMessage()
              }}
              placeholder={translate('menu.aiTool.inputPlaceholder', {
                _: 'Ask AI anything...',
              })}
            />
            <Button
              className={isSending ? classes.chatStopButton : classes.chatSendButton}
              variant="contained"
              onClick={isSending ? stopMessage : sendMessage}
              title={isSending ? 'Stop response' : 'Send message'}
            >
              {isSending ? <StopIcon fontSize="small" /> : '>'}
            </Button>
          </Box>
          {chatError ? (
            <Typography className={classes.chatError} variant="body2">
              {chatError}
            </Typography>
          ) : null}
        </Box>
      ) : (
        <Button className={classes.chatLauncher} onClick={() => setIsChatOpen(true)}>
          AI
        </Button>
      )}

      <Dialog
        open={songDialogOpen}
        onClose={() => setSongDialogOpen(false)}
        fullWidth
        maxWidth="lg"
      >
        <DialogTitle>{translate('menu.aiTool.addSongs', { _: 'Add songs' })}</DialogTitle>
        <DialogContent>
          {songsLoading ? (
            <Typography>{translate('menu.retailPlayer.loading', { _: 'Loading devices…' })}</Typography>
          ) : (
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell padding="checkbox" />
                  <TableCell>{translate('resources.song.fields.title', { _: 'Title' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.album', { _: 'Album' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.artist', { _: 'Artist' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.year', { _: 'Year' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.explicitStatus', { _: 'Explicit' })}</TableCell>
                  <TableCell>{translate('menu.aiTool.lyrics', { _: 'Lyrics' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.duration', { _: 'Time' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.genre', { _: 'Genre' })}</TableCell>
                  <TableCell>{translate('menu.aiTool.aiGenre', { _: 'AI Genre' })}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {availableSongs.map((song) => (
                  <TableRow key={song.id} hover onClick={() => toggleSong(song.id)}>
                    <TableCell padding="checkbox">
                      <Checkbox checked={selectedSongIds.includes(song.id)} />
                    </TableCell>
                    <TableCell>{song.title || ''}</TableCell>
                    <TableCell>{song.album || ''}</TableCell>
                    <TableCell>{song.artist || ''}</TableCell>
                    <TableCell>{song.year || ''}</TableCell>
                    <TableCell>{formatExplicitStatus(song.explicitStatus)}</TableCell>
                    <TableCell>
                      {hasSavedLyrics(song)
                        ? translate('menu.aiTool.lyricsAvailable', { _: 'Available' })
                        : translate('menu.aiTool.lyricsMissing', { _: 'Missing' })}
                    </TableCell>
                    <TableCell>{formatDuration(song.duration)}</TableCell>
                    <TableCell>{song.genre || ''}</TableCell>
                    <TableCell>{song.aiGenre || '-'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSongDialogOpen(false)}>
            {translate('ra.action.cancel', { _: 'Cancel' })}
          </Button>
          <Button color="primary" variant="contained" onClick={addSelectedSongs}>
            {translate('menu.aiTool.addSelected', { _: 'Add selected songs' })}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={lyricsDialogOpen}
        onClose={() => setLyricsDialogOpen(false)}
        fullWidth
        maxWidth="md"
      >
        <DialogTitle>{lyricsDialogTitle}</DialogTitle>
        <DialogContent>
          <Typography style={{ whiteSpace: 'pre-wrap' }}>
            {lyricsText || translate('menu.aiTool.noLyrics', { _: 'No saved lyrics found.' })}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setLyricsDialogOpen(false)}>
            {translate('ra.action.close', { _: 'Close' })}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

export default AiToolPage
