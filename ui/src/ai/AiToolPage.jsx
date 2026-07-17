import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
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
  FormControlLabel,
  IconButton,
  LinearProgress,
  Menu,
  MenuItem,
  Switch,
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
import DeleteOutlineIcon from '@material-ui/icons/DeleteOutline'
import ExpandLessIcon from '@material-ui/icons/ExpandLess'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import LabelOutlinedIcon from '@material-ui/icons/LabelOutlined'
import LibraryMusicIcon from '@material-ui/icons/LibraryMusic'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import SettingsApplicationsIcon from '@material-ui/icons/SettingsApplications'
import { Title, useDataProvider, useTranslate } from 'react-admin'
import { httpClient } from '../dataProvider'

const ADDED_SONGS_STORAGE_KEY = 'aiToolAddedSongs'
const DEFAULT_AI_PROVIDER_STORAGE_KEY = 'aiToolDefaultProviderV3'
const AI_TOOL_COLUMNS_STORAGE_KEY = 'aiToolVisibleColumns'
const EXPLICIT_WORD_RULES_STORAGE_KEY = 'aiToolExplicitWordRules'
const AUTO_FETCH_ALL_LYRICS_STORAGE_KEY = 'aiToolAutoFetchAllLyrics'
const AUTO_FETCH_ALL_METADATA_STORAGE_KEY = 'aiToolAutoFetchAllMetadata'
const METADATA_AI_PROVIDER_STORAGE_KEY = 'aiToolMetadataAIProvider'
const METADATA_BATCH_SIZE_STORAGE_KEY = 'aiToolMetadataBatchSize'
const AI_CHAT_DEVELOPER_TRACE_STORAGE_KEY = 'aiToolChatDeveloperTrace'
const DEFAULT_AI_PROVIDER = 'deepseek-v3.2'
const EXPLICIT_AI_PROVIDER = 'deepseek-v3.2'
const EXPLICIT_WORD_RULES_VERSION = 2
const MAX_METADATA_BATCH_SIZE = 20
const DEFAULT_METADATA_BATCH_SIZE = 1
const METADATA_BATCH_SIZE_OPTIONS = Array.from(
  { length: MAX_METADATA_BATCH_SIZE },
  (_, index) => index + 1,
)
const normalizeMetadataBatchSize = (value) => {
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed)) return DEFAULT_METADATA_BATCH_SIZE
  return Math.min(Math.max(parsed, 1), MAX_METADATA_BATCH_SIZE)
}
const DEFAULT_WHISPER_MODEL = 'large-v3'
const AUTO_FETCH_ALL_LYRICS_INTERVAL_MS = 10 * 60 * 1000
const AUTO_FETCH_ALL_METADATA_INTERVAL_MS = 10 * 60 * 1000
const SERVER_LYRICS_LOADING_ID = 'server-job'

const AI_TOOL_COLUMNS = [
  { id: 'title', label: 'Title' },
  { id: 'album', label: 'Album' },
  { id: 'artist', label: 'Artist' },
  { id: 'year', label: 'Year' },
  { id: 'explicit', label: 'Explicit' },
  { id: 'lyrics', label: 'Lyrics' },
  { id: 'duration', label: 'Time' },
  { id: 'genre', label: 'Genre' },
  { id: 'spotifyGenre', label: 'Spotify Genre' },
  { id: 'musicBrainzGenre', label: 'iTunes Genre' },
  { id: 'aiGenre', label: 'AI Genre' },
  { id: 'aiSubgenre', label: 'AI Subgenre' },
  { id: 'genreConfidence', label: 'Genre Confidence' },
  { id: 'tokens', label: 'Tokens' },
]

const CONFIDENCE_COLUMN_IDS = ['genreConfidence']
const DEFAULT_AI_TOOL_COLUMN_VISIBILITY = Object.fromEntries(
  AI_TOOL_COLUMNS.map((column) => [column.id, true]),
)

const AI_PROVIDERS = [
  { id: 'gemini-2.5', label: 'Gemini 2.5' },
  { id: 'gemini-3.5', label: 'Gemini 3.5' },
  { id: 'deepseek-v3.2', label: 'DeepSeek V3.2' },
  { id: 'gemma-26b', label: 'Gemma 26B' },
  { id: 'gemma-3-4b', label: 'Gemma 3:4b' },
]

const AI_SERVICES = [
  { id: 'gemma-26b', label: 'Gemma 26B' },
  { id: 'gemma-3-4b', label: 'Gemma 3:4b' },
  { id: 'whisper', label: 'Whisper' },
  { id: 'gemini-2.5', label: 'Gemini 2.5' },
  { id: 'gemini-3.5', label: 'Gemini 3.5' },
  { id: 'deepseek-v3.2', label: 'DeepSeek V3.2' },
]

const WHISPER_MODELS = [
  { id: 'tiny', label: 'Tiny' },
  { id: 'base', label: 'Base' },
  { id: 'small', label: 'Small' },
  { id: 'medium', label: 'Medium' },
  { id: 'large-v3', label: 'Large v3' },
  { id: 'turbo', label: 'Turbo' },
]

const CHAT_MIN_WIDTH = 320
const CHAT_MIN_HEIGHT = 420
const CHAT_MARGIN = 16
const CHAT_DEFAULT_WIDTH = 360
const CHAT_DEFAULT_HEIGHT = 520
const DEFAULT_RAG_INDEX_LIMIT = 50
const MAX_RAG_INDEX_LIMIT = 500
const LEGACY_DEFAULT_EXPLICIT_INCLUDED_WORDS = [
  'fuck',
  'fucking',
  'motherfucker',
  'shit',
  'bitch',
  'cunt',
  'nigga',
  'nigger',
  'pussy',
  'dick',
  'cock',
]
const DEFAULT_EXPLICIT_INCLUDED_WORDS = [
  'fuck',
  'fucks',
  'fucked',
  'fucker',
  'fuckers',
  'fuckin',
  'fucking',
  'motherfuck',
  'motherfucker',
  'motherfuckers',
  'motherfucking',
  'shit',
  'shits',
  'shitty',
  'bullshit',
  'horseshit',
  'dipshit',
  'shithead',
  'bitch',
  'bitches',
  'cunt',
  'cunts',
  'nigga',
  'niggas',
  'nigger',
  'niggers',
  'faggot',
  'faggots',
  'asshole',
  'assholes',
  'cocksucker',
  'cocksuckers',
  'pussy',
  'dick',
  'cock',
  'tits',
  'whore',
  'whores',
  'slut',
  'sluts',
  'cum',
  'blowjob',
  'blow job',
  'handjob',
  'hand job',
]
const LEGACY_DEFAULT_EXPLICIT_EXCLUDED_WORDS = [
  'damn',
  'hell',
  'crap',
  'ass',
  'alcohol',
  'drunk',
  'weed',
  'marijuana',
  'kiss',
  'kissing',
  'sexy',
  'gun',
  'kill',
]
const DEFAULT_EXPLICIT_EXCLUDED_WORDS = [
  'damn',
  'goddamn',
  'hell',
  'crap',
  'ass',
  'bloody',
  'stupid',
  'idiot',
  'alcohol',
  'drunk',
  'weed',
  'marijuana',
  'kiss',
  'kissing',
  'sexy',
  'gun',
  'kill',
]
const DEFAULT_RAG_SEARCH_FILTERS = {
  cleanOnly: false,
  genre: '',
  lyricsContains: '',
  yearMin: '',
  yearMax: '',
  bpmMin: '',
  bpmMax: '',
  lufsMin: '',
  lufsMax: '',
  playCountMax: '',
  durationMax: '',
  hasLyrics: false,
  hasGenre: false,
  hasYear: false,
  hasBpm: false,
  hasLufs: false,
}

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

const defaultNormalChatFrame = () => {
  const frame = defaultChatFrame()
  if (typeof window === 'undefined') return frame
  return {
    ...frame,
    left: Math.max(
      window.innerWidth - CHAT_DEFAULT_WIDTH * 2 - 40,
      CHAT_MARGIN,
    ),
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
    case 'deepseek-v3.2':
    case 'deepseek.v3.2':
    case 'deepseek':
      return 'deepseek-v3.2'
    case 'gemini-3.5':
    case 'gemini-3.5-flash':
      return 'gemini-3.5'
    case 'gemma-26b':
    case 'gemma-26':
    case 'gemma-4':
      return 'gemma-26b'
    case 'gemma-3-4b':
    case 'gemma-3:4b':
    case 'gemma-3':
    case 'gemma3':
    case 'gemma3:4b':
      return 'gemma-3-4b'
    case 'gemini-2.5':
    case 'gemini-2.5-flash':
      return 'gemini-2.5'
    default:
      return DEFAULT_AI_PROVIDER
  }
}

const aiProviderLabel = (provider) =>
  AI_PROVIDERS.find((option) => option.id === normalizeAIProvider(provider))
    ?.label || 'AI'

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
    minWidth: 0,
    maxWidth: '100%',
    boxSizing: 'border-box',
    overflowX: 'hidden',
    '& > .MuiCard-root': {
      minWidth: 0,
      maxWidth: '100%',
      overflow: 'hidden',
    },
    '& .MuiCardContent-root': {
      minWidth: 0,
      maxWidth: '100%',
      boxSizing: 'border-box',
    },
  },
  section: {
    marginBottom: theme.spacing(2),
  },
  tableWrap: {
    marginTop: theme.spacing(2),
    width: '100%',
    minWidth: 0,
    maxWidth: '100%',
    overflowX: 'auto',
    overscrollBehaviorX: 'contain',
  },
  tableActions: {
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
    '& .MuiButton-root': {
      minWidth: 'auto',
      minHeight: 32,
      padding: theme.spacing(0.5, 1.25),
      borderRadius: 7,
      fontSize: 12,
      fontWeight: 600,
      lineHeight: 1.25,
      letterSpacing: 0.15,
      textTransform: 'none',
    },
    '& .MuiButton-startIcon': {
      marginRight: theme.spacing(0.5),
    },
  },
  songToolsPanel: {
    width: '100%',
    marginTop: theme.spacing(1.5),
    borderRadius: 8,
    border: '1px solid rgba(255, 255, 255, 0.12)',
    background: '#151f2d',
    overflow: 'hidden',
  },
  songToolsHeader: {
    width: '100%',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: theme.spacing(1, 1.5),
    color: '#f7f8fb',
    cursor: 'pointer',
    background: 'transparent',
    border: 0,
    textAlign: 'left',
    font: 'inherit',
  },
  songToolsSummary: {
    color: '#c9d1dc',
    fontSize: 13,
  },
  songToolsActions: {
    padding: theme.spacing(0, 1.5, 1.25),
  },
  songToolsActionButton: {
    borderColor: 'rgba(255, 42, 142, 0.52)',
    color: '#ff8fc6',
    background: 'rgba(255, 42, 142, 0.06)',
    '&:hover': {
      borderColor: '#ff2a8e',
      background: 'rgba(255, 42, 142, 0.15)',
    },
    '& .MuiButton-endIcon': {
      marginLeft: theme.spacing(0.25),
    },
  },
  songToolsActionButtonOpen: {
    borderColor: '#ff2a8e',
    color: '#ffffff',
    background: '#ff2a8e',
    '&:hover': {
      borderColor: '#e9197c',
      background: '#e9197c',
    },
  },
  songToolsMenu: {
    '& .MuiPaper-root': {
      width: 980,
      maxWidth: 'calc(100vw - 32px)',
      maxHeight: 'calc(100vh - 96px)',
      border: '1px solid rgba(255, 42, 142, 0.48)',
      borderRadius: 10,
      color: '#f7f8fb',
      background: '#151f2d',
      boxShadow:
        '0 20px 46px rgba(0, 0, 0, 0.46), 0 0 18px rgba(255, 42, 142, 0.12)',
      overflow: 'hidden',
    },
    '& .MuiMenu-list': {
      display: 'grid',
      gridTemplateColumns: 'repeat(4, minmax(180px, 1fr))',
      gap: theme.spacing(1),
      maxHeight: 'calc(100vh - 96px)',
      padding: theme.spacing(1),
      overflowY: 'auto',
    },
    [theme.breakpoints.down('sm')]: {
      '& .MuiPaper-root': {
        width: 'calc(100vw - 24px)',
      },
      '& .MuiMenu-list': {
        gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
      },
    },
  },
  songToolsMenuItem: {
    minHeight: 58,
    borderRadius: 8,
    margin: 0,
    padding: theme.spacing(1, 1.25),
    border: '1px solid rgba(255, 255, 255, 0.09)',
    color: '#f7f8fb',
    background: '#1f2b3a',
    fontSize: 13,
    fontWeight: 650,
    gap: theme.spacing(1),
    transition: 'background-color 140ms ease, color 140ms ease',
    '& .MuiSvgIcon-root': {
      color: '#ff8fc6',
    },
    '&:hover': {
      color: '#ffffff',
      borderColor: 'rgba(255, 42, 142, 0.62)',
      background: 'rgba(255, 42, 142, 0.16)',
      transform: 'none',
    },
    '&:hover .MuiSvgIcon-root': {
      color: '#ff2a8e',
    },
    '&.Mui-disabled': {
      color: '#7f8997',
      background: '#192331',
      opacity: 0.72,
    },
    '&.Mui-disabled .MuiSvgIcon-root': {
      color: '#7f8997',
    },
  },
  songToolsMenuItemPrimary: {
    color: '#ff8fc6',
    borderColor: 'rgba(255, 42, 142, 0.48)',
    background: 'rgba(255, 42, 142, 0.11)',
    '& .MuiSvgIcon-root': {
      color: '#ff2a8e',
    },
    '&:hover': {
      color: '#ffffff',
      background: '#ff2a8e',
    },
    '&:hover .MuiSvgIcon-root': {
      color: '#ffffff',
    },
  },
  songToolsMenuControl: {
    display: 'block',
    minHeight: 0,
    margin: 0,
    padding: 0,
    borderRadius: 8,
    cursor: 'default',
    '&:hover': {
      background: 'transparent',
    },
  },
  songToolsMenuToggle: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    minHeight: 54,
    padding: theme.spacing(0.75, 1.25),
    border: '1px solid rgba(255, 255, 255, 0.09)',
    borderRadius: 8,
    background: '#1f2b3a',
    boxSizing: 'border-box',
  },
  songToolsMenuToggleEnabled: {
    borderColor: 'rgba(255, 42, 142, 0.58)',
    background: 'rgba(255, 42, 142, 0.13)',
  },
  songToolsMenuToggleText: {
    minWidth: 0,
  },
  songToolsMenuToggleLabel: {
    color: '#f7f8fb',
    fontSize: 13,
    fontWeight: 700,
  },
  songToolsMenuToggleHint: {
    marginTop: 2,
    color: '#aeb8c5',
    fontSize: 11,
    lineHeight: 1.35,
  },
  songToolsMenuToggleAction: {
    display: 'flex',
    alignItems: 'center',
    flexShrink: 0,
    gap: theme.spacing(0.25),
  },
  songToolsMenuToggleStatus: {
    color: '#ff8fc6',
    fontSize: 11,
    fontWeight: 700,
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
  songToolsMenuSelect: {
    width: '100%',
    minWidth: 0,
    '& .MuiOutlinedInput-root': {
      color: '#f7f8fb',
      background: '#111b28',
      borderRadius: 8,
    },
    '& .MuiInputLabel-root': {
      color: '#ff8fc6',
    },
    '& .MuiOutlinedInput-notchedOutline': {
      borderColor: 'rgba(255, 42, 142, 0.45)',
    },
    '& .MuiOutlinedInput-root:hover .MuiOutlinedInput-notchedOutline': {
      borderColor: '#ff2a8e',
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
    flexWrap: 'wrap',
    gap: theme.spacing(0.75, 1.5),
    padding: theme.spacing(1.25, 1.5),
    color: '#ffffff',
  },
  serviceStatusHeaderTitle: {
    fontWeight: 600,
    whiteSpace: 'nowrap',
  },
  serviceStatusRibbon: {
    display: 'inline-flex',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: theme.spacing(0.5),
  },
  serviceStatusRibbonItem: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    padding: theme.spacing(0.25, 0.75),
    border: '1px solid rgba(255, 255, 255, 0.12)',
    borderRadius: 12,
    background: 'rgba(15, 23, 34, 0.72)',
    fontSize: 12,
    whiteSpace: 'nowrap',
  },
  serviceStatusRibbonLabel: {
    color: '#c9d1dc',
  },
  serviceStatusSummary: {
    marginLeft: 'auto',
    color: '#c9d1dc',
    fontSize: 13,
    whiteSpace: 'nowrap',
  },
  serviceStatusList: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(150px, 1fr))',
    gap: theme.spacing(1),
    padding: theme.spacing(0, 1.5, 1.5),
  },
  serviceStatusContent: {
    width: '100%',
  },
  ragControlsHeader: {
    width: '100%',
    display: 'flex',
    alignItems: 'center',
    padding: theme.spacing(1, 1.5),
    border: 0,
    borderTop: '1px solid rgba(255, 255, 255, 0.1)',
    color: '#f7f8fb',
    background: 'transparent',
    cursor: 'pointer',
    font: 'inherit',
    textAlign: 'left',
  },
  ragControlsSummary: {
    marginLeft: 'auto',
    color: '#c9d1dc',
    fontSize: 13,
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
  ragStatusCard: {
    width: `calc(100% - ${theme.spacing(3)}px)`,
    display: 'flex',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: theme.spacing(1, 2),
    padding: theme.spacing(1.25, 1.5),
    borderRadius: 8,
    border: '1px solid rgba(255, 255, 255, 0.12)',
    color: '#f7f8fb',
    background: '#151f2d',
    margin: theme.spacing(0, 1.5, 1.5),
    boxSizing: 'border-box',
  },
  ragStatusEnabled: {
    color: '#3ddc84',
  },
  ragStatusDisabled: {
    color: '#ff8fc6',
  },
  ragStatusErrorText: {
    width: '100%',
    color: '#ff8fc6',
    fontSize: 12,
  },
  ragToggleButton: {
    marginLeft: 'auto',
  },
  ragIndexMessage: {
    width: '100%',
    color: '#3ddc84',
    fontSize: 12,
  },
  ragIndexControls: {
    width: '100%',
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  ragIndexLimitInput: {
    width: 150,
  },
  ragIndexHint: {
    color: '#c9d1dc',
    fontSize: 12,
  },
  fetchAllLyricsSwitch: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    padding: theme.spacing(0.25, 1),
    border: '1px solid #a855f7',
    borderRadius: 18,
    color: '#f3e8ff',
    background: 'rgba(168, 85, 247, 0.16)',
    '& .MuiFormControlLabel-root': {
      margin: 0,
    },
    '& .MuiFormControlLabel-label': {
      color: '#f3e8ff',
      fontSize: 13,
      fontWeight: 600,
    },
    '& .MuiSwitch-colorPrimary.Mui-checked': {
      color: '#e9d5ff',
    },
    '& .MuiSwitch-colorPrimary.Mui-checked + .MuiSwitch-track': {
      backgroundColor: '#c084fc',
    },
  },
  fetchAllLyricsSwitchEnabled: {
    borderColor: '#c084fc',
    background: 'rgba(147, 51, 234, 0.52)',
  },
  fetchAllLyricsSwitchStatus: {
    color: '#f3e8ff',
    fontSize: 12,
    fontWeight: 700,
  },
  ragSearchControls: {
    width: '100%',
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  ragSearchInput: {
    flex: '1 1 260px',
  },
  ragFilterControls: {
    width: '100%',
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  ragFilterInput: {
    width: 130,
  },
  ragFilterCheckbox: {
    marginRight: theme.spacing(1),
    color: '#c9d1dc',
  },
  ragAppliedFilters: {
    width: '100%',
    color: '#c9d1dc',
    fontSize: 12,
    wordBreak: 'break-word',
  },
  ragSearchResults: {
    width: '100%',
    display: 'grid',
    gap: theme.spacing(0.75),
  },
  ragSearchResult: {
    padding: theme.spacing(0.75, 1),
    borderRadius: 6,
    color: '#c9d1dc',
    background: '#0f1722',
    fontSize: 12,
  },
  ragStatusDetails: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(0.5, 2),
    color: '#c9d1dc',
    fontSize: 12,
    wordBreak: 'break-word',
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
  statusDotBusy: {
    background: '#c084fc',
    boxShadow: '0 0 8px rgba(192, 132, 252, 0.6)',
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
  lyricsFetchingRow: {
    backgroundColor: 'rgba(192, 132, 252, 0.16)',
    boxShadow: 'inset 3px 0 0 #c084fc',
    '& > .MuiTableCell-root': {
      backgroundColor: 'inherit',
    },
  },
  valueExisting: {
    color: '#f7f8fb',
  },
  valueAI: {
    color: '#ff2a8e',
  },
  genreTraceButton: {
    minWidth: 0,
    padding: 0,
    color: 'inherit',
    fontSize: 'inherit',
    lineHeight: 'inherit',
    textAlign: 'left',
    textTransform: 'none',
    justifyContent: 'flex-start',
    borderRadius: 0,
    borderBottom: '1px dashed currentColor',
  },
  tokenUsageButton: {
    minWidth: 0,
    padding: 0,
    color: 'inherit',
    fontSize: 'inherit',
    lineHeight: 'inherit',
    textTransform: 'none',
    borderRadius: 0,
    borderBottom: '1px dashed currentColor',
  },
  genreTraceAttempt: {
    marginTop: theme.spacing(1.5),
    padding: theme.spacing(1),
    borderRadius: 6,
    background: '#0f1722',
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
    minWidth: 96,
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
  chatSources: {
    maxWidth: '82%',
    marginTop: theme.spacing(0.75),
    padding: theme.spacing(0.75, 1),
    borderRadius: 8,
    color: '#c9d1dc',
    background: '#111b28',
    fontSize: 11,
  },
  chatSource: {
    display: 'block',
    marginTop: 2,
  },
  chatLyricSnippet: {
    display: 'block',
    marginTop: 2,
    color: '#f4b6d2',
    fontStyle: 'italic',
  },
  chatBubble: {
    maxWidth: '82%',
    borderRadius: 18,
    padding: theme.spacing(1.2, 1.5),
    lineHeight: 1.45,
    whiteSpace: 'pre-wrap',
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
  ragChatLauncher: {
    right: theme.spacing(3),
    bottom: theme.spacing(3),
  },
  normalChatLauncher: {
    right: theme.spacing(12),
    bottom: theme.spacing(3),
  },
  chatError: {
    color: '#ff8fc6',
    padding: theme.spacing(0, 1.5, 1.25),
    background: '#0f1722',
  },
  chatRAGWarning: {
    maxWidth: 310,
    marginTop: theme.spacing(0.75),
    color: '#f6c177',
    fontSize: 11,
  },
  chatAssistantContent: {
    width: '100%',
    maxWidth: '100%',
  },
  chatTraceToggleBar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: theme.spacing(0.25, 1.5),
    color: '#d7c6ff',
    background: '#101927',
    borderBottom: '1px solid rgba(255, 255, 255, 0.08)',
    '& .MuiFormControlLabel-label': {
      fontSize: 12,
    },
    '& .MuiSwitch-colorSecondary.Mui-checked': {
      color: '#a970ff',
    },
    '& .MuiSwitch-colorSecondary.Mui-checked + .MuiSwitch-track': {
      backgroundColor: '#7b3fc6',
    },
  },
  chatTraceHint: {
    color: '#8492a6',
    fontSize: 10,
  },
  chatTracePanel: {
    width: '100%',
    maxWidth: 680,
    marginTop: theme.spacing(0.75),
    color: '#d7deea',
    background: '#0d1520',
    border: '1px solid rgba(169, 112, 255, 0.35)',
    borderRadius: 8,
    overflow: 'hidden',
  },
  chatTraceToggle: {
    width: '100%',
    justifyContent: 'space-between',
    color: '#d7c6ff',
    textTransform: 'none',
    fontSize: 11,
    padding: theme.spacing(0.75, 1),
  },
  chatTraceBody: {
    maxHeight: 440,
    overflow: 'auto',
    padding: theme.spacing(0.75),
    borderTop: '1px solid rgba(255, 255, 255, 0.08)',
  },
  chatTraceSummary: {
    padding: theme.spacing(0.5, 0.75, 1),
    color: '#9ba9bc',
    fontSize: 10,
    wordBreak: 'break-all',
  },
  chatTraceStage: {
    marginBottom: theme.spacing(0.75),
    background: '#131e2c',
    borderRadius: 6,
    border: '1px solid rgba(255, 255, 255, 0.07)',
  },
  chatTraceStageButton: {
    width: '100%',
    justifyContent: 'flex-start',
    color: '#e7ebf2',
    textTransform: 'none',
    textAlign: 'left',
    fontSize: 11,
    padding: theme.spacing(0.75, 1),
  },
  chatTraceStageStatus: {
    display: 'inline-block',
    minWidth: 18,
    color: '#68d391',
  },
  chatTraceStageStatusWarning: {
    color: '#f6c177',
  },
  chatTraceStageStatusError: {
    color: '#ff7aa8',
  },
  chatTraceStageContent: {
    padding: theme.spacing(0, 1, 1),
    color: '#b8c3d2',
    fontSize: 10,
  },
  chatTraceSectionHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: theme.spacing(0.75),
    marginBottom: theme.spacing(0.25),
    color: '#d7c6ff',
    fontSize: 10,
    fontWeight: 700,
    textTransform: 'uppercase',
    letterSpacing: 0.4,
  },
  chatTraceCopy: {
    minWidth: 0,
    color: '#b98cff',
    fontSize: 9,
    padding: theme.spacing(0.25, 0.5),
    textTransform: 'none',
  },
  chatTracePre: {
    margin: 0,
    padding: theme.spacing(0.75),
    maxHeight: 240,
    overflow: 'auto',
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word',
    color: '#dce3ed',
    background: '#091019',
    borderRadius: 4,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: 9,
    lineHeight: 1.45,
  },
  chatTraceError: {
    color: '#ff8fc6',
  },
  chatErrorTrace: {
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
  const normalized = String(status || '')
    .trim()
    .toLowerCase()
  if (normalized === 'e' || normalized === 'explicit') return 'Explicit'
  if (normalized === 'c' || normalized === 'clean') return 'Clean'
  return ''
}

const isUnknownValue = (value) => {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
  return (
    normalized === '' ||
    normalized === 'unknown' ||
    normalized === 'unknown album' ||
    normalized === '[unknown album]'
  )
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

const parseExplicitWordList = (value) => [
  ...new Set(
    String(value || '')
      .split(/[,\n]/)
      .map((word) => word.trim().toLowerCase())
      .filter(Boolean),
  ),
]

const sameExplicitWordSet = (left, right) => {
  const leftWords = parseExplicitWordList(left)
  const rightSet = new Set(parseExplicitWordList(right))
  return (
    leftWords.length === rightSet.size &&
    leftWords.every((word) => rightSet.has(word))
  )
}

const loadExplicitWordRuleList = (field, defaults, legacyDefaults) => {
  try {
    const saved = JSON.parse(
      localStorage.getItem(EXPLICIT_WORD_RULES_STORAGE_KEY) || '{}',
    )
    if (!Array.isArray(saved[field])) return defaults
    const savedVersion = Number(saved.version)
    // Upgrade only an untouched legacy default. Custom rule sets are always
    // preserved exactly as the user saved them.
    if (
      (!Number.isFinite(savedVersion) ||
        savedVersion < EXPLICIT_WORD_RULES_VERSION) &&
      sameExplicitWordSet(saved[field], legacyDefaults)
    ) {
      return defaults
    }
    return parseExplicitWordList(saved[field])
  } catch {
    return defaults
  }
}

const formatTraceValue = (value) => {
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

const copyTraceText = (value) => {
  if (!value || !navigator?.clipboard?.writeText) return
  navigator.clipboard.writeText(value).catch(() => {})
}

const ChatTraceSection = ({ label, value, classes, copyLabel }) => {
  if (value === undefined || value === null || value === '') return null
  const text = formatTraceValue(value)
  return (
    <Box>
      <Box className={classes.chatTraceSectionHeader}>
        <span>{label}</span>
        {copyLabel ? (
          <Button
            className={classes.chatTraceCopy}
            size="small"
            onClick={() => copyTraceText(text)}
          >
            {copyLabel}
          </Button>
        ) : null}
      </Box>
      <pre className={classes.chatTracePre}>{text}</pre>
    </Box>
  )
}

const ChatTraceStage = ({ stage, classes }) => {
  const [open, setOpen] = useState(false)
  const failed = stage.status === 'failed'
  const warning = stage.status === 'fallback' || stage.status === 'skipped'
  const statusSymbol = failed ? '!' : warning ? '•' : '✓'
  const duration = Number(stage.durationMs || 0)

  return (
    <Box className={classes.chatTraceStage}>
      <Button
        className={classes.chatTraceStageButton}
        onClick={() => setOpen((previous) => !previous)}
        aria-expanded={open}
      >
        <span
          className={`${classes.chatTraceStageStatus} ${
            failed
              ? classes.chatTraceStageStatusError
              : warning
                ? classes.chatTraceStageStatusWarning
                : ''
          }`}
        >
          {statusSymbol}
        </span>
        {stage.label || stage.id}
        {duration > 0 ? ` · ${duration} ms` : ''}
      </Button>
      <Collapse in={open} unmountOnExit>
        <Box className={classes.chatTraceStageContent}>
          {stage.detail ? (
            <Typography variant="caption">{stage.detail}</Typography>
          ) : null}
          {stage.error ? (
            <Typography variant="caption" className={classes.chatTraceError}>
              {stage.error}
            </Typography>
          ) : null}
          <ChatTraceSection
            label="Input"
            value={stage.input}
            classes={classes}
          />
          <ChatTraceSection
            label="Prompt sent to AI"
            value={stage.prompt}
            classes={classes}
            copyLabel="Copy prompt"
          />
          <ChatTraceSection
            label="Raw AI response"
            value={stage.response}
            classes={classes}
            copyLabel="Copy response"
          />
          <ChatTraceSection
            label="Output"
            value={stage.output}
            classes={classes}
          />
        </Box>
      </Collapse>
    </Box>
  )
}

const ChatDeveloperTrace = ({ trace, classes }) => {
  const [open, setOpen] = useState(false)
  if (!trace) return null
  const stages = Array.isArray(trace.stages) ? trace.stages : []

  return (
    <Box className={classes.chatTracePanel} aria-label="Developer trace">
      <Button
        className={classes.chatTraceToggle}
        onClick={() => setOpen((previous) => !previous)}
        aria-expanded={open}
      >
        <span>Behind the scenes</span>
        <span>
          {stages.length} steps · {Number(trace.durationMs || 0)} ms
        </span>
      </Button>
      <Collapse in={open} unmountOnExit>
        <Box className={classes.chatTraceBody}>
          <Typography className={classes.chatTraceSummary}>
            Request {trace.requestId || '—'} · {trace.provider || 'unknown'} /{' '}
            {trace.model || 'unknown'} · RAG{' '}
            {trace.useRag ? 'enabled' : 'disabled'}
          </Typography>
          {stages.map((stage, index) => (
            <ChatTraceStage
              key={`${stage.id || 'stage'}-${index}`}
              stage={stage}
              classes={classes}
            />
          ))}
        </Box>
      </Collapse>
    </Box>
  )
}

const AiToolPage = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const dataProvider = useDataProvider()
  const chatMessagesRef = useRef(null)
  const chatAbortControllerRef = useRef(null)
  const normalChatMessagesRef = useRef(null)
  const normalChatAbortControllerRef = useRef(null)
  const lyricsAbortControllerRef = useRef(null)
  const metadataAbortControllerRef = useRef(null)
  const progressTimeoutRefs = useRef({ lyrics: null, metadata: null })
  const addedSongsRef = useRef([])
  const lyricsLoadingIdRef = useRef('')
  const serverLyricsJobActiveRef = useRef(false)
  const explicitClassifyingSongIdsRef = useRef(new Set())
  const fetchLyricsForSongsRef = useRef(null)
  const fetchAIMetadataForSongsRef = useRef(null)
  const applyLyricsJobStatusRef = useRef(null)
  const [messages, setMessages] = useState([])
  const [prompt, setPrompt] = useState('')
  const [normalMessages, setNormalMessages] = useState([])
  const [normalPrompt, setNormalPrompt] = useState('')
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
  const [isNormalSending, setIsNormalSending] = useState(false)
  const [defaultProvider, setDefaultProvider] = useState(() => {
    try {
      return normalizeAIProvider(
        localStorage.getItem(DEFAULT_AI_PROVIDER_STORAGE_KEY),
      )
    } catch {
      return DEFAULT_AI_PROVIDER
    }
  })
  const [whisperModel, setWhisperModel] = useState(DEFAULT_WHISPER_MODEL)
  const [isUpdatingWhisperModel, setIsUpdatingWhisperModel] = useState(false)
  const [chatProvider, setChatProvider] = useState(defaultProvider)
  const [chatProviderOverridden, setChatProviderOverridden] = useState(false)
  const [normalChatProvider, setNormalChatProvider] = useState(defaultProvider)
  const [normalChatProviderOverridden, setNormalChatProviderOverridden] =
    useState(false)
  const [chatError, setChatError] = useState('')
  const [normalChatError, setNormalChatError] = useState('')
  const [chatErrorTrace, setChatErrorTrace] = useState(null)
  const [normalChatErrorTrace, setNormalChatErrorTrace] = useState(null)
  const [developerTraceEnabled, setDeveloperTraceEnabled] = useState(() => {
    try {
      return (
        localStorage.getItem(AI_CHAT_DEVELOPER_TRACE_STORAGE_KEY) === 'true'
      )
    } catch {
      return false
    }
  })
  const [toolError, setToolError] = useState('')
  const [isChatOpen, setIsChatOpen] = useState(false)
  const [isChatExpanded, setIsChatExpanded] = useState(false)
  const [chatFrame, setChatFrame] = useState(defaultChatFrame)
  const [isNormalChatOpen, setIsNormalChatOpen] = useState(false)
  const [isNormalChatExpanded, setIsNormalChatExpanded] = useState(false)
  const [normalChatFrame, setNormalChatFrame] = useState(defaultNormalChatFrame)
  const [lyricsLoadingId, setLyricsLoadingId] = useState('')
  const [lyricsDialogOpen, setLyricsDialogOpen] = useState(false)
  const [lyricsDialogTitle, setLyricsDialogTitle] = useState('')
  const [lyricsText, setLyricsText] = useState('')
  const [lyricsDialogSong, setLyricsDialogSong] = useState(null)
  const [explicitReasonSong, setExplicitReasonSong] = useState(null)
  const [confidenceDetail, setConfidenceDetail] = useState(null)
  const [genreTraceDetail, setGenreTraceDetail] = useState(null)
  const [tokenUsageDetail, setTokenUsageDetail] = useState(null)
  const [explicitRulesOpen, setExplicitRulesOpen] = useState(false)
  const [explicitIncludedWords, setExplicitIncludedWords] = useState(() =>
    loadExplicitWordRuleList(
      'included',
      DEFAULT_EXPLICIT_INCLUDED_WORDS,
      LEGACY_DEFAULT_EXPLICIT_INCLUDED_WORDS,
    ).join(', '),
  )
  const [explicitExcludedWords, setExplicitExcludedWords] = useState(() =>
    loadExplicitWordRuleList(
      'excluded',
      DEFAULT_EXPLICIT_EXCLUDED_WORDS,
      LEGACY_DEFAULT_EXPLICIT_EXCLUDED_WORDS,
    ).join(', '),
  )
  const [isDeletingLyrics, setIsDeletingLyrics] = useState(false)
  const [isClassifyingExplicit, setIsClassifyingExplicit] = useState(false)
  const [explicitClassifyingSongIds, setExplicitClassifyingSongIds] = useState(
    [],
  )
  const [isFetchingMetadata, setIsFetchingMetadata] = useState(false)
  const [metadataProvider, setMetadataProvider] = useState(() => {
    try {
      return normalizeAIProvider(
        localStorage.getItem(METADATA_AI_PROVIDER_STORAGE_KEY),
      )
    } catch {
      return DEFAULT_AI_PROVIDER
    }
  })
  const [metadataBatchSize, setMetadataBatchSize] = useState(() => {
    try {
      return normalizeMetadataBatchSize(
        localStorage.getItem(METADATA_BATCH_SIZE_STORAGE_KEY),
      )
    } catch {
      return DEFAULT_METADATA_BATCH_SIZE
    }
  })
  const [isAutoFetchAllMetadataEnabled, setIsAutoFetchAllMetadataEnabled] =
    useState(() => {
      try {
        return (
          localStorage.getItem(AUTO_FETCH_ALL_METADATA_STORAGE_KEY) === 'true'
        )
      } catch {
        return false
      }
    })
  const [lyricsFetchingSongIds, setLyricsFetchingSongIds] = useState([])
  const [isAutoFetchAllLyricsEnabled, setIsAutoFetchAllLyricsEnabled] =
    useState(() => {
      try {
        return (
          localStorage.getItem(AUTO_FETCH_ALL_LYRICS_STORAGE_KEY) === 'true'
        )
      } catch {
        return false
      }
    })
  const [isClearingMetadata, setIsClearingMetadata] = useState(false)
  const [explicitDialogSongs, setExplicitDialogSongs] = useState([])
  const [explicitMenuAnchorEl, setExplicitMenuAnchorEl] = useState(null)
  const [metadataMenuAnchorEl, setMetadataMenuAnchorEl] = useState(null)
  const [columnMenuAnchorEl, setColumnMenuAnchorEl] = useState(null)
  const [visibleColumns, setVisibleColumns] = useState(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem(AI_TOOL_COLUMNS_STORAGE_KEY) || '{}',
      )
      return { ...DEFAULT_AI_TOOL_COLUMN_VISIBILITY, ...saved }
    } catch {
      return DEFAULT_AI_TOOL_COLUMN_VISIBILITY
    }
  })
  const [rowActionAnchorEl, setRowActionAnchorEl] = useState(null)
  const [rowActionSong, setRowActionSong] = useState(null)
  const [jobProgresses, setJobProgresses] = useState({
    lyrics: null,
    metadata: null,
  })
  const [isLyricsProgressHidden, setIsLyricsProgressHidden] = useState(false)
  const [isMetadataProgressHidden, setIsMetadataProgressHidden] =
    useState(false)
  const [progressClock, setProgressClock] = useState(() => Date.now())
  const [isRAGControlsOpen, setIsRAGControlsOpen] = useState(false)
  const [isSongToolsOpen, setIsSongToolsOpen] = useState(true)
  const [modelStatuses, setModelStatuses] = useState(() =>
    AI_SERVICES.map((service) => ({ ...service, online: null })),
  )
  const [ragStatus, setRAGStatus] = useState(null)
  const [ragStatusError, setRAGStatusError] = useState('')
  const [isTogglingRAG, setIsTogglingRAG] = useState(false)
  const [isIndexingRAG, setIsIndexingRAG] = useState(false)
  const [isRefreshingRAG, setIsRefreshingRAG] = useState(false)
  const [isClearingRAG, setIsClearingRAG] = useState(false)
  const [ragIndexLimit, setRAGIndexLimit] = useState(
    String(DEFAULT_RAG_INDEX_LIMIT),
  )
  const [ragIncludePlaylists, setRAGIncludePlaylists] = useState(false)
  const [ragIndexMessage, setRAGIndexMessage] = useState('')
  const [ragIndexError, setRAGIndexError] = useState('')
  const [isRAGDocumentsOpen, setIsRAGDocumentsOpen] = useState(false)
  const [isLoadingRAGDocuments, setIsLoadingRAGDocuments] = useState(false)
  const [ragDocuments, setRAGDocuments] = useState([])
  const [ragDocumentsCollection, setRAGDocumentsCollection] = useState('')
  const [ragDocumentsCount, setRAGDocumentsCount] = useState(0)
  const [ragDocumentsError, setRAGDocumentsError] = useState('')
  const [ragDocumentDetails, setRAGDocumentDetails] = useState(null)
  const [isAddingQdrantLyrics, setIsAddingQdrantLyrics] = useState(false)
  const [isQdrantLyricsOpen, setIsQdrantLyricsOpen] = useState(false)
  const [isLoadingQdrantLyrics, setIsLoadingQdrantLyrics] = useState(false)
  const [qdrantLyrics, setQdrantLyrics] = useState([])
  const [qdrantLyricsCollection, setQdrantLyricsCollection] = useState('')
  const [qdrantLyricsQuery, setQdrantLyricsQuery] = useState('')
  const [qdrantLyricsCount, setQdrantLyricsCount] = useState(0)
  const [qdrantLyricsError, setQdrantLyricsError] = useState('')
  const [qdrantLyricDetails, setQdrantLyricDetails] = useState(null)
  const [ragSearchQuery, setRAGSearchQuery] = useState('')
  const [ragSearchResults, setRAGSearchResults] = useState([])
  const [ragSearchFilters, setRAGSearchFilters] = useState(
    DEFAULT_RAG_SEARCH_FILTERS,
  )
  const [ragAppliedFilters, setRAGAppliedFilters] = useState(null)
  const [ragSearchCount, setRAGSearchCount] = useState(0)
  const [isSearchingRAG, setIsSearchingRAG] = useState(false)
  const [ragSearchError, setRAGSearchError] = useState('')

  const parsedRAGIndexLimit = Number(ragIndexLimit)
  const isRAGIndexLimitValid =
    Number.isInteger(parsedRAGIndexLimit) &&
    parsedRAGIndexLimit >= 1 &&
    parsedRAGIndexLimit <= MAX_RAG_INDEX_LIMIT

  const loadRAGStatus = useCallback(async () => {
    try {
      const { json } = await httpClient('/api/ai/rag/status')
      setRAGStatus({
        enabled: json?.enabled === true,
        vectorUrl: json?.vectorUrl || '',
        collection: json?.collection || '',
        topK: Number(json?.topK) || 0,
        vectorDbOnline: json?.vectorDbOnline === true,
        collectionExists: json?.collectionExists === true,
        indexedCount: Number(json?.indexedCount) || 0,
        embeddingBackend: json?.embeddingBackend || 'unconfigured',
        embeddingModel: json?.embeddingModel || '',
        embeddingLocal: json?.embeddingLocal === true,
        offlineMode: json?.offlineMode === true,
        error: json?.error || '',
      })
      setRAGStatusError('')
    } catch (error) {
      setRAGStatus(null)
      setRAGStatusError(error?.message || 'Could not load RAG status')
    }
  }, [])

  const toggleRAG = async () => {
    if (!ragStatus || isTogglingRAG) return
    const enabled = !ragStatus.enabled
    setIsTogglingRAG(true)
    setRAGStatusError('')
    try {
      const { json } = await httpClient('/api/ai/rag/enabled', {
        method: 'POST',
        body: JSON.stringify({ enabled }),
      })
      setRAGStatus((current) => ({
        ...current,
        enabled: json?.enabled === true,
        vectorDbOnline: json?.enabled === true ? current.vectorDbOnline : false,
        collectionExists:
          json?.enabled === true ? current.collectionExists : false,
        indexedCount: json?.enabled === true ? current.indexedCount : 0,
      }))
      if (json?.enabled === true) await loadRAGStatus()
    } catch (error) {
      setRAGStatusError(error?.message || 'Could not change RAG status')
    } finally {
      setIsTogglingRAG(false)
    }
  }

  const updateWhisperModel = async (model) => {
    if (isUpdatingWhisperModel) return
    setIsUpdatingWhisperModel(true)
    try {
      const { json } = await httpClient('/api/ai/whisper/model', {
        method: 'POST',
        body: JSON.stringify({ model }),
      })
      setWhisperModel(json?.model || model)
    } catch (error) {
      setToolError(error?.message || 'Could not change Whisper model')
    } finally {
      setIsUpdatingWhisperModel(false)
    }
  }

  const indexRAGSongs = async (force = false) => {
    if (
      !ragStatus?.enabled ||
      isIndexingRAG ||
      isRefreshingRAG ||
      isAddingQdrantLyrics ||
      isClearingRAG
    )
      return

    if (!isRAGIndexLimitValid) {
      setRAGIndexError(
        `Choose a whole number between 1 and ${MAX_RAG_INDEX_LIMIT}.`,
      )
      return
    }

    if (force) setIsRefreshingRAG(true)
    else setIsIndexingRAG(true)
    setRAGIndexMessage('')
    setRAGIndexError('')
    try {
      const { json } = await httpClient('/api/ai/rag/index', {
        method: 'POST',
        body: JSON.stringify({
          limit: parsedRAGIndexLimit,
          force,
          ...(ragIncludePlaylists ? { includePlaylists: true } : {}),
        }),
      })
      const playlistMessage = json?.playlists
        ? ` Playlists: indexed ${Number(json.playlists.indexed) || 0}, skipped ${
            Number(json.playlists.skipped) || 0
          }, failed ${Number(json.playlists.failed) || 0}.`
        : ''
      setRAGIndexMessage(
        `${force ? 'Refreshed' : 'Indexed'} ${Number(json?.indexed) || 0}, skipped ${
          Number(json?.skipped) || 0
        }, failed ${Number(json?.failed) || 0}.${playlistMessage}`,
      )
      if (json?.error) setRAGIndexError(json.error)
      await loadRAGStatus()
    } catch (error) {
      setRAGIndexError(error?.message || 'Could not index songs')
    } finally {
      if (force) setIsRefreshingRAG(false)
      else setIsIndexingRAG(false)
    }
  }

  const clearRAGIndex = async () => {
    if (!ragStatus?.enabled || !ragStatus?.vectorDbOnline || isClearingRAG)
      return
    const collection = ragStatus.collection || 'the configured collection'
    const confirmed = window.confirm(
      `Clear all indexed RAG data from ${collection}? This only deletes the vector index; your Navidrome songs and files stay untouched.`,
    )
    if (!confirmed) return

    setIsClearingRAG(true)
    setRAGIndexMessage('')
    setRAGIndexError('')
    try {
      const { json } = await httpClient('/api/ai/rag/index', {
        method: 'DELETE',
      })
      setRAGDocuments([])
      setRAGDocumentsCount(0)
      setQdrantLyrics([])
      setQdrantLyricsCount(0)
      setRAGIndexMessage(
        `Cleared indexed songs from ${json?.collection || collection}. Reindex when ready.`,
      )
      await loadRAGStatus()
    } catch (error) {
      setRAGIndexError(error?.message || 'Could not clear indexed songs')
    } finally {
      setIsClearingRAG(false)
    }
  }

  const openRAGDocuments = async () => {
    setIsRAGDocumentsOpen(true)
    setIsLoadingRAGDocuments(true)
    setRAGDocumentsError('')
    try {
      const { json } = await httpClient('/api/ai/rag/documents?limit=100')
      setRAGDocuments(Array.isArray(json?.songs) ? json.songs : [])
      setRAGDocumentsCollection(json?.collection || '')
      setRAGDocumentsCount(Number(json?.indexedCount) || 0)
    } catch (error) {
      setRAGDocuments([])
      setRAGDocumentsError(
        error?.message || 'Could not load indexed songs from Qdrant',
      )
    } finally {
      setIsLoadingRAGDocuments(false)
    }
  }

  const addQdrantLyrics = async () => {
    if (
      !ragStatus?.enabled ||
      isAddingQdrantLyrics ||
      isIndexingRAG ||
      isRefreshingRAG ||
      isClearingRAG
    )
      return

    setIsAddingQdrantLyrics(true)
    setRAGIndexMessage('')
    setRAGIndexError('')
    try {
      const { json } = await httpClient('/api/ai/rag/lyrics', {
        method: 'POST',
        body: JSON.stringify({ limit: MAX_RAG_INDEX_LIMIT }),
      })
      setRAGIndexMessage(
        `Added or updated ${Number(json?.indexed) || 0} Qdrant lyrics, skipped ${
          Number(json?.skipped) || 0
        }, failed ${Number(json?.failed) || 0}.`,
      )
      if (json?.error) setRAGIndexError(json.error)
      await loadRAGStatus()
    } catch (error) {
      setRAGIndexError(error?.message || 'Could not add lyrics to Qdrant')
    } finally {
      setIsAddingQdrantLyrics(false)
    }
  }

  const loadQdrantLyrics = async (query = '') => {
    const normalizedQuery = String(query || '').trim()
    setIsQdrantLyricsOpen(true)
    setIsLoadingQdrantLyrics(true)
    setQdrantLyricsError('')
    try {
      const queryString = normalizedQuery
        ? `&query=${encodeURIComponent(normalizedQuery)}`
        : ''
      const { json } = await httpClient(
        `/api/ai/rag/lyrics?limit=${MAX_RAG_INDEX_LIMIT}${queryString}`,
      )
      setQdrantLyrics(Array.isArray(json?.songs) ? json.songs : [])
      setQdrantLyricsCollection(json?.collection || '')
      setQdrantLyricsCount(Number(json?.count) || 0)
    } catch (error) {
      setQdrantLyrics([])
      setQdrantLyricsCount(0)
      setQdrantLyricsError(
        error?.message || 'Could not load lyrics stored in Qdrant',
      )
    } finally {
      setIsLoadingQdrantLyrics(false)
    }
  }

  const openQdrantLyrics = () => {
    setQdrantLyricsQuery('')
    setQdrantLyricDetails(null)
    void loadQdrantLyrics('')
  }

  const searchQdrantLyrics = () => {
    void loadQdrantLyrics(qdrantLyricsQuery)
  }

  const searchRAGSongs = async () => {
    const query = ragSearchQuery.trim()
    if (!query || !ragStatus?.enabled || isSearchingRAG) return

    setIsSearchingRAG(true)
    setRAGSearchError('')
    setRAGSearchResults([])
    setRAGAppliedFilters(null)
    setRAGSearchCount(0)
    try {
      const filters = {}
      if (ragSearchFilters.cleanOnly) filters.explicit = 'clean'
      if (ragSearchFilters.genre.trim()) {
        filters.genre = ragSearchFilters.genre.trim()
      }
      if (ragSearchFilters.lyricsContains.trim()) {
        filters.lyricsContains = ragSearchFilters.lyricsContains.trim()
      }
      ;[
        'yearMin',
        'yearMax',
        'bpmMin',
        'bpmMax',
        'lufsMin',
        'lufsMax',
        'playCountMax',
        'durationMax',
      ].forEach((name) => {
        const rawValue = ragSearchFilters[name].trim()
        if (rawValue === '') return
        const value = Number(rawValue)
        if (Number.isFinite(value)) filters[name] = value
      })
      ;['hasLyrics', 'hasGenre', 'hasYear', 'hasBpm', 'hasLufs'].forEach(
        (name) => {
          if (ragSearchFilters[name]) filters[name] = true
        },
      )
      const { json } = await httpClient('/api/ai/rag/search', {
        method: 'POST',
        body: JSON.stringify({
          query,
          topK: ragStatus.topK || 20,
          filters,
        }),
      })
      setRAGSearchResults(Array.isArray(json?.results) ? json.results : [])
      setRAGAppliedFilters(json?.appliedFilters || {})
      setRAGSearchCount(Number(json?.count) || 0)
    } catch (error) {
      setRAGSearchError(error?.message || 'Could not search RAG')
    } finally {
      setIsSearchingRAG(false)
    }
  }

  const updateRAGSearchFilter = (name, value) => {
    setRAGSearchFilters((current) => ({ ...current, [name]: value }))
  }

  const addedSongIdSet = useMemo(
    () => new Set(addedSongs.map((song) => song.id)),
    [addedSongs],
  )

  const selectableSongIds = useMemo(
    () =>
      availableSongs
        .filter((song) => !addedSongIdSet.has(song.id))
        .map((song) => song.id),
    [availableSongs, addedSongIdSet],
  )

  const selectedSelectableSongCount = selectableSongIds.filter((id) =>
    selectedSongIds.includes(id),
  ).length

  const selectedSongs = useMemo(
    () =>
      availableSongs.filter(
        (song) =>
          selectedSongIds.includes(song.id) && !addedSongIdSet.has(song.id),
      ),
    [availableSongs, selectedSongIds, addedSongIdSet],
  )

  const selectedAddedSongIdSet = useMemo(
    () => new Set(selectedAddedSongIds),
    [selectedAddedSongIds],
  )

  const selectedAddedSongs = useMemo(
    () => addedSongs.filter((song) => selectedAddedSongIdSet.has(song.id)),
    [addedSongs, selectedAddedSongIdSet],
  )

  const selectedSongsWithLyrics = useMemo(
    () => selectedAddedSongs.filter(hasSavedLyrics),
    [selectedAddedSongs],
  )

  const selectedSongsMissingLyrics = useMemo(
    () => selectedAddedSongs.filter((song) => !hasSavedLyrics(song)),
    [selectedAddedSongs],
  )

  const selectedAddedIds = useMemo(
    () => selectedAddedSongs.map((song) => song.id),
    [selectedAddedSongs],
  )

  const lyricsFetchingSongIdSet = useMemo(
    () => new Set(lyricsFetchingSongIds),
    [lyricsFetchingSongIds],
  )
  const explicitClassifyingSongIdSet = useMemo(
    () => new Set(explicitClassifyingSongIds),
    [explicitClassifyingSongIds],
  )
  const selectedSongsAvailableForExplicit = useMemo(
    () =>
      selectedAddedSongs.filter(
        (song) => hasSavedLyrics(song) && !lyricsFetchingSongIdSet.has(song.id),
      ),
    [selectedAddedSongs, lyricsFetchingSongIdSet],
  )
  const selectedSongsAvailableForLyrics = useMemo(
    () =>
      selectedSongsMissingLyrics.filter(
        (song) => !explicitClassifyingSongIdSet.has(song.id),
      ),
    [selectedSongsMissingLyrics, explicitClassifyingSongIdSet],
  )
  const selectedSongsAvailableForLyricsDeletion = useMemo(
    () =>
      selectedSongsWithLyrics.filter(
        (song) =>
          !lyricsFetchingSongIdSet.has(song.id) &&
          !explicitClassifyingSongIdSet.has(song.id),
      ),
    [
      selectedSongsWithLyrics,
      lyricsFetchingSongIdSet,
      explicitClassifyingSongIdSet,
    ],
  )
  const lyricsJobProgress = jobProgresses.lyrics
  const lyricsProgressStatus = lyricsJobProgress?.status
  const metadataProgressStatus = jobProgresses.metadata?.status
  const lyricsProgressStartedAt = lyricsJobProgress?.startedAt
  const currentLyricsFetchingSongId =
    lyricsLoadingId === SERVER_LYRICS_LOADING_ID
      ? ['running', 'stopping'].includes(lyricsProgressStatus)
        ? lyricsJobProgress?.currentSongId || ''
        : ''
      : lyricsLoadingId
  const lyricsElapsedSeconds = lyricsJobProgress?.startedAt
    ? Math.max(
        0,
        Math.floor(
          ((lyricsJobProgress.finishedAt || progressClock) -
            lyricsJobProgress.startedAt) /
            1000,
        ),
      )
    : 0
  const lyricsTimingText = !lyricsJobProgress
    ? ''
    : lyricsJobProgress.status === 'complete'
      ? `Whisper finished fetching lyrics in ${lyricsElapsedSeconds} seconds`
      : lyricsJobProgress.status === 'failed'
        ? `Whisper stopped with an error after ${lyricsElapsedSeconds} seconds`
        : lyricsJobProgress.status === 'stopped'
          ? `Whisper stopped after ${lyricsElapsedSeconds} seconds`
          : `Whisper running for ${lyricsElapsedSeconds} seconds`
  const isLyricsJobRunning = Boolean(lyricsLoadingId)

  const selectedModelStatus = modelStatuses.find(
    (service) => service.id === normalizeAIProvider(chatProvider),
  )
  const selectedModelOnline = selectedModelStatus?.online
  const selectedNormalModelStatus = modelStatuses.find(
    (service) => service.id === normalizeAIProvider(normalChatProvider),
  )
  const selectedNormalModelOnline = selectedNormalModelStatus?.online
  const onlineServiceCount = modelStatuses.filter(
    (service) =>
      service.online === true ||
      (service.id === 'whisper' && isLyricsJobRunning),
  ).length
  const areStatusesChecking = modelStatuses.some(
    (service) => service.online === null,
  )

  const isAIValue = (song, field) => {
    if (song.aiFields?.[field]) return true
    if (field === 'aiGenre') return !isUnknownValue(song.aiGenre)
    if (field === 'spotifyGenre') return !isUnknownValue(song.spotifyGenre)
    if (field === 'musicBrainzGenre')
      return !isUnknownValue(song.musicBrainzGenre)
    if (field === 'lyrics') return hasSavedLyrics(song)
    if (field === 'explicitStatus')
      return Boolean(formatExplicitStatus(song.explicitStatus))
    return false
  }

  const valueClass = (song, field) =>
    isAIValue(song, field) ? classes.valueAI : classes.valueExisting

  const renderMetadataConfidence = (value, song, field) => {
    const confidence = normalizeMetadataConfidence(value)
    if (confidence === null) return null
    const color =
      confidence >= 80 ? '#3ddc84' : confidence >= 50 ? '#ffcb6b' : '#ff8fc6'
    const breakdown = song?.metadataConfidenceBreakdown
    const open = () =>
      setConfidenceDetail({ song, field, value: confidence, breakdown })
    return (
      <Typography
        component="span"
        role="button"
        tabIndex={0}
        className={classes.confidenceBadge}
        style={{
          color,
          cursor: 'pointer',
        }}
        title="Click to see how this score was calculated"
        onClick={open}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault()
            open()
          }
        }}
      >
        {confidence}%
      </Typography>
    )
  }

  const renderTokenUsage = (song) => {
    const usage = song?.aiTokens
    if (!usage || !usage.total) return '—'
    return (
      <Button
        className={classes.tokenUsageButton}
        size="small"
        aria-label={`View token breakdown for ${song.title || 'song'}`}
        title={`Input ${usage.input || 0} · Output ${usage.output || 0}`}
        onClick={(event) => {
          event.stopPropagation()
          setTokenUsageDetail({ song, usage })
        }}
      >
        {usage.total}
      </Button>
    )
  }

  const renderTokenUsageBreakdown = (detail) => {
    const input = Number(detail?.usage?.input) || 0
    const output = Number(detail?.usage?.output) || 0
    const total = Number(detail?.usage?.total) || 0
    const calculatedTotal = input + output
    return (
      <Box>
        <Typography variant="body2" paragraph>
          {input} input + {output} output = {calculatedTotal} calculated tokens.
        </Typography>
        <Table size="small">
          <TableBody>
            <TableRow>
              <TableCell>Input tokens</TableCell>
              <TableCell align="right">{input}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell>Output tokens</TableCell>
              <TableCell align="right">{output}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell>
                <strong>Total tokens shown</strong>
              </TableCell>
              <TableCell align="right">
                <strong>{total}</strong>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
        {total !== calculatedTotal ? (
          <Typography variant="body2" color="textSecondary" paragraph>
            The displayed total is the provider-reported total allocated to this
            song. Provider accounting and batch rounding can make it differ from
            input plus output.
          </Typography>
        ) : null}
      </Box>
    )
  }

  const renderLyricsState = (song) => {
    if (hasSavedLyrics(song)) {
      return (
        <Button
          size="small"
          color="primary"
          onClick={(event) => {
            event.stopPropagation()
            showLyrics(song)
          }}
        >
          Available
        </Button>
      )
    }
    return <Typography component="span">Failed</Typography>
  }

  const genreTraceSourceLabel = (source) =>
    source === 'itunes' ? 'iTunes' : 'AI'

  const renderFetchedGenre = (song, source) => {
    const value = source === 'itunes' ? song.musicBrainzGenre : song.aiGenre
    if (!value) return '-'
    const sourceLabel = genreTraceSourceLabel(source)
    return (
      <Button
        className={classes.genreTraceButton}
        size="small"
        aria-label={`View ${sourceLabel} genre developer trace for ${
          song.title || 'song'
        }`}
        title={`View ${sourceLabel} genre developer trace`}
        onClick={(event) => {
          event.stopPropagation()
          setGenreTraceDetail({
            song,
            source,
            trace: song.genreDeveloperTrace?.[source],
          })
        }}
      >
        {value}
      </Button>
    )
  }

  const renderGenreDeveloperTrace = (detail) => {
    const trace = detail?.trace
    if (!trace) {
      return (
        <Typography variant="body2" color="textSecondary">
          This genre was fetched before developer tracing was added. Run{' '}
          <strong>Fetch AI Metadata</strong> again to capture its request and
          response.
        </Typography>
      )
    }
    const attempts = Array.isArray(trace.attempts) ? trace.attempts : []
    return (
      <Box>
        <Typography variant="body2" paragraph>
          <strong>Fetched genre:</strong> {trace.fetchedGenre || '—'}
          {trace.provider
            ? ` · Provider: ${aiProviderLabel(trace.provider)}`
            : ''}
          {trace.model ? ` · Model: ${trace.model}` : ''}
        </Typography>
        <ChatTraceSection
          label="Request sent to iTunes"
          value={trace.request}
          classes={classes}
          copyLabel="Copy request"
        />
        <ChatTraceSection
          label="Prompt sent to AI"
          value={trace.prompt}
          classes={classes}
          copyLabel="Copy prompt"
        />
        <ChatTraceSection
          label={
            detail.source === 'itunes'
              ? 'Fetched iTunes response'
              : 'Raw AI response'
          }
          value={trace.response}
          classes={classes}
          copyLabel="Copy response"
        />
        {attempts.length > 1 ? (
          <Box mt={2}>
            <Typography variant="subtitle2">AI attempts</Typography>
            {attempts.map((attempt) => (
              <Box
                className={classes.genreTraceAttempt}
                key={`genre-trace-attempt-${attempt.number}`}
              >
                <Typography variant="body2">
                  <strong>Attempt {attempt.number}</strong>
                </Typography>
                <ChatTraceSection
                  label="Prompt"
                  value={attempt.prompt}
                  classes={classes}
                />
                <ChatTraceSection
                  label={attempt.error ? 'Error' : 'Response'}
                  value={attempt.error || attempt.response}
                  classes={classes}
                />
              </Box>
            ))}
          </Box>
        ) : null}
      </Box>
    )
  }

  const confidenceFieldLabel = (field) =>
    field === 'album' ? 'Album' : field === 'year' ? 'Year' : 'Genre'

  const CONFIDENCE_SOURCE_TEXT = {
    verified: 'Two independent sources agree on this value — verified.',
    spotify: 'Taken from Spotify (primary source for album and year).',
    musicbrainz: 'Taken from MusicBrainz (iTunes for genre).',
    conflict:
      'The stored value disagrees with the sources — treat with caution and prefer the source value below.',
    'ai-only':
      'From the AI only. No external source could confirm it, so the score is intentionally low.',
    none: 'No value was available for this field.',
  }

  const renderConfidenceBreakdown = (detail) => {
    const { field, value, breakdown } = detail
    const explanation = breakdown?.[field]
    if (!explanation) {
      return (
        <Box>
          <Typography variant="body2" paragraph>
            This {confidenceFieldLabel(field).toLowerCase()} confidence is{' '}
            <strong>{value}%</strong>.
          </Typography>
          <Typography variant="body2" color="textSecondary">
            The detailed breakdown isn’t stored for this row because it was
            fetched with an earlier version. Run{' '}
            <strong>Fetch AI Metadata</strong> again to see how it was resolved.
          </Typography>
        </Box>
      )
    }
    const source = explanation.source || 'none'
    return (
      <Box>
        <Typography variant="body2" paragraph>
          {CONFIDENCE_SOURCE_TEXT[source] || CONFIDENCE_SOURCE_TEXT.none}
        </Typography>
        <Table size="small">
          <TableBody>
            <TableRow>
              <TableCell>Spotify</TableCell>
              <TableCell align="right">
                {explanation.spotify || 'No match'}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell>
                {field === 'genre' ? 'iTunes' : 'MusicBrainz'}
              </TableCell>
              <TableCell align="right">
                {explanation.musicBrainz || 'No match'}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell>AI</TableCell>
              <TableCell align="right">{explanation.ai || '—'}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell>
                <strong>Confidence</strong>
              </TableCell>
              <TableCell align="right">
                <strong>{value}%</strong>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </Box>
    )
  }

  const isColumnVisible = (column) => visibleColumns[column] !== false
  const allConfidenceColumnsVisible =
    CONFIDENCE_COLUMN_IDS.every(isColumnVisible)
  const someConfidenceColumnsVisible =
    CONFIDENCE_COLUMN_IDS.some(isColumnVisible)

  const toggleColumn = (column) => {
    setVisibleColumns((current) => ({
      ...current,
      [column]: current[column] === false,
    }))
  }

  const toggleAllConfidenceColumns = () => {
    const visible = !someConfidenceColumnsVisible
    setVisibleColumns((current) => ({
      ...current,
      ...Object.fromEntries(
        CONFIDENCE_COLUMN_IDS.map((column) => [column, visible]),
      ),
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
    if (!normalChatProviderOverridden) {
      setNormalChatProvider(defaultProvider)
    }
  }, [defaultProvider, chatProviderOverridden, normalChatProviderOverridden])

  useEffect(() => {
    localStorage.setItem(
      AUTO_FETCH_ALL_LYRICS_STORAGE_KEY,
      isAutoFetchAllLyricsEnabled ? 'true' : 'false',
    )
  }, [isAutoFetchAllLyricsEnabled])

  useEffect(() => {
    localStorage.setItem(METADATA_AI_PROVIDER_STORAGE_KEY, metadataProvider)
  }, [metadataProvider])

  useEffect(() => {
    localStorage.setItem(
      METADATA_BATCH_SIZE_STORAGE_KEY,
      String(metadataBatchSize),
    )
  }, [metadataBatchSize])

  useEffect(() => {
    localStorage.setItem(
      AUTO_FETCH_ALL_METADATA_STORAGE_KEY,
      isAutoFetchAllMetadataEnabled ? 'true' : 'false',
    )
  }, [isAutoFetchAllMetadataEnabled])

  useEffect(() => {
    localStorage.setItem(
      AI_TOOL_COLUMNS_STORAGE_KEY,
      JSON.stringify(visibleColumns),
    )
  }, [visibleColumns])

  useEffect(() => {
    let active = true

    const refreshStatuses = async () => {
      try {
        const { json } = await httpClient('/api/ai/status')
        if (!active) return
        const byId = new Map(
          (json?.services || []).map((service) => [service.id, service]),
        )
        setWhisperModel(json?.whisperModel || DEFAULT_WHISPER_MODEL)
        setModelStatuses(
          AI_SERVICES.map((service) => {
            const reported = byId.get(service.id)
            const state = ['online', 'busy', 'offline'].includes(
              reported?.state,
            )
              ? reported.state
              : reported?.online === true
                ? 'online'
                : 'offline'
            return {
              ...service,
              online: reported?.online === true || state === 'busy',
              state,
            }
          }),
        )
      } catch {
        if (active) {
          setModelStatuses(
            AI_SERVICES.map((service) => ({
              ...service,
              online: false,
              state: 'offline',
            })),
          )
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
    loadRAGStatus()
  }, [loadRAGStatus])

  useEffect(() => {
    addedSongsRef.current = addedSongs
  }, [addedSongs])

  useEffect(() => {
    lyricsLoadingIdRef.current = lyricsLoadingId
  }, [lyricsLoadingId])

  useEffect(() => {
    explicitClassifyingSongIdsRef.current = new Set(explicitClassifyingSongIds)
  }, [explicitClassifyingSongIds])

  useEffect(() => {
    if (
      !lyricsProgressStatus ||
      !['running', 'stopping'].includes(lyricsProgressStatus)
    )
      return

    setProgressClock(Date.now())
    const interval = window.setInterval(
      () => setProgressClock(Date.now()),
      1000,
    )
    return () => window.clearInterval(interval)
  }, [lyricsProgressStartedAt, lyricsProgressStatus])

  useEffect(() => {
    if (!isChatOpen || !chatMessagesRef.current) return

    const messagesEl = chatMessagesRef.current
    messagesEl.scrollTop = messagesEl.scrollHeight
  }, [messages, isSending, isChatOpen, isChatExpanded])

  useEffect(() => {
    if (!isNormalChatOpen || !normalChatMessagesRef.current) return

    const messagesEl = normalChatMessagesRef.current
    messagesEl.scrollTop = messagesEl.scrollHeight
  }, [normalMessages, isNormalSending, isNormalChatOpen, isNormalChatExpanded])

  useEffect(
    () => () => {
      chatAbortControllerRef.current?.abort()
      normalChatAbortControllerRef.current?.abort()
      lyricsAbortControllerRef.current?.abort()
      metadataAbortControllerRef.current?.abort()
      Object.values(progressTimeoutRefs.current).forEach((timeout) => {
        if (timeout !== null) window.clearTimeout(timeout)
      })
    },
    [],
  )

  useEffect(() => {
    const visibleIds = new Set(addedSongs.map((song) => song.id))
    setSelectedAddedSongIds((prev) => prev.filter((id) => visibleIds.has(id)))
  }, [addedSongs])

  useEffect(() => {
    if (!addedSongIds) return undefined

    let active = true
    const reconcileAddedSongs = async () => {
      try {
        const query = addedSongs
          .map((song) => `id=${encodeURIComponent(song.id)}`)
          .join('&')
        const { json } = await httpClient(`/api/song?${query}`)
        const currentSongs = new Map(
          (json || []).map((song) => [song.id, song]),
        )
        const nextSongs = addedSongs
          .filter((song) => currentSongs.has(song.id))
          .map((song) => {
            const current = currentSongs.get(song.id)
            return {
              ...song,
              ...current,
              aiGenre: song.aiGenre || '',
              spotifyGenre: song.spotifyGenre || '',
              musicBrainzGenre: song.musicBrainzGenre || '',
            }
          })

        if (
          active &&
          JSON.stringify(nextSongs) !== JSON.stringify(addedSongs)
        ) {
          setAddedSongs(nextSongs)
          localStorage.setItem(
            ADDED_SONGS_STORAGE_KEY,
            JSON.stringify(nextSongs),
          )
        }
      } catch {
        // Keep the local queue if the API is temporarily unavailable.
      }
    }

    reconcileAddedSongs()
    return () => {
      active = false
    }
  }, [addedSongIds, addedSongs])

  const changeDeveloperTrace = (enabled) => {
    setDeveloperTraceEnabled(enabled)
    try {
      localStorage.setItem(AI_CHAT_DEVELOPER_TRACE_STORAGE_KEY, String(enabled))
    } catch {
      // The switch still works for this page session if storage is unavailable.
    }
  }

  const sendMessage = async () => {
    const trimmed = prompt.trim()
    if (!trimmed || isSending) return

    const provider = normalizeAIProvider(chatProvider)
    const abortController = new AbortController()
    chatAbortControllerRef.current = abortController
    setChatError('')
    setChatErrorTrace(null)
    // Send the recent conversation so the assistant can resolve follow-ups.
    const history = messages.slice(-8).map((message) => ({
      role: message.role,
      content: message.text || '',
    }))
    setMessages((prev) => [...prev, { role: 'user', text: trimmed, provider }])
    setPrompt('')
    setIsSending(true)

    try {
      const { json: payload } = await httpClient('/api/ai/chat', {
        method: 'POST',
        signal: abortController.signal,
        body: JSON.stringify({
          message: trimmed,
          provider,
          useRag: true,
          history,
          ...(developerTraceEnabled ? { developerTrace: true } : {}),
        }),
      })

      setMessages((prev) => [
        ...prev,
        {
          role: 'assistant',
          text: payload.response || '',
          provider: payload.provider || provider,
          model: payload.model || '',
          sources: Array.isArray(payload.sources) ? payload.sources : [],
          ragError: payload.ragError || '',
          direct: Boolean(payload.direct),
          trace: payload.trace || null,
        },
      ])
    } catch (err) {
      if (abortController.signal.aborted || err?.name === 'AbortError') {
        setChatError('')
      } else {
        setChatError(err?.message || 'Could not get response from AI')
        setChatErrorTrace(err?.body?.trace || err?.trace || null)
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

  const sendNormalMessage = async () => {
    const trimmed = normalPrompt.trim()
    if (!trimmed || isNormalSending) return

    const provider = normalizeAIProvider(normalChatProvider)
    const abortController = new AbortController()
    normalChatAbortControllerRef.current = abortController
    setNormalChatError('')
    setNormalChatErrorTrace(null)
    setNormalMessages((prev) => [
      ...prev,
      { role: 'user', text: trimmed, provider },
    ])
    setNormalPrompt('')
    setIsNormalSending(true)

    try {
      const { json: payload } = await httpClient('/api/ai/chat', {
        method: 'POST',
        signal: abortController.signal,
        body: JSON.stringify({
          message: trimmed,
          provider,
          useRag: false,
          ...(developerTraceEnabled ? { developerTrace: true } : {}),
        }),
      })

      setNormalMessages((prev) => [
        ...prev,
        {
          role: 'assistant',
          text: payload.response || '',
          provider: payload.provider || provider,
          model: payload.model || '',
          trace: payload.trace || null,
        },
      ])
    } catch (err) {
      if (abortController.signal.aborted || err?.name === 'AbortError') {
        setNormalChatError('')
      } else {
        setNormalChatError(err?.message || 'Could not get response from AI')
        setNormalChatErrorTrace(err?.body?.trace || err?.trace || null)
      }
    } finally {
      if (normalChatAbortControllerRef.current === abortController) {
        normalChatAbortControllerRef.current = null
      }
      setIsNormalSending(false)
    }
  }

  const stopNormalMessage = () => {
    normalChatAbortControllerRef.current?.abort()
  }

  const startFloatingChatResize = (
    corner,
    event,
    expanded,
    frame,
    setExpanded,
    setFrame,
  ) => {
    event.preventDefault()
    event.stopPropagation()
    setExpanded(false)

    const startX = event.clientX
    const startY = event.clientY
    const startFrame = expanded ? expandedChatFrame() : frame

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

      setFrame({ left, top, width, height })
    }

    const onMouseUp = () => {
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }

    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
  }

  const startChatResize = (corner, event) =>
    startFloatingChatResize(
      corner,
      event,
      isChatExpanded,
      chatFrame,
      setIsChatExpanded,
      setChatFrame,
    )

  const startNormalChatResize = (corner, event) =>
    startFloatingChatResize(
      corner,
      event,
      isNormalChatExpanded,
      normalChatFrame,
      setIsNormalChatExpanded,
      setNormalChatFrame,
    )

  const openAddSongsDialog = async () => {
    setSongDialogOpen(true)
    setSelectedSongIds((current) =>
      current.filter((id) => !addedSongIdSet.has(id)),
    )
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
    if (addedSongIdSet.has(songId)) return
    setSelectedSongIds((prev) =>
      prev.includes(songId)
        ? prev.filter((id) => id !== songId)
        : [...prev, songId],
    )
  }

  const toggleAllAvailableSongs = () => {
    setSelectedSongIds(
      selectableSongIds.length > 0 &&
        selectedSelectableSongCount === selectableSongIds.length
        ? []
        : selectableSongIds,
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
    setSelectedSongIds([])
    setSongDialogOpen(false)
  }

  const toggleAddedSong = (songId) => {
    setSelectedAddedSongIds((prev) =>
      prev.includes(songId)
        ? prev.filter((id) => id !== songId)
        : [...prev, songId],
    )
  }

  const toggleAllAddedSongs = () => {
    setSelectedAddedSongIds(
      selectedAddedIds.length === addedSongs.length
        ? []
        : addedSongs.map((song) => song.id),
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
    if (
      !song?.id ||
      !String(err?.message || '')
        .toLowerCase()
        .includes('not found')
    )
      return
    removeSong(song.id)
  }

  const startProgress = (type, songs) => {
    if (type === 'lyrics') setIsLyricsProgressHidden(false)
    if (type === 'metadata') setIsMetadataProgressHidden(false)
    const startedAt = Date.now()
    setProgressClock(startedAt)
    setJobProgresses((prev) => ({
      ...prev,
      [type]: {
        type,
        currentTitle: songs[0]?.title || '',
        done: 0,
        total: songs.length,
        status: 'running',
        startedAt,
        finishedAt: null,
      },
    }))
  }

  const updateProgress = (type, song, done, total) => {
    setJobProgresses((prev) => ({
      ...prev,
      [type]: {
        type,
        currentTitle: song?.title || '',
        done,
        total,
        status: 'running',
        startedAt: prev[type]?.startedAt || Date.now(),
        finishedAt: null,
      },
    }))
  }

  const finishProgress = (type, total) => {
    const finishedAt = Date.now()
    setProgressClock(finishedAt)
    setJobProgresses((prev) => ({
      ...prev,
      [type]: {
        type,
        currentTitle: 'Complete',
        done: total,
        total,
        status: 'complete',
        startedAt: prev[type]?.startedAt || finishedAt,
        finishedAt,
      },
    }))
    if (progressTimeoutRefs.current[type] !== null) {
      window.clearTimeout(progressTimeoutRefs.current[type])
    }
    progressTimeoutRefs.current[type] = window.setTimeout(
      () => {
        progressTimeoutRefs.current[type] = null
        setJobProgresses((prev) => ({
          ...prev,
          [type]: prev[type]?.status === 'complete' ? null : prev[type],
        }))
      },
      type === 'lyrics' ? 5000 : 1500,
    )
  }

  const stopProgress = (type) => {
    const finishedAt = Date.now()
    setProgressClock(finishedAt)
    setJobProgresses((prev) => ({
      ...prev,
      [type]: prev[type]
        ? {
            ...prev[type],
            currentTitle: 'Stopped',
            status: 'stopped',
            finishedAt,
          }
        : null,
    }))
    if (progressTimeoutRefs.current[type] !== null) {
      window.clearTimeout(progressTimeoutRefs.current[type])
    }
    progressTimeoutRefs.current[type] = window.setTimeout(() => {
      progressTimeoutRefs.current[type] = null
      setJobProgresses((prev) => ({
        ...prev,
        [type]: prev[type]?.status === 'stopped' ? null : prev[type],
      }))
    }, 1500)
  }

  const updateSongsFromLyricsJobResults = (results = []) => {
    const completed = results.filter(
      (result) => result?.songId && result.success,
    )
    if (!completed.length) return
    const completedIDs = new Set(completed.map((result) => result.songId))
    setAddedSongs((prev) => {
      const nextSongs = prev.map((song) =>
        completedIDs.has(song.id) ? { ...song, lyrics: 'saved' } : song,
      )
      localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
      return nextSongs
    })
  }

  const applyLyricsJobStatus = (status) => {
    if (!status || typeof status !== 'object') return
    if (typeof status.status !== 'string') return
    updateSongsFromLyricsJobResults(status.results || [])
    if (status.status === 'idle') {
      if (lyricsLoadingIdRef.current === SERVER_LYRICS_LOADING_ID) {
        lyricsLoadingIdRef.current = ''
        serverLyricsJobActiveRef.current = false
        setLyricsLoadingId('')
        setLyricsFetchingSongIds([])
        setJobProgresses((prev) => ({ ...prev, lyrics: null }))
      }
      return
    }

    const startedAt = status.startedAt
      ? new Date(status.startedAt).getTime()
      : Date.now()
    const finishedAt = status.finishedAt
      ? new Date(status.finishedAt).getTime()
      : null
    setProgressClock(Date.now())
    setJobProgresses((prev) => ({
      ...prev,
      lyrics: {
        type: 'lyrics',
        currentSongId: status.currentSongId || '',
        currentTitle:
          status.currentTitle ||
          (status.status === 'complete'
            ? 'Complete'
            : status.status === 'stopped'
              ? 'Stopped'
              : 'Fetching lyrics'),
        done: Number(status.done) || 0,
        total: Number(status.total) || 0,
        status:
          status.status === 'stopping'
            ? 'stopping'
            : status.running
              ? 'running'
              : status.status === 'stopped'
                ? 'stopped'
                : status.status === 'failed'
                  ? 'failed'
                  : 'complete',
        startedAt,
        finishedAt,
      },
    }))
    lyricsLoadingIdRef.current = status.running ? SERVER_LYRICS_LOADING_ID : ''
    serverLyricsJobActiveRef.current = Boolean(status.running)
    setLyricsLoadingId(status.running ? SERVER_LYRICS_LOADING_ID : '')
    setLyricsFetchingSongIds(
      status.running && Array.isArray(status.songIds) ? status.songIds : [],
    )
    if (status.error && status.status !== 'complete') {
      setToolError(status.error)
    }
  }
  applyLyricsJobStatusRef.current = applyLyricsJobStatus

  const stopFetchJob = async (type) => {
    if (type === 'metadata') {
      metadataAbortControllerRef.current?.abort()
      setJobProgresses((prev) => ({
        ...prev,
        metadata: prev.metadata
          ? {
              ...prev.metadata,
              currentTitle: 'Stopping…',
              status: 'stopping',
            }
          : prev.metadata,
      }))
      return
    }

    if (
      serverLyricsJobActiveRef.current ||
      lyricsLoadingIdRef.current === SERVER_LYRICS_LOADING_ID ||
      (!lyricsAbortControllerRef.current && lyricsJobProgress)
    ) {
      setJobProgresses((prev) => ({
        ...prev,
        lyrics: prev.lyrics
          ? {
              ...prev.lyrics,
              currentTitle: 'Stopping…',
              status: 'stopping',
            }
          : prev.lyrics,
      }))
      try {
        const { json } = await httpClient('/api/ai/lyrics/fetch-job', {
          method: 'DELETE',
        })
        applyLyricsJobStatus(json)
      } catch (err) {
        setToolError(err?.message || 'Could not stop lyrics job')
      }
      return
    }

    if (!lyricsAbortControllerRef.current) return

    lyricsAbortControllerRef.current.abort()
    setJobProgresses((prev) => ({
      ...prev,
      lyrics: prev.lyrics
        ? { ...prev.lyrics, currentTitle: 'Stopping…', status: 'stopping' }
        : prev.lyrics,
    }))
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
    if (
      !song?.id ||
      lyricsLoadingId ||
      lyricsAbortControllerRef.current ||
      explicitClassifyingSongIdsRef.current.has(song.id)
    )
      return

    const abortController = new AbortController()
    lyricsAbortControllerRef.current = abortController
    setToolError('')
    lyricsLoadingIdRef.current = song.id
    setLyricsLoadingId(song.id)
    setLyricsFetchingSongIds([song.id])
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
        setJobProgresses((prev) => ({ ...prev, lyrics: null }))
      }
    } finally {
      if (lyricsAbortControllerRef.current === abortController) {
        lyricsAbortControllerRef.current = null
      }
      if (lyricsLoadingIdRef.current === song.id) {
        lyricsLoadingIdRef.current = ''
      }
      setLyricsLoadingId('')
      setLyricsFetchingSongIds([])
    }
  }

  const fetchSelectedLyrics = async () => {
    if (!selectedAddedIds.length) return
    await fetchLyricsForSongs(selectedAddedSongs, {
      emptyMessage: 'All selected songs already have lyrics.',
    })
  }

  const fetchLyricsForSongs = async (songs, { emptyMessage = '' } = {}) => {
    if (lyricsLoadingIdRef.current || lyricsAbortControllerRef.current)
      return false

    const songsToFetch = songs.filter(
      (song) =>
        song?.id &&
        !hasSavedLyrics(song) &&
        !explicitClassifyingSongIdsRef.current.has(song.id),
    )
    if (!songsToFetch.length) {
      if (emptyMessage) setToolError(emptyMessage)
      return false
    }

    setToolError('')
    lyricsLoadingIdRef.current = SERVER_LYRICS_LOADING_ID
    serverLyricsJobActiveRef.current = true
    setLyricsLoadingId(SERVER_LYRICS_LOADING_ID)
    setLyricsFetchingSongIds(songsToFetch.map((song) => song.id))
    startProgress('lyrics', songsToFetch)
    try {
      const { json } = await httpClient('/api/ai/lyrics/fetch-job', {
        method: 'POST',
        body: JSON.stringify({ songIds: songsToFetch.map((song) => song.id) }),
      })
      applyLyricsJobStatus(json)
      return true
    } catch (err) {
      if (lyricsLoadingIdRef.current === SERVER_LYRICS_LOADING_ID) {
        lyricsLoadingIdRef.current = ''
      }
      serverLyricsJobActiveRef.current = false
      setLyricsLoadingId('')
      setLyricsFetchingSongIds([])
      setToolError(err?.message || 'Could not fetch lyrics')
      setJobProgresses((prev) => ({ ...prev, lyrics: null }))
      return false
    }
  }
  fetchLyricsForSongsRef.current = fetchLyricsForSongs

  const toggleAutoFetchAllLyrics = () => {
    const disabling = isAutoFetchAllLyricsEnabled
    setIsLyricsProgressHidden(disabling)
    setIsAutoFetchAllLyricsEnabled((enabled) => !enabled)
    if (
      disabling &&
      (lyricsLoadingIdRef.current ||
        lyricsAbortControllerRef.current ||
        serverLyricsJobActiveRef.current)
    ) {
      void stopFetchJob('lyrics')
    }
  }

  useEffect(() => {
    if (!isAutoFetchAllLyricsEnabled) return undefined

    const runLyricsCheck = () => {
      void fetchLyricsForSongsRef.current?.(addedSongsRef.current, {
        emptyMessage: '',
      })
    }

    runLyricsCheck()
    const interval = window.setInterval(
      runLyricsCheck,
      AUTO_FETCH_ALL_LYRICS_INTERVAL_MS,
    )
    return () => window.clearInterval(interval)
  }, [isAutoFetchAllLyricsEnabled])

  useEffect(() => {
    let active = true
    const refreshLyricsJobStatus = async () => {
      try {
        const { json } = await httpClient('/api/ai/lyrics/fetch-job/status')
        if (active) applyLyricsJobStatusRef.current?.(json)
      } catch {
        // Status polling should never interrupt the rest of the AI page.
      }
    }

    refreshLyricsJobStatus()
    const interval = window.setInterval(refreshLyricsJobStatus, 2000)
    return () => {
      active = false
      window.clearInterval(interval)
    }
  }, [])

  const deleteLyricsForSongs = async (songs) => {
    const songsToDelete = songs.filter(
      (song) =>
        hasSavedLyrics(song) &&
        !lyricsFetchingSongIdSet.has(song.id) &&
        !explicitClassifyingSongIdSet.has(song.id),
    )
    if (!songsToDelete.length || isDeletingLyrics) return

    setToolError('')
    setIsDeletingLyrics(true)
    try {
      for (const song of songsToDelete) {
        await httpClient(`/api/ai/songs/${song.id}/lyrics`, {
          method: 'DELETE',
        })
      }
      const deletedIDs = new Set(songsToDelete.map((song) => song.id))
      setAddedSongs((prev) => {
        const nextSongs = prev.map((song) =>
          deletedIDs.has(song.id)
            ? { ...song, lyrics: '', lyricsText: '' }
            : song,
        )
        localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
        return nextSongs
      })
      if (explicitReasonSong && deletedIDs.has(explicitReasonSong.id)) {
        setExplicitReasonSong((song) =>
          song ? { ...song, lyrics: '', lyricsText: '' } : song,
        )
      }
      if (lyricsDialogSong && deletedIDs.has(lyricsDialogSong.id)) {
        setLyricsDialogOpen(false)
        setLyricsDialogSong(null)
        setLyricsText('')
      }
    } catch (err) {
      setToolError(err?.message || 'Could not delete lyrics')
    } finally {
      setIsDeletingLyrics(false)
    }
  }

  const showLyrics = async (song) => {
    if (!song?.id) return

    setToolError('')
    try {
      const { json: payload } = await httpClient(
        `/api/ai/songs/${song.id}/lyrics`,
      )
      setLyricsDialogTitle(
        song.title || translate('menu.aiTool.lyrics', { _: 'Lyrics' }),
      )
      setLyricsDialogSong(song)
      setLyricsText(payload.text || '')
      setLyricsDialogOpen(true)
    } catch (err) {
      removeStaleSongOnNotFound(err, song)
      setToolError(err?.message || 'Could not load lyrics')
    }
  }

  const openExplicitDialog = (songs) => {
    if (!songs.length) return

    setExplicitDialogSongs(songs)
  }

  const closeExplicitDialog = () => {
    setExplicitDialogSongs([])
  }

  const saveExplicitWordRules = () => {
    const included = parseExplicitWordList(explicitIncludedWords)
    const excluded = parseExplicitWordList(explicitExcludedWords)
    setExplicitIncludedWords(included.join(', '))
    setExplicitExcludedWords(excluded.join(', '))
    localStorage.setItem(
      EXPLICIT_WORD_RULES_STORAGE_KEY,
      JSON.stringify({
        version: EXPLICIT_WORD_RULES_VERSION,
        included,
        excluded,
      }),
    )
    setExplicitRulesOpen(false)
  }

  const resetExplicitWordRules = () => {
    setExplicitIncludedWords(DEFAULT_EXPLICIT_INCLUDED_WORDS.join(', '))
    setExplicitExcludedWords(DEFAULT_EXPLICIT_EXCLUDED_WORDS.join(', '))
  }

  const classifyExplicit = async (songs) => {
    const songIds = songs
      .filter(
        (song) =>
          song?.id &&
          hasSavedLyrics(song) &&
          !lyricsFetchingSongIdSet.has(song.id),
      )
      .map((song) => song.id)
    if (!songIds.length || isClassifyingExplicit) return

    setToolError('')
    setIsClassifyingExplicit(true)
    explicitClassifyingSongIdsRef.current = new Set(songIds)
    setExplicitClassifyingSongIds(songIds)
    try {
      const { json: payload } = await httpClient('/api/ai/classify-explicit', {
        method: 'POST',
        body: JSON.stringify({
          songIds,
          provider: EXPLICIT_AI_PROVIDER,
          includedWords: parseExplicitWordList(explicitIncludedWords),
          excludedWords: parseExplicitWordList(explicitExcludedWords),
        }),
      })
      const classifications = new Map(
        (payload.songs || []).map((song) => [song.id, song]),
      )
      setAddedSongs((prev) => {
        const nextSongs = prev.map((song) => {
          const classification = classifications.get(song.id)
          if (!classification) return song
          return {
            ...song,
            explicitStatus: classification.explicitStatus || '',
            explicitReason: classification.reason || '',
            explicitConfidence:
              normalizeMetadataConfidence(classification.confidence) ?? 0,
            explicitEvidence: Array.isArray(classification.evidence)
              ? classification.evidence
              : [],
            explicitProvider: classification.provider || '',
            explicitBasis: classification.basis || '',
            aiFields: {
              ...(song.aiFields || {}),
              explicitStatus: Boolean(classification.explicitStatus),
            },
          }
        })
        localStorage.setItem(ADDED_SONGS_STORAGE_KEY, JSON.stringify(nextSongs))
        return nextSongs
      })
    } catch (err) {
      setToolError(err?.message || 'Could not classify explicit content')
    } finally {
      setIsClassifyingExplicit(false)
      explicitClassifyingSongIdsRef.current = new Set()
      setExplicitClassifyingSongIds([])
    }
  }

  const fetchAIMetadataForSongs = async (songs, provider) => {
    if (
      !songs.length ||
      isFetchingMetadata ||
      isClearingMetadata ||
      metadataAbortControllerRef.current
    )
      return

    const abortController = new AbortController()
    metadataAbortControllerRef.current = abortController
    setToolError('')
    setIsFetchingMetadata(true)
    startProgress('metadata', songs)
    try {
      // The batch size controls how many songs the server classifies in one
      // AI prompt: the instruction block is paid for once per batch, saving
      // input tokens at some accuracy cost. 1 keeps the focused single-song
      // prompt.
      const batchSize = normalizeMetadataBatchSize(metadataBatchSize)
      let done = 0
      for (let start = 0; start < songs.length; start += batchSize) {
        const batch = songs.slice(start, start + batchSize)
        updateProgress('metadata', batch[0], done, songs.length)
        const { json: payload } = await httpClient('/api/ai/fetch-metadata', {
          method: 'POST',
          signal: abortController.signal,
          body: JSON.stringify({
            songIds: batch.map((song) => song.id),
            provider: normalizeAIProvider(provider),
          }),
        })
        const metadata = new Map(
          (payload.songs || []).map((item) => [item.id, item]),
        )
        setAddedSongs((prev) => {
          const nextSongs = prev.map((item) => {
            const update = metadata.get(item.id)
            if (!update) return item
            const metadataConfidence = {
              ...(item.metadataConfidence || {}),
              genre: normalizeMetadataConfidence(update.genreConfidence) ?? 0,
            }
            const genreDeveloperTrace =
              update.genreDeveloperTrace || item.genreDeveloperTrace
                ? {
                    ...(item.genreDeveloperTrace || {}),
                    ...(update.genreDeveloperTrace || {}),
                  }
                : undefined
            return {
              ...item,
              aiGenre: update.aiGenre || item.aiGenre || '',
              aiSubgenre: update.aiSubgenre || item.aiSubgenre || '',
              spotifyGenre: update.spotifyGenre || item.spotifyGenre || '',
              musicBrainzGenre:
                update.musicBrainzGenre || item.musicBrainzGenre || '',
              aiTokens: update.aiTokens || item.aiTokens || null,
              metadataConfidence,
              metadataConfidenceBreakdown:
                update.confidenceBreakdown || item.metadataConfidenceBreakdown,
              genreDeveloperTrace,
              aiFields: {
                ...(item.aiFields || {}),
                aiGenre:
                  Boolean(update.aiGenre) || Boolean(item.aiFields?.aiGenre),
                aiSubgenre:
                  Boolean(update.aiSubgenre) ||
                  Boolean(item.aiFields?.aiSubgenre),
                spotifyGenre:
                  Boolean(update.spotifyGenre) ||
                  Boolean(item.aiFields?.spotifyGenre),
                musicBrainzGenre:
                  Boolean(update.musicBrainzGenre) ||
                  Boolean(item.aiFields?.musicBrainzGenre),
              },
            }
          })
          localStorage.setItem(
            ADDED_SONGS_STORAGE_KEY,
            JSON.stringify(nextSongs),
          )
          return nextSongs
        })
        done += batch.length
        updateProgress('metadata', batch[batch.length - 1], done, songs.length)
      }
      finishProgress('metadata', songs.length)
    } catch (err) {
      if (abortController.signal.aborted || err?.name === 'AbortError') {
        stopProgress('metadata')
      } else {
        setToolError(err?.message || 'Could not fetch AI metadata')
        setJobProgresses((prev) => ({ ...prev, metadata: null }))
      }
    } finally {
      if (metadataAbortControllerRef.current === abortController) {
        metadataAbortControllerRef.current = null
      }
      setIsFetchingMetadata(false)
    }
  }
  fetchAIMetadataForSongsRef.current = fetchAIMetadataForSongs

  const toggleAutoFetchAllMetadata = () => {
    const disabling = isAutoFetchAllMetadataEnabled
    setIsMetadataProgressHidden(disabling)
    setIsAutoFetchAllMetadataEnabled((enabled) => !enabled)
    if (disabling && metadataAbortControllerRef.current) {
      void stopFetchJob('metadata')
    }
  }

  useEffect(() => {
    if (!isAutoFetchAllMetadataEnabled) return undefined

    const runMetadataCheck = () => {
      void fetchAIMetadataForSongsRef.current?.(
        addedSongsRef.current,
        metadataProvider,
      )
    }

    runMetadataCheck()
    const interval = window.setInterval(
      runMetadataCheck,
      AUTO_FETCH_ALL_METADATA_INTERVAL_MS,
    )
    return () => window.clearInterval(interval)
  }, [isAutoFetchAllMetadataEnabled, metadataProvider])

  const clearFetchedMetadata = async () => {
    if (
      !selectedAddedSongs.length ||
      isClearingMetadata ||
      isFetchingMetadata ||
      metadataAbortControllerRef.current
    )
      return

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
                aiSubgenre: '',
                spotifyGenre: '',
                musicBrainzGenre: '',
                aiTokens: null,
                metadataConfidence: {},
                metadataConfidenceBreakdown: undefined,
                genreDeveloperTrace: undefined,
                aiFields: {
                  ...(song.aiFields || {}),
                  album: false,
                  year: false,
                  aiGenre: false,
                  aiSubgenre: false,
                  spotifyGenre: false,
                  musicBrainzGenre: false,
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

  const runExplicitAction = async () => {
    const songs = explicitDialogSongs
    closeExplicitDialog()
    await classifyExplicit(songs)
  }

  const runRowAction = async (action) => {
    const song = rowActionSong
    closeRowActions()
    if (!song) return

    if (action === 'fetchLyrics') {
      await fetchLyrics(song)
    } else if (action === 'showLyrics') {
      await showLyrics(song)
    } else if (action === 'deleteLyrics') {
      await deleteLyricsForSongs([song])
    } else if (action === 'fetchMetadata') {
      await fetchAIMetadataForSongs([song], metadataProvider)
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
              onChange={(event) =>
                setDefaultProvider(normalizeAIProvider(event.target.value))
              }
            >
              {AI_PROVIDERS.map((provider) => (
                <MenuItem key={provider.id} value={provider.id}>
                  {provider.label}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              select
              className={classes.defaultProviderSelect}
              label="Default Whisper Model"
              variant="outlined"
              size="small"
              value={whisperModel}
              disabled={isUpdatingWhisperModel}
              onChange={(event) => updateWhisperModel(event.target.value)}
            >
              {WHISPER_MODELS.map((model) => (
                <MenuItem key={model.id} value={model.id}>
                  {model.label}
                </MenuItem>
              ))}
            </TextField>
            <Box className={classes.serviceStatusPanel}>
              <Box className={classes.serviceStatusHeader}>
                <Typography
                  component="span"
                  className={classes.serviceStatusHeaderTitle}
                >
                  AI model and RAG status
                </Typography>
                <Box component="span" className={classes.serviceStatusRibbon}>
                  <span
                    className={classes.serviceStatusRibbonItem}
                    aria-label={`RAG status: ${
                      ragStatus
                        ? ragStatus.enabled
                          ? 'Enabled'
                          : 'Disabled'
                        : ragStatusError
                          ? 'Unavailable'
                          : 'Loading…'
                    }`}
                  >
                    <span className={classes.serviceStatusRibbonLabel}>
                      RAG
                    </span>
                    <strong
                      className={
                        ragStatus?.enabled
                          ? classes.ragStatusEnabled
                          : classes.ragStatusDisabled
                      }
                    >
                      {ragStatus
                        ? ragStatus.enabled
                          ? 'Enabled'
                          : 'Disabled'
                        : ragStatusError
                          ? 'Unavailable'
                          : 'Loading…'}
                    </strong>
                  </span>
                  <span
                    className={classes.serviceStatusRibbonItem}
                    aria-label={`Vector DB status: ${
                      ragStatus
                        ? ragStatus.vectorDbOnline
                          ? 'Online'
                          : 'Offline'
                        : ragStatusError
                          ? 'Unavailable'
                          : 'Checking…'
                    }`}
                  >
                    <span className={classes.serviceStatusRibbonLabel}>
                      Vector DB
                    </span>
                    <strong
                      className={
                        ragStatus?.vectorDbOnline
                          ? classes.ragStatusEnabled
                          : classes.ragStatusDisabled
                      }
                    >
                      {ragStatus
                        ? ragStatus.vectorDbOnline
                          ? 'Online'
                          : 'Offline'
                        : ragStatusError
                          ? 'Unavailable'
                          : 'Checking…'}
                    </strong>
                  </span>
                </Box>
                <Typography
                  component="span"
                  className={classes.serviceStatusSummary}
                >
                  {areStatusesChecking
                    ? 'Checking…'
                    : `${onlineServiceCount}/${AI_SERVICES.length} AI models online`}
                </Typography>
              </Box>
              <Box className={classes.serviceStatusContent}>
                <Box className={classes.serviceStatusList}>
                  {modelStatuses.map((service) => {
                    const serviceState =
                      service.id === 'whisper' && isLyricsJobRunning
                        ? 'busy'
                        : service.state
                    const statusLabel =
                      serviceState === 'busy'
                        ? 'Busy'
                        : service.online === null
                          ? 'Checking…'
                          : service.online
                            ? 'Online'
                            : 'Offline'
                    return (
                      <Box
                        className={classes.serviceStatusItem}
                        key={service.id}
                      >
                        <Typography
                          className={classes.serviceStatusName}
                          variant="body2"
                        >
                          {service.label}
                        </Typography>
                        <Typography
                          component="span"
                          className={classes.serviceStatusValue}
                          style={{
                            color:
                              serviceState === 'busy'
                                ? '#c084fc'
                                : service.online === true
                                  ? '#3ddc84'
                                  : service.online === false
                                    ? '#ff8fc6'
                                    : '#c9d1dc',
                          }}
                        >
                          <span
                            className={`${classes.statusDot} ${
                              serviceState === 'busy'
                                ? classes.statusDotBusy
                                : service.online === true
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
                <button
                  type="button"
                  className={classes.ragControlsHeader}
                  onClick={() => setIsRAGControlsOpen((open) => !open)}
                  aria-expanded={isRAGControlsOpen}
                >
                  <Typography component="span">RAG controls</Typography>
                  <Typography
                    component="span"
                    className={classes.ragControlsSummary}
                  >
                    {isRAGControlsOpen ? '−' : '+'}
                  </Typography>
                </button>
                <Collapse in={isRAGControlsOpen}>
                  <Box
                    className={classes.ragStatusCard}
                    role="region"
                    aria-label="RAG controls"
                  >
                    <Button
                      className={classes.ragToggleButton}
                      size="small"
                      variant="outlined"
                      color="primary"
                      onClick={toggleRAG}
                      disabled={!ragStatus || isTogglingRAG}
                    >
                      {isTogglingRAG
                        ? 'Updating…'
                        : ragStatus?.enabled
                          ? 'Disable RAG'
                          : 'Enable RAG'}
                    </Button>
                    {ragStatus ? (
                      <Box className={classes.ragStatusDetails}>
                        <span>Vector URL: {ragStatus.vectorUrl}</span>
                        <span>Collection: {ragStatus.collection}</span>
                        <span>
                          Collection status:{' '}
                          <strong
                            className={
                              ragStatus.collectionExists
                                ? classes.ragStatusEnabled
                                : classes.ragStatusDisabled
                            }
                          >
                            {ragStatus.collectionExists ? 'Exists' : 'Missing'}
                          </strong>
                        </span>
                        <span>Indexed: {ragStatus.indexedCount}</span>
                        <span>Top K: {ragStatus.topK}</span>
                        <span>
                          Embeddings:{' '}
                          {ragStatus.offlineMode
                            ? 'Offline protected'
                            : ragStatus.embeddingLocal
                              ? 'Local'
                              : 'Cloud'}
                          {' · '}
                          {ragStatus.embeddingBackend}
                          {ragStatus.embeddingModel
                            ? ` (${ragStatus.embeddingModel})`
                            : ''}
                        </span>
                      </Box>
                    ) : null}
                    <Box className={classes.ragIndexControls}>
                      <TextField
                        id="rag-index-limit"
                        className={classes.ragIndexLimitInput}
                        variant="outlined"
                        size="small"
                        type="number"
                        label="Songs to index"
                        value={ragIndexLimit}
                        onChange={(event) =>
                          setRAGIndexLimit(event.target.value)
                        }
                        inputProps={{
                          min: 1,
                          max: MAX_RAG_INDEX_LIMIT,
                          step: 1,
                          'aria-label': 'Songs to index',
                        }}
                        error={ragIndexLimit !== '' && !isRAGIndexLimitValid}
                      />
                      <FormControlLabel
                        control={
                          <Checkbox
                            checked={ragIncludePlaylists}
                            onChange={(event) =>
                              setRAGIncludePlaylists(event.target.checked)
                            }
                            color="primary"
                          />
                        }
                        label="Include playlists"
                      />
                      <Button
                        size="small"
                        variant="outlined"
                        color="primary"
                        onClick={() => indexRAGSongs(false)}
                        disabled={
                          !ragStatus?.enabled ||
                          !isRAGIndexLimitValid ||
                          isIndexingRAG ||
                          isRefreshingRAG ||
                          isAddingQdrantLyrics ||
                          isClearingRAG
                        }
                      >
                        {isIndexingRAG
                          ? 'Indexing…'
                          : `Index ${isRAGIndexLimitValid ? parsedRAGIndexLimit : ''} songs`}
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        color="primary"
                        onClick={() => indexRAGSongs(true)}
                        disabled={
                          !ragStatus?.enabled ||
                          !isRAGIndexLimitValid ||
                          isIndexingRAG ||
                          isRefreshingRAG ||
                          isAddingQdrantLyrics ||
                          isClearingRAG
                        }
                      >
                        {isRefreshingRAG
                          ? 'Refreshing…'
                          : `Refresh ${isRAGIndexLimitValid ? parsedRAGIndexLimit : ''} indexed songs`}
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        onClick={addQdrantLyrics}
                        disabled={
                          !ragStatus?.enabled ||
                          isIndexingRAG ||
                          isRefreshingRAG ||
                          isAddingQdrantLyrics ||
                          isClearingRAG
                        }
                        style={{ color: '#c084fc', borderColor: '#a855f7' }}
                      >
                        {isAddingQdrantLyrics
                          ? 'Adding Qdrant lyrics…'
                          : 'Add Qdrant lyrics'}
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        onClick={openQdrantLyrics}
                        disabled={
                          !ragStatus?.enabled ||
                          !ragStatus?.vectorDbOnline ||
                          !ragStatus?.collectionExists ||
                          isClearingRAG
                        }
                        style={{ color: '#c084fc', borderColor: '#a855f7' }}
                      >
                        View Qdrant lyrics
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        color="primary"
                        onClick={openRAGDocuments}
                        disabled={
                          !ragStatus?.enabled ||
                          !ragStatus?.vectorDbOnline ||
                          !ragStatus?.collectionExists ||
                          isClearingRAG
                        }
                      >
                        View indexed songs
                      </Button>
                      <Button
                        size="small"
                        variant="outlined"
                        color="primary"
                        onClick={clearRAGIndex}
                        disabled={
                          !ragStatus?.enabled ||
                          !ragStatus?.vectorDbOnline ||
                          isIndexingRAG ||
                          isRefreshingRAG ||
                          isAddingQdrantLyrics ||
                          isClearingRAG
                        }
                      >
                        {isClearingRAG ? 'Clearing…' : 'Clear indexed songs'}
                      </Button>
                      <Typography
                        className={classes.ragIndexHint}
                        variant="body2"
                      >
                        Index skips unchanged songs. Refresh updates the latest
                        information for songs already stored in Qdrant.
                      </Typography>
                    </Box>
                    {ragIndexMessage ? (
                      <Typography
                        className={classes.ragIndexMessage}
                        variant="body2"
                      >
                        {ragIndexMessage}
                      </Typography>
                    ) : null}
                    {ragIndexError ? (
                      <Typography
                        className={classes.ragStatusErrorText}
                        variant="body2"
                      >
                        {ragIndexError}
                      </Typography>
                    ) : null}
                    <Box className={classes.ragSearchControls}>
                      <TextField
                        className={classes.ragSearchInput}
                        variant="outlined"
                        size="small"
                        value={ragSearchQuery}
                        onChange={(event) =>
                          setRAGSearchQuery(event.target.value)
                        }
                        onKeyPress={(event) => {
                          if (event.key === 'Enter') searchRAGSongs()
                        }}
                        placeholder="Test RAG search"
                      />
                      <Button
                        size="small"
                        variant="outlined"
                        color="primary"
                        onClick={searchRAGSongs}
                        disabled={
                          !ragStatus?.enabled ||
                          !ragSearchQuery.trim() ||
                          isSearchingRAG
                        }
                      >
                        {isSearchingRAG ? 'Searching…' : 'Search RAG'}
                      </Button>
                    </Box>
                    <Box
                      className={classes.ragFilterControls}
                      aria-label="RAG search filters"
                    >
                      <FormControlLabel
                        className={classes.ragFilterCheckbox}
                        control={
                          <Checkbox
                            checked={ragSearchFilters.cleanOnly}
                            onChange={(event) =>
                              updateRAGSearchFilter(
                                'cleanOnly',
                                event.target.checked,
                              )
                            }
                          />
                        }
                        label="Clean only"
                      />
                      <TextField
                        className={classes.ragFilterInput}
                        label="Genre"
                        variant="outlined"
                        size="small"
                        value={ragSearchFilters.genre}
                        inputProps={{ 'aria-label': 'Genre' }}
                        onChange={(event) =>
                          updateRAGSearchFilter('genre', event.target.value)
                        }
                      />
                      <TextField
                        className={classes.ragFilterInput}
                        label="Lyrics contain"
                        variant="outlined"
                        size="small"
                        value={ragSearchFilters.lyricsContains}
                        inputProps={{ 'aria-label': 'Lyrics contain' }}
                        onChange={(event) =>
                          updateRAGSearchFilter(
                            'lyricsContains',
                            event.target.value,
                          )
                        }
                      />
                      {[
                        ['yearMin', 'Year min'],
                        ['yearMax', 'Year max'],
                        ['bpmMin', 'BPM min'],
                        ['bpmMax', 'BPM max'],
                        ['lufsMin', 'LUFS min'],
                        ['lufsMax', 'LUFS max'],
                        ['playCountMax', 'Max play count'],
                        ['durationMax', 'Max duration (sec)'],
                      ].map(([name, label]) => (
                        <TextField
                          className={classes.ragFilterInput}
                          key={name}
                          label={label}
                          type="number"
                          variant="outlined"
                          size="small"
                          value={ragSearchFilters[name]}
                          inputProps={{ 'aria-label': label }}
                          onChange={(event) =>
                            updateRAGSearchFilter(name, event.target.value)
                          }
                        />
                      ))}
                      {[
                        ['hasLyrics', 'Has lyrics'],
                        ['hasGenre', 'Has genre'],
                        ['hasYear', 'Has year'],
                        ['hasBpm', 'Has BPM'],
                        ['hasLufs', 'Has LUFS'],
                      ].map(([name, label]) => (
                        <FormControlLabel
                          className={classes.ragFilterCheckbox}
                          key={name}
                          control={
                            <Checkbox
                              checked={ragSearchFilters[name]}
                              onChange={(event) =>
                                updateRAGSearchFilter(
                                  name,
                                  event.target.checked,
                                )
                              }
                            />
                          }
                          label={label}
                        />
                      ))}
                    </Box>
                    {ragSearchError ? (
                      <Typography
                        className={classes.ragStatusErrorText}
                        variant="body2"
                      >
                        {ragSearchError}
                      </Typography>
                    ) : null}
                    {ragAppliedFilters !== null ? (
                      <Typography
                        className={classes.ragAppliedFilters}
                        variant="body2"
                      >
                        Applied filters: {JSON.stringify(ragAppliedFilters)} ·{' '}
                        {ragSearchCount} result{ragSearchCount === 1 ? '' : 's'}
                      </Typography>
                    ) : null}
                    {ragSearchResults.length ? (
                      <Box className={classes.ragSearchResults}>
                        {ragSearchResults.map((result, index) => (
                          <Box
                            className={classes.ragSearchResult}
                            key={`${result.songId || 'song'}-${index}`}
                          >
                            {result.title || 'Unknown title'} —{' '}
                            {result.artist || 'Unknown artist'} · score{' '}
                            {Number(result.score || 0).toFixed(3)} ·{' '}
                            {result.genre || 'Unknown genre'} ·{' '}
                            {result.explicit ? 'Explicit' : 'Clean'}
                          </Box>
                        ))}
                      </Box>
                    ) : null}
                    {ragStatus?.error || ragStatusError ? (
                      <Typography
                        className={classes.ragStatusErrorText}
                        variant="body2"
                      >
                        {ragStatus?.error || ragStatusError}
                      </Typography>
                    ) : null}
                  </Box>
                </Collapse>
              </Box>
            </Box>
          </Box>
          <Box className={classes.songToolsPanel}>
            <button
              type="button"
              className={classes.songToolsHeader}
              onClick={() => setIsSongToolsOpen((open) => !open)}
              aria-expanded={isSongToolsOpen}
            >
              <Typography component="span">Song tools</Typography>
              <Typography component="span" className={classes.songToolsSummary}>
                {selectedAddedIds.length
                  ? `${selectedAddedIds.length} selected · `
                  : ''}
                {isSongToolsOpen ? '−' : '+'}
              </Typography>
            </button>
            <Collapse in={isSongToolsOpen}>
              <Box
                className={`${classes.tableActions} ${classes.songToolsActions}`}
              >
                <Button
                  variant="outlined"
                  color="primary"
                  onClick={openAddSongsDialog}
                >
                  {translate('menu.aiTool.addSongs', { _: 'Add songs' })}
                </Button>
                <Button
                  variant="outlined"
                  color="primary"
                  className={`${classes.songToolsActionButton} ${
                    explicitMenuAnchorEl
                      ? classes.songToolsActionButtonOpen
                      : ''
                  }`}
                  aria-controls={
                    explicitMenuAnchorEl ? 'explicit-actions-menu' : undefined
                  }
                  aria-expanded={Boolean(explicitMenuAnchorEl)}
                  aria-haspopup="menu"
                  endIcon={
                    explicitMenuAnchorEl ? (
                      <ExpandLessIcon />
                    ) : (
                      <ExpandMoreIcon />
                    )
                  }
                  onClick={(event) => {
                    setMetadataMenuAnchorEl(null)
                    setExplicitMenuAnchorEl(event.currentTarget)
                  }}
                >
                  Explicit
                </Button>
                <Button
                  variant="outlined"
                  color="primary"
                  className={`${classes.songToolsActionButton} ${
                    metadataMenuAnchorEl
                      ? classes.songToolsActionButtonOpen
                      : ''
                  }`}
                  aria-controls={
                    metadataMenuAnchorEl ? 'metadata-actions-menu' : undefined
                  }
                  aria-expanded={Boolean(metadataMenuAnchorEl)}
                  aria-haspopup="menu"
                  endIcon={
                    metadataMenuAnchorEl ? (
                      <ExpandLessIcon />
                    ) : (
                      <ExpandMoreIcon />
                    )
                  }
                  onClick={(event) => {
                    setExplicitMenuAnchorEl(null)
                    setMetadataMenuAnchorEl(event.currentTarget)
                  }}
                >
                  Metadata
                </Button>
                <Button
                  variant="outlined"
                  color="secondary"
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
            </Collapse>
            <Menu
              id="explicit-actions-menu"
              anchorEl={explicitMenuAnchorEl}
              open={Boolean(explicitMenuAnchorEl)}
              onClose={() => setExplicitMenuAnchorEl(null)}
              className={classes.songToolsMenu}
              PaperProps={{ elevation: 0 }}
              getContentAnchorEl={null}
              anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
              transformOrigin={{ vertical: 'top', horizontal: 'left' }}
              MenuListProps={{ 'aria-label': 'Explicit actions' }}
            >
              <MenuItem
                className={`${classes.songToolsMenuItem} ${classes.songToolsMenuItemPrimary}`}
                onClick={() => {
                  setExplicitMenuAnchorEl(null)
                  openExplicitDialog(selectedSongsAvailableForExplicit)
                }}
                disabled={
                  !selectedSongsAvailableForExplicit.length ||
                  isClassifyingExplicit
                }
              >
                <LabelOutlinedIcon fontSize="small" />
                {isClassifyingExplicit
                  ? translate('menu.aiTool.classifyingExplicit', {
                      _: 'Classifying...',
                    })
                  : translate('menu.aiTool.classifyExplicit', {
                      _: 'Classify Explicit',
                    })}
              </MenuItem>
              <MenuItem
                className={classes.songToolsMenuItem}
                onClick={() => {
                  setExplicitMenuAnchorEl(null)
                  setExplicitRulesOpen(true)
                }}
              >
                <SettingsApplicationsIcon fontSize="small" />
                Explicit word rules
              </MenuItem>
              <MenuItem
                className={classes.songToolsMenuItem}
                onClick={() => {
                  setExplicitMenuAnchorEl(null)
                  void fetchSelectedLyrics()
                }}
                disabled={
                  selectedSongsAvailableForLyrics.length === 0 ||
                  isLyricsJobRunning
                }
              >
                <LibraryMusicIcon fontSize="small" />
                {lyricsProgressStatus === 'stopping'
                  ? translate('menu.aiTool.stoppingLyrics', {
                      _: 'Stopping Lyrics...',
                    })
                  : lyricsLoadingId
                    ? translate('menu.aiTool.fetchingLyrics', {
                        _: 'Fetching Lyrics...',
                      })
                    : translate('menu.aiTool.fetchLyrics', {
                        _: 'Fetch Lyrics',
                      })}
              </MenuItem>
              <MenuItem
                disableRipple
                className={classes.songToolsMenuControl}
                onClick={(event) => event.stopPropagation()}
              >
                <Box
                  className={`${classes.songToolsMenuToggle} ${
                    isAutoFetchAllLyricsEnabled
                      ? classes.songToolsMenuToggleEnabled
                      : ''
                  }`}
                >
                  <Box className={classes.songToolsMenuToggleText}>
                    <Typography className={classes.songToolsMenuToggleLabel}>
                      Fetch all song lyrics
                    </Typography>
                    <Typography className={classes.songToolsMenuToggleHint}>
                      Automatically fill lyrics for added songs
                    </Typography>
                  </Box>
                  <Box className={classes.songToolsMenuToggleAction}>
                    <Typography className={classes.songToolsMenuToggleStatus}>
                      {isAutoFetchAllLyricsEnabled ? 'On' : 'Off'}
                    </Typography>
                    <Switch
                      checked={isAutoFetchAllLyricsEnabled}
                      onChange={toggleAutoFetchAllLyrics}
                      color="primary"
                      inputProps={{ 'aria-label': 'Fetch All Song Lyrics' }}
                    />
                  </Box>
                </Box>
              </MenuItem>
              <MenuItem
                className={classes.songToolsMenuItem}
                onClick={() => {
                  setExplicitMenuAnchorEl(null)
                  void deleteLyricsForSongs(selectedAddedSongs)
                }}
                disabled={
                  selectedSongsAvailableForLyricsDeletion.length === 0 ||
                  isDeletingLyrics
                }
              >
                <DeleteOutlineIcon fontSize="small" />
                {isDeletingLyrics ? 'Deleting lyrics…' : 'Delete Lyrics'}
              </MenuItem>
            </Menu>
            <Menu
              id="metadata-actions-menu"
              anchorEl={metadataMenuAnchorEl}
              open={Boolean(metadataMenuAnchorEl)}
              onClose={() => setMetadataMenuAnchorEl(null)}
              className={classes.songToolsMenu}
              PaperProps={{ elevation: 0 }}
              getContentAnchorEl={null}
              anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
              transformOrigin={{ vertical: 'top', horizontal: 'left' }}
              MenuListProps={{ 'aria-label': 'Metadata actions' }}
            >
              <MenuItem
                className={`${classes.songToolsMenuItem} ${classes.songToolsMenuItemPrimary}`}
                onClick={() => {
                  setMetadataMenuAnchorEl(null)
                  void fetchAIMetadataForSongs(
                    selectedAddedSongs,
                    metadataProvider,
                  )
                }}
                disabled={
                  !selectedAddedIds.length ||
                  isFetchingMetadata ||
                  isClearingMetadata
                }
              >
                <QueueMusicIcon fontSize="small" />
                {metadataProgressStatus === 'stopping'
                  ? translate('menu.aiTool.stoppingMetadata', {
                      _: 'Stopping Metadata...',
                    })
                  : isFetchingMetadata
                    ? translate('menu.aiTool.fetchingMetadata', {
                        _: 'Fetching Metadata...',
                      })
                    : translate('menu.aiTool.fetchAIMetadata', {
                        _: 'Fetch AI Metadata',
                      })}
              </MenuItem>
              <MenuItem
                disableRipple
                className={classes.songToolsMenuControl}
                onClick={(event) => event.stopPropagation()}
              >
                <Box
                  className={`${classes.songToolsMenuToggle} ${
                    isAutoFetchAllMetadataEnabled
                      ? classes.songToolsMenuToggleEnabled
                      : ''
                  }`}
                >
                  <Box className={classes.songToolsMenuToggleText}>
                    <Typography className={classes.songToolsMenuToggleLabel}>
                      Fetch all song metadata
                    </Typography>
                    <Typography className={classes.songToolsMenuToggleHint}>
                      Refresh metadata for added songs every 10 minutes
                    </Typography>
                  </Box>
                  <Box className={classes.songToolsMenuToggleAction}>
                    <Typography className={classes.songToolsMenuToggleStatus}>
                      {isAutoFetchAllMetadataEnabled ? 'On' : 'Off'}
                    </Typography>
                    <Switch
                      checked={isAutoFetchAllMetadataEnabled}
                      onChange={toggleAutoFetchAllMetadata}
                      color="primary"
                      inputProps={{
                        'aria-label': 'Fetch All Song Metadata',
                      }}
                    />
                  </Box>
                </Box>
              </MenuItem>
              <MenuItem
                disableRipple
                className={classes.songToolsMenuControl}
                onClick={(event) => event.stopPropagation()}
              >
                <TextField
                  select
                  fullWidth
                  className={`${classes.defaultProviderSelect} ${classes.songToolsMenuSelect}`}
                  label="Default Metadata AI Provider"
                  variant="outlined"
                  size="small"
                  value={metadataProvider}
                  inputProps={{
                    'aria-label': 'Default Metadata AI Provider',
                  }}
                  onChange={(event) =>
                    setMetadataProvider(normalizeAIProvider(event.target.value))
                  }
                >
                  {AI_PROVIDERS.map((provider) => (
                    <MenuItem key={provider.id} value={provider.id}>
                      {provider.label}
                    </MenuItem>
                  ))}
                </TextField>
              </MenuItem>
              <MenuItem
                disableRipple
                className={classes.songToolsMenuControl}
                onClick={(event) => event.stopPropagation()}
              >
                <TextField
                  select
                  fullWidth
                  className={`${classes.defaultProviderSelect} ${classes.songToolsMenuSelect}`}
                  label="Songs per AI Prompt"
                  variant="outlined"
                  size="small"
                  value={metadataBatchSize}
                  disabled={isFetchingMetadata}
                  inputProps={{ 'aria-label': 'Songs per AI Prompt' }}
                  onChange={(event) =>
                    setMetadataBatchSize(
                      normalizeMetadataBatchSize(event.target.value),
                    )
                  }
                >
                  {METADATA_BATCH_SIZE_OPTIONS.map((size) => (
                    <MenuItem key={size} value={size}>
                      {size}
                    </MenuItem>
                  ))}
                </TextField>
              </MenuItem>
              <MenuItem
                className={classes.songToolsMenuItem}
                onClick={() => {
                  setColumnMenuAnchorEl(metadataMenuAnchorEl)
                  setMetadataMenuAnchorEl(null)
                }}
              >
                <ViewColumnIcon fontSize="small" />
                {translate('ra.toggleFieldsMenu.columnsToDisplay', {
                  _: 'Columns to display',
                })}
              </MenuItem>
              <MenuItem
                className={classes.songToolsMenuItem}
                onClick={() => {
                  toggleAllConfidenceColumns()
                  setMetadataMenuAnchorEl(null)
                }}
              >
                {someConfidenceColumnsVisible
                  ? 'Hide Confidence'
                  : 'Show Confidence'}
              </MenuItem>
              <MenuItem
                className={classes.songToolsMenuItem}
                onClick={() => {
                  setMetadataMenuAnchorEl(null)
                  void clearFetchedMetadata()
                }}
                disabled={
                  !selectedAddedIds.length ||
                  isClearingMetadata ||
                  isFetchingMetadata
                }
              >
                {isClearingMetadata
                  ? translate('menu.aiTool.clearingMetadata', {
                      _: 'Clearing...',
                    })
                  : translate('menu.aiTool.clearFetchedMetadata', {
                      _: 'Clear Fetched Metadata',
                    })}
              </MenuItem>
            </Menu>
          </Box>

          {['lyrics', 'metadata'].map((type) => {
            const progress = jobProgresses[type]
            if (
              !progress ||
              (type === 'lyrics' && isLyricsProgressHidden) ||
              (type === 'metadata' && isMetadataProgressHidden)
            )
              return null
            const progressValue = progress.total
              ? Math.round((progress.done / progress.total) * 100)
              : 0
            const running =
              type === 'lyrics' ? isLyricsJobRunning : isFetchingMetadata
            return (
              <Box className={classes.progressPanel} key={type}>
                <Box className={classes.progressHeader}>
                  <Typography variant="body2" className={classes.progressText}>
                    {type === 'lyrics'
                      ? 'Fetching lyrics'
                      : 'Fetching AI metadata'}
                    : {progress.currentTitle}
                  </Typography>
                  <Box className={classes.progressActions}>
                    <Typography
                      variant="body2"
                      className={classes.progressMeta}
                    >
                      {progress.done}/{progress.total} done,{' '}
                      {Math.max(progress.total - progress.done, 0)} left
                      {type === 'lyrics' && lyricsTimingText
                        ? ` · ${lyricsTimingText}`
                        : ''}
                    </Typography>
                    {running ? (
                      <Button
                        className={classes.stopJobButton}
                        variant="outlined"
                        color="secondary"
                        size="small"
                        onClick={() => stopFetchJob(type)}
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
                  value={progressValue}
                />
              </Box>
            )
          })}

          <Box className={classes.tableWrap}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell padding="checkbox">
                    <Checkbox
                      inputProps={{ 'aria-label': 'Select all added songs' }}
                      checked={
                        addedSongs.length > 0 &&
                        selectedAddedIds.length === addedSongs.length
                      }
                      indeterminate={
                        selectedAddedIds.length > 0 &&
                        selectedAddedIds.length < addedSongs.length
                      }
                      onChange={toggleAllAddedSongs}
                    />
                  </TableCell>
                  {isColumnVisible('title') ? (
                    <TableCell>
                      {translate('resources.song.fields.title', { _: 'Title' })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('album') ? (
                    <TableCell>
                      {translate('resources.song.fields.album', { _: 'Album' })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('artist') ? (
                    <TableCell>
                      {translate('resources.song.fields.artist', {
                        _: 'Artist',
                      })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('year') ? (
                    <TableCell>
                      {translate('resources.song.fields.year', { _: 'Year' })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('explicit') ? (
                    <TableCell>
                      {translate('resources.song.fields.explicitStatus', {
                        _: 'Explicit',
                      })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('lyrics') ? (
                    <TableCell>
                      {translate('menu.aiTool.lyrics', { _: 'Lyrics' })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('duration') ? (
                    <TableCell>
                      {translate('resources.song.fields.duration', {
                        _: 'Time',
                      })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('genre') ? (
                    <TableCell>
                      {translate('resources.song.fields.genre', { _: 'Genre' })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('spotifyGenre') ? (
                    <TableCell>Spotify Genre</TableCell>
                  ) : null}
                  {isColumnVisible('musicBrainzGenre') ? (
                    <TableCell>iTunes Genre</TableCell>
                  ) : null}
                  {isColumnVisible('aiGenre') ? (
                    <TableCell>
                      {translate('menu.aiTool.aiGenre', { _: 'AI Genre' })}
                    </TableCell>
                  ) : null}
                  {isColumnVisible('aiSubgenre') ? (
                    <TableCell>AI Subgenre</TableCell>
                  ) : null}
                  {isColumnVisible('genreConfidence') ? (
                    <TableCell className={classes.confidenceColumn}>
                      Genre Confidence
                    </TableCell>
                  ) : null}
                  {isColumnVisible('tokens') ? (
                    <TableCell className={classes.confidenceColumn}>
                      Tokens
                    </TableCell>
                  ) : null}
                  <TableCell>
                    {translate('ra.action.actions', { _: 'Actions' })}
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {addedSongs.map((song) => (
                  <TableRow
                    key={song.id}
                    className={
                      currentLyricsFetchingSongId === song.id
                        ? classes.lyricsFetchingRow
                        : undefined
                    }
                    data-lyrics-fetching={
                      currentLyricsFetchingSongId === song.id
                        ? 'true'
                        : undefined
                    }
                  >
                    <TableCell padding="checkbox">
                      <Checkbox
                        checked={selectedAddedSongIdSet.has(song.id)}
                        onChange={() => toggleAddedSong(song.id)}
                      />
                    </TableCell>
                    {isColumnVisible('title') ? (
                      <TableCell className={classes.valueExisting}>
                        {song.title || ''}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('album') ? (
                      <TableCell className={valueClass(song, 'album')}>
                        {song.album || ''}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('artist') ? (
                      <TableCell className={classes.valueExisting}>
                        {song.artist || ''}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('year') ? (
                      <TableCell className={valueClass(song, 'year')}>
                        {song.year || ''}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('explicit') ? (
                      <TableCell className={valueClass(song, 'explicitStatus')}>
                        {formatExplicitStatus(song.explicitStatus) ? (
                          <Button
                            size="small"
                            color="primary"
                            onClick={() => setExplicitReasonSong(song)}
                          >
                            {formatExplicitStatus(song.explicitStatus)}
                          </Button>
                        ) : null}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('lyrics') ? (
                      <TableCell className={valueClass(song, 'lyrics')}>
                        {renderLyricsState(song)}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('duration') ? (
                      <TableCell className={classes.valueExisting}>
                        {formatDuration(song.duration)}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('genre') ? (
                      <TableCell className={classes.valueExisting}>
                        {song.genre || ''}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('spotifyGenre') ? (
                      <TableCell className={valueClass(song, 'spotifyGenre')}>
                        {song.spotifyGenre || '-'}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('musicBrainzGenre') ? (
                      <TableCell
                        className={valueClass(song, 'musicBrainzGenre')}
                      >
                        {renderFetchedGenre(song, 'itunes')}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('aiGenre') ? (
                      <TableCell className={valueClass(song, 'aiGenre')}>
                        {renderFetchedGenre(song, 'ai')}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('aiSubgenre') ? (
                      <TableCell className={valueClass(song, 'aiSubgenre')}>
                        {song.aiSubgenre || '-'}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('genreConfidence') ? (
                      <TableCell className={classes.confidenceColumn}>
                        {renderMetadataConfidence(
                          song.metadataConfidence?.genre,
                          song,
                          'genre',
                        ) || '—'}
                      </TableCell>
                    ) : null}
                    {isColumnVisible('tokens') ? (
                      <TableCell className={classes.confidenceColumn}>
                        {renderTokenUsage(song)}
                      </TableCell>
                    ) : null}
                    <TableCell>
                      <IconButton
                        size="small"
                        onClick={(event) => openRowActions(event, song)}
                        aria-label={translate('ra.action.actions', {
                          _: 'Actions',
                        })}
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
          {translate('ra.toggleFieldsMenu.columnsToDisplay', {
            _: 'Columns to display',
          })}
        </Typography>
        <Box className={classes.columnMenuItems}>
          <MenuItem onClick={toggleAllConfidenceColumns}>
            <Checkbox
              checked={allConfidenceColumnsVisible}
              indeterminate={
                someConfidenceColumnsVisible && !allConfidenceColumnsVisible
              }
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
        <MenuItem
          onClick={() => runRowAction('fetchLyrics')}
          disabled={
            isLyricsJobRunning ||
            explicitClassifyingSongIdSet.has(rowActionSong?.id)
          }
        >
          {lyricsLoadingId === rowActionSong?.id
            ? translate('menu.aiTool.fetchingLyrics', {
                _: 'Fetching Lyrics...',
              })
            : translate('menu.aiTool.fetchLyrics', { _: 'Fetch Lyrics' })}
        </MenuItem>
        <MenuItem
          onClick={() => runRowAction('showLyrics')}
          disabled={!hasSavedLyrics(rowActionSong)}
        >
          {translate('menu.aiTool.showLyrics', { _: 'Show Lyrics' })}
        </MenuItem>
        <MenuItem
          onClick={() => runRowAction('deleteLyrics')}
          disabled={
            !hasSavedLyrics(rowActionSong) ||
            isDeletingLyrics ||
            lyricsFetchingSongIdSet.has(rowActionSong?.id) ||
            explicitClassifyingSongIdSet.has(rowActionSong?.id)
          }
        >
          Delete Lyrics
        </MenuItem>
        <MenuItem
          onClick={() => runRowAction('fetchMetadata')}
          disabled={isFetchingMetadata || isClearingMetadata}
        >
          {isFetchingMetadata
            ? translate('menu.aiTool.fetchingMetadata', {
                _: 'Fetching Metadata...',
              })
            : translate('menu.aiTool.fetchAIMetadata', {
                _: 'Fetch AI Metadata',
              })}
        </MenuItem>
        <MenuItem onClick={() => runRowAction('removeSong')}>
          {translate('menu.aiTool.removeSong', { _: 'Remove Song' })}
        </MenuItem>
      </Menu>

      <Dialog
        open={explicitDialogSongs.length > 0}
        onClose={closeExplicitDialog}
        fullWidth
        maxWidth="xs"
      >
        <DialogTitle>
          {translate('menu.aiTool.classifyExplicit', {
            _: 'Classify Explicit',
          })}
        </DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            {`DeepSeek V3.2 will analyze the saved lyrics for ${
              explicitDialogSongs.length
            } ${explicitDialogSongs.length === 1 ? 'song' : 'songs'}.`}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeExplicitDialog}>
            {translate('ra.action.cancel', { _: 'Cancel' })}
          </Button>
          <Button
            color="primary"
            variant="contained"
            onClick={runExplicitAction}
            disabled={
              isClassifyingExplicit ||
              !explicitDialogSongs.some(
                (song) => !lyricsFetchingSongIdSet.has(song.id),
              )
            }
          >
            {translate('menu.aiTool.classify', { _: 'Classify' })}
          </Button>
        </DialogActions>
      </Dialog>

      {isChatOpen ? (
        <Box
          className={classes.chatWidget}
          style={isChatExpanded ? expandedChatFrame() : chatFrame}
          role="dialog"
          aria-label="RAG"
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
            <Box className={classes.chatAvatar}>RAG</Box>
            <Box className={classes.chatTitleWrap}>
              <Typography className={classes.chatTitle}>RAG</Typography>
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
                {selectedModelOnline === null ||
                selectedModelOnline === undefined
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
          <Box className={classes.chatTraceToggleBar}>
            <FormControlLabel
              control={
                <Switch
                  size="small"
                  checked={developerTraceEnabled}
                  onChange={(event) =>
                    changeDeveloperTrace(event.target.checked)
                  }
                  inputProps={{ 'aria-label': 'Developer Trace' }}
                />
              }
              label="Developer Trace"
            />
            <Typography className={classes.chatTraceHint}>
              Prompts · retrieval · raw responses
            </Typography>
          </Box>
          <Box className={classes.chatMessages} ref={chatMessagesRef}>
            {messages.length === 0 ? (
              <Box className={classes.chatMessageRow}>
                <Box
                  className={`${classes.chatBubble} ${classes.chatBubbleAssistant}`}
                >
                  Ask me about songs in your indexed library.
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
                  <Box
                    className={
                      message.role === 'assistant'
                        ? classes.chatAssistantContent
                        : undefined
                    }
                  >
                    <Typography
                      className={classes.chatMessageLabel}
                      align={message.role === 'user' ? 'right' : 'left'}
                    >
                      {message.role === 'user'
                        ? 'You'
                        : message.direct
                          ? 'Qdrant exact search'
                          : aiProviderLabel(message.provider)}
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
                    {message.role === 'assistant' &&
                    !message.direct &&
                    message.sources?.length ? (
                      <Box className={classes.chatSources}>
                        Sources
                        {message.sources.map((source, sourceIndex) => (
                          <span
                            className={classes.chatSource}
                            key={`${source.songId || 'source'}-${sourceIndex}`}
                          >
                            {source.title || 'Unknown title'} —{' '}
                            {source.artist || 'Unknown artist'} ·{' '}
                            {Number(source.score || 0).toFixed(3)}
                            {source.lyricSnippet ? (
                              <span className={classes.chatLyricSnippet}>
                                {source.lyricSnippet}
                              </span>
                            ) : null}
                          </span>
                        ))}
                      </Box>
                    ) : null}
                    {message.role === 'assistant' && message.ragError ? (
                      <Typography className={classes.chatRAGWarning}>
                        RAG unavailable: {message.ragError}. Answered without
                        library context.
                      </Typography>
                    ) : null}
                    {message.role === 'assistant' && message.trace ? (
                      <ChatDeveloperTrace
                        trace={message.trace}
                        classes={classes}
                      />
                    ) : null}
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
                  <Box
                    className={`${classes.chatBubble} ${classes.chatBubbleAssistant}`}
                  >
                    <Box
                      className={classes.chatThinking}
                      component="span"
                      title="Thinking"
                    >
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
              placeholder="Ask RAG about your library..."
            />
            <Button
              className={
                isSending ? classes.chatStopButton : classes.chatSendButton
              }
              variant="contained"
              onClick={isSending ? stopMessage : sendMessage}
              title={isSending ? 'Stop RAG response' : 'Send RAG message'}
            >
              {isSending ? <StopIcon fontSize="small" /> : '>'}
            </Button>
          </Box>
          {chatError ? (
            <Typography className={classes.chatError} variant="body2">
              {chatError}
            </Typography>
          ) : null}
          {chatErrorTrace ? (
            <Box className={classes.chatErrorTrace}>
              <ChatDeveloperTrace trace={chatErrorTrace} classes={classes} />
            </Box>
          ) : null}
        </Box>
      ) : (
        <Button
          className={`${classes.chatLauncher} ${classes.ragChatLauncher}`}
          onClick={() => setIsChatOpen(true)}
          aria-label="Open RAG"
        >
          RAG
        </Button>
      )}

      {isNormalChatOpen ? (
        <Box
          className={classes.chatWidget}
          style={isNormalChatExpanded ? expandedChatFrame() : normalChatFrame}
          role="dialog"
          aria-label="AI Chat"
        >
          <Box
            className={`${classes.chatResizeHandle} ${classes.chatResizeTopLeft}`}
            onMouseDown={(event) => startNormalChatResize('top-left', event)}
            title="Resize normal chat"
          />
          <Box
            className={`${classes.chatResizeHandle} ${classes.chatResizeTopRight}`}
            onMouseDown={(event) => startNormalChatResize('top-right', event)}
            title="Resize normal chat"
          />
          <Box
            className={`${classes.chatResizeHandle} ${classes.chatResizeBottomLeft}`}
            onMouseDown={(event) => startNormalChatResize('bottom-left', event)}
            title="Resize normal chat"
          />
          <Box
            className={`${classes.chatResizeHandle} ${classes.chatResizeBottomRight}`}
            onMouseDown={(event) =>
              startNormalChatResize('bottom-right', event)
            }
            title="Resize normal chat"
          />
          <Box className={classes.chatHeader}>
            <Box className={classes.chatAvatar}>AI</Box>
            <Box className={classes.chatTitleWrap}>
              <Typography className={classes.chatTitle}>AI Chat</Typography>
              <Typography
                className={`${classes.chatStatus} ${
                  selectedNormalModelOnline === true
                    ? classes.chatStatusOnline
                    : selectedNormalModelOnline === false
                      ? classes.chatStatusOffline
                      : ''
                }`}
              >
                {aiProviderLabel(normalChatProvider)} -{' '}
                {selectedNormalModelOnline === null ||
                selectedNormalModelOnline === undefined
                  ? 'Checking…'
                  : selectedNormalModelOnline
                    ? 'Online'
                    : 'Offline'}
              </Typography>
            </Box>
            <Box className={classes.chatHeaderActions}>
              <Button
                className={classes.chatClose}
                variant="outlined"
                onClick={() => setIsNormalChatExpanded((prev) => !prev)}
                title={
                  isNormalChatExpanded
                    ? 'Collapse normal chat'
                    : 'Expand normal chat'
                }
              >
                <AspectRatioIcon fontSize="small" />
              </Button>
              <Button
                className={classes.chatClose}
                variant="outlined"
                onClick={() => setIsNormalChatOpen(false)}
                title="Close normal chat"
              >
                x
              </Button>
            </Box>
          </Box>
          <Box className={classes.chatTraceToggleBar}>
            <FormControlLabel
              control={
                <Switch
                  size="small"
                  checked={developerTraceEnabled}
                  onChange={(event) =>
                    changeDeveloperTrace(event.target.checked)
                  }
                  inputProps={{ 'aria-label': 'Developer Trace' }}
                />
              }
              label="Developer Trace"
            />
            <Typography className={classes.chatTraceHint}>
              Prompts · provider · raw responses
            </Typography>
          </Box>
          <Box className={classes.chatMessages} ref={normalChatMessagesRef}>
            {normalMessages.length === 0 ? (
              <Box className={classes.chatMessageRow}>
                <Box
                  className={`${classes.chatBubble} ${classes.chatBubbleAssistant}`}
                >
                  Hi. What can I help with today?
                </Box>
              </Box>
            ) : (
              normalMessages.map((message, index) => (
                <Box
                  key={`normal-${message.role}-${index}`}
                  className={`${classes.chatMessageRow} ${
                    message.role === 'user' ? classes.chatMessageRowUser : ''
                  }`}
                >
                  <Box
                    className={
                      message.role === 'assistant'
                        ? classes.chatAssistantContent
                        : undefined
                    }
                  >
                    <Typography
                      className={classes.chatMessageLabel}
                      align={message.role === 'user' ? 'right' : 'left'}
                    >
                      {message.role === 'user'
                        ? 'You'
                        : aiProviderLabel(message.provider)}
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
                    {message.role === 'assistant' && message.trace ? (
                      <ChatDeveloperTrace
                        trace={message.trace}
                        classes={classes}
                      />
                    ) : null}
                  </Box>
                </Box>
              ))
            )}
            {isNormalSending ? (
              <Box className={classes.chatMessageRow}>
                <Box>
                  <Typography className={classes.chatMessageLabel}>
                    {aiProviderLabel(normalChatProvider)}
                  </Typography>
                  <Box
                    className={`${classes.chatBubble} ${classes.chatBubbleAssistant}`}
                  >
                    <Box
                      className={classes.chatThinking}
                      component="span"
                      title="Normal chat thinking"
                    >
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
              value={normalChatProvider}
              onChange={(event) => {
                setNormalChatProvider(normalizeAIProvider(event.target.value))
                setNormalChatProviderOverridden(true)
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
              value={normalPrompt}
              onChange={(event) => setNormalPrompt(event.target.value)}
              onKeyPress={(event) => {
                if (event.key === 'Enter') sendNormalMessage()
              }}
              placeholder="Ask AI anything..."
            />
            <Button
              className={
                isNormalSending
                  ? classes.chatStopButton
                  : classes.chatSendButton
              }
              variant="contained"
              onClick={isNormalSending ? stopNormalMessage : sendNormalMessage}
              title={
                isNormalSending ? 'Stop normal response' : 'Send normal message'
              }
            >
              {isNormalSending ? <StopIcon fontSize="small" /> : '>'}
            </Button>
          </Box>
          {normalChatError ? (
            <Typography className={classes.chatError} variant="body2">
              {normalChatError}
            </Typography>
          ) : null}
          {normalChatErrorTrace ? (
            <Box className={classes.chatErrorTrace}>
              <ChatDeveloperTrace
                trace={normalChatErrorTrace}
                classes={classes}
              />
            </Box>
          ) : null}
        </Box>
      ) : (
        <Button
          className={`${classes.chatLauncher} ${classes.normalChatLauncher}`}
          onClick={() => setIsNormalChatOpen(true)}
          aria-label="Open AI Chat"
        >
          AI
        </Button>
      )}

      <Dialog
        open={isRAGDocumentsOpen}
        onClose={() => setIsRAGDocumentsOpen(false)}
        fullWidth
        maxWidth="lg"
        aria-labelledby="rag-documents-title"
      >
        <DialogTitle id="rag-documents-title">Indexed RAG songs</DialogTitle>
        <DialogContent dividers>
          <Typography variant="body2" gutterBottom>
            Collection: {ragDocumentsCollection || ragStatus?.collection || '—'}
            {' · '}Showing {ragDocuments.length} of {ragDocumentsCount} indexed
            songs
          </Typography>
          {isLoadingRAGDocuments ? (
            <Box display="flex" justifyContent="center" p={3}>
              <CircularProgress size={28} />
            </Box>
          ) : ragDocumentsError ? (
            <Typography color="error">{ragDocumentsError}</Typography>
          ) : ragDocuments.length === 0 ? (
            <Typography variant="body2">No indexed songs found.</Typography>
          ) : (
            <Table size="small" stickyHeader aria-label="Indexed songs table">
              <TableHead>
                <TableRow>
                  <TableCell>Song ID</TableCell>
                  <TableCell>Title</TableCell>
                  <TableCell>Artist</TableCell>
                  <TableCell>Album</TableCell>
                  <TableCell>Album Artist</TableCell>
                  <TableCell>Track</TableCell>
                  <TableCell>Year</TableCell>
                  <TableCell>Genre</TableCell>
                  <TableCell>Explicit</TableCell>
                  <TableCell>BPM</TableCell>
                  <TableCell>LUFS</TableCell>
                  <TableCell>Duration</TableCell>
                  <TableCell>Plays</TableCell>
                  <TableCell>Last Played</TableCell>
                  <TableCell>Lyrics</TableCell>
                  <TableCell>Audio</TableCell>
                  <TableCell>Details</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {ragDocuments.map((song) => (
                  <TableRow key={song.songId}>
                    <TableCell>{song.songId}</TableCell>
                    <TableCell>{song.title || '—'}</TableCell>
                    <TableCell>{song.artist || '—'}</TableCell>
                    <TableCell>{song.album || '—'}</TableCell>
                    <TableCell>{song.albumArtist || '—'}</TableCell>
                    <TableCell>
                      {song.trackNumber || '—'} / {song.discNumber || '—'}
                    </TableCell>
                    <TableCell>{song.year || '—'}</TableCell>
                    <TableCell>{song.genre || '—'}</TableCell>
                    <TableCell>
                      {song.explicitStatus ||
                        (song.explicit ? 'explicit' : 'clean')}
                    </TableCell>
                    <TableCell>{song.bpm || '—'}</TableCell>
                    <TableCell>
                      {Number.isFinite(Number(song.lufs))
                        ? Number(song.lufs).toFixed(2)
                        : '—'}
                    </TableCell>
                    <TableCell>{formatDuration(song.duration)}</TableCell>
                    <TableCell>{song.playCount || 0}</TableCell>
                    <TableCell>{song.lastPlayedAt || '—'}</TableCell>
                    <TableCell>{song.hasLyrics ? 'Yes' : 'No'}</TableCell>
                    <TableCell>
                      {song.codec || song.suffix || '—'}
                      {song.bitRate ? ` · ${song.bitRate} kbps` : ''}
                    </TableCell>
                    <TableCell>
                      <Button
                        size="small"
                        onClick={() => setRAGDocumentDetails(song)}
                      >
                        View
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setIsRAGDocumentsOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={Boolean(ragDocumentDetails)}
        onClose={() => setRAGDocumentDetails(null)}
        fullWidth
        maxWidth="md"
      >
        <DialogTitle>Indexed song payload</DialogTitle>
        <DialogContent dividers>
          <Typography component="pre" style={{ whiteSpace: 'pre-wrap' }}>
            {JSON.stringify(ragDocumentDetails, null, 2)}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRAGDocumentDetails(null)}>Close</Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={isQdrantLyricsOpen}
        onClose={() => setIsQdrantLyricsOpen(false)}
        fullWidth
        maxWidth="lg"
        aria-labelledby="qdrant-lyrics-title"
      >
        <DialogTitle id="qdrant-lyrics-title">Qdrant lyrics</DialogTitle>
        <DialogContent dividers>
          <Box display="flex" alignItems="center" mb={2} style={{ gap: 8 }}>
            <TextField
              fullWidth
              size="small"
              variant="outlined"
              label="Search words in Qdrant lyrics"
              inputProps={{ 'aria-label': 'Search words in Qdrant lyrics' }}
              value={qdrantLyricsQuery}
              onChange={(event) => setQdrantLyricsQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') searchQdrantLyrics()
              }}
            />
            <Button
              variant="outlined"
              color="primary"
              onClick={searchQdrantLyrics}
              disabled={isLoadingQdrantLyrics}
            >
              {isLoadingQdrantLyrics ? 'Searching…' : 'Search lyrics'}
            </Button>
            <Button
              variant="outlined"
              onClick={() => {
                setQdrantLyricsQuery('')
                void loadQdrantLyrics('')
              }}
              disabled={isLoadingQdrantLyrics || !qdrantLyricsQuery.trim()}
            >
              Clear search
            </Button>
          </Box>
          <Typography variant="body2" gutterBottom>
            Collection: {qdrantLyricsCollection || ragStatus?.collection || '—'}
            {' · '}
            {qdrantLyricsCount} stored lyric{qdrantLyricsCount === 1 ? '' : 's'}
            {qdrantLyricsQuery.trim()
              ? ` matching “${qdrantLyricsQuery.trim()}”`
              : ''}
          </Typography>
          {isLoadingQdrantLyrics ? (
            <Box display="flex" justifyContent="center" p={3}>
              <CircularProgress size={28} />
            </Box>
          ) : qdrantLyricsError ? (
            <Typography color="error">{qdrantLyricsError}</Typography>
          ) : qdrantLyrics.length === 0 ? (
            <Typography variant="body2">
              {qdrantLyricsQuery.trim()
                ? 'No stored lyrics contain that word or phrase.'
                : 'No lyrics are stored in Qdrant yet.'}
            </Typography>
          ) : (
            <Table size="small" stickyHeader aria-label="Qdrant lyrics table">
              <TableHead>
                <TableRow>
                  <TableCell>Title</TableCell>
                  <TableCell>Artist</TableCell>
                  <TableCell>Album</TableCell>
                  <TableCell>Year</TableCell>
                  <TableCell>Genre</TableCell>
                  <TableCell>Stored lyrics</TableCell>
                  <TableCell>Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {qdrantLyrics.map((song) => {
                  const storedLyrics = String(song.lyricsText || '').trim()
                  const preview =
                    storedLyrics.length > 180
                      ? `${storedLyrics.slice(0, 180)}…`
                      : storedLyrics
                  return (
                    <TableRow key={song.songId}>
                      <TableCell>{song.title || '—'}</TableCell>
                      <TableCell>{song.artist || '—'}</TableCell>
                      <TableCell>{song.album || '—'}</TableCell>
                      <TableCell>{song.year || '—'}</TableCell>
                      <TableCell>{song.genre || '—'}</TableCell>
                      <TableCell style={{ whiteSpace: 'pre-wrap' }}>
                        {preview || '—'}
                      </TableCell>
                      <TableCell>
                        <Button
                          size="small"
                          color="primary"
                          onClick={() => setQdrantLyricDetails(song)}
                        >
                          View lyrics
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setIsQdrantLyricsOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={Boolean(qdrantLyricDetails)}
        onClose={() => setQdrantLyricDetails(null)}
        fullWidth
        maxWidth="md"
        aria-labelledby="qdrant-lyric-details-title"
      >
        <DialogTitle id="qdrant-lyric-details-title">
          {qdrantLyricDetails?.title || 'Stored Qdrant lyrics'}
        </DialogTitle>
        <DialogContent dividers>
          <Typography variant="body2" gutterBottom>
            {qdrantLyricDetails?.artist || 'Unknown artist'}
            {qdrantLyricDetails?.album ? ` · ${qdrantLyricDetails.album}` : ''}
          </Typography>
          <Typography component="pre" style={{ whiteSpace: 'pre-wrap' }}>
            {qdrantLyricDetails?.lyricsText || 'No stored lyrics.'}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setQdrantLyricDetails(null)}>Close</Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={songDialogOpen}
        onClose={() => setSongDialogOpen(false)}
        fullWidth
        maxWidth="lg"
      >
        <DialogTitle>
          {translate('menu.aiTool.addSongs', { _: 'Add songs' })}
        </DialogTitle>
        <DialogContent>
          {songsLoading ? (
            <Typography>
              {translate('menu.retailPlayer.loading', {
                _: 'Loading devices…',
              })}
            </Typography>
          ) : (
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell padding="checkbox">
                    <Checkbox
                      inputProps={{ 'aria-label': 'Select all new songs' }}
                      checked={
                        selectableSongIds.length > 0 &&
                        selectedSelectableSongCount === selectableSongIds.length
                      }
                      indeterminate={
                        selectedSelectableSongCount > 0 &&
                        selectedSelectableSongCount < selectableSongIds.length
                      }
                      disabled={selectableSongIds.length === 0}
                      onChange={toggleAllAvailableSongs}
                    />
                  </TableCell>
                  <TableCell>
                    {translate('resources.song.fields.title', { _: 'Title' })}
                  </TableCell>
                  <TableCell>
                    {translate('resources.song.fields.album', { _: 'Album' })}
                  </TableCell>
                  <TableCell>
                    {translate('resources.song.fields.artist', { _: 'Artist' })}
                  </TableCell>
                  <TableCell>
                    {translate('resources.song.fields.year', { _: 'Year' })}
                  </TableCell>
                  <TableCell>
                    {translate('resources.song.fields.explicitStatus', {
                      _: 'Explicit',
                    })}
                  </TableCell>
                  <TableCell>
                    {translate('menu.aiTool.lyrics', { _: 'Lyrics' })}
                  </TableCell>
                  <TableCell>
                    {translate('resources.song.fields.duration', { _: 'Time' })}
                  </TableCell>
                  <TableCell>
                    {translate('resources.song.fields.genre', { _: 'Genre' })}
                  </TableCell>
                  <TableCell>
                    {translate('menu.aiTool.aiGenre', { _: 'AI Genre' })}
                  </TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {availableSongs.map((song) => {
                  const alreadyAdded = addedSongIdSet.has(song.id)
                  return (
                    <TableRow
                      key={song.id}
                      hover={!alreadyAdded}
                      onClick={() => toggleSong(song.id)}
                    >
                      <TableCell padding="checkbox">
                        <Checkbox
                          inputProps={{
                            'aria-label': `Select ${song.title || 'song'}`,
                          }}
                          checked={
                            !alreadyAdded && selectedSongIds.includes(song.id)
                          }
                          disabled={alreadyAdded}
                        />
                      </TableCell>
                      <TableCell>
                        {song.title || ''}
                        {alreadyAdded ? ' (Already added)' : ''}
                      </TableCell>
                      <TableCell>{song.album || ''}</TableCell>
                      <TableCell>{song.artist || ''}</TableCell>
                      <TableCell>{song.year || ''}</TableCell>
                      <TableCell>
                        {formatExplicitStatus(song.explicitStatus)}
                      </TableCell>
                      <TableCell>{renderLyricsState(song)}</TableCell>
                      <TableCell>{formatDuration(song.duration)}</TableCell>
                      <TableCell>{song.genre || ''}</TableCell>
                      <TableCell>{song.aiGenre || '-'}</TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSongDialogOpen(false)}>
            {translate('ra.action.cancel', { _: 'Cancel' })}
          </Button>
          <Button
            color="primary"
            variant="contained"
            onClick={addSelectedSongs}
            disabled={selectedSongs.length === 0}
          >
            {translate('menu.aiTool.addSelected', { _: 'Add selected songs' })}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={Boolean(explicitReasonSong)}
        onClose={() => setExplicitReasonSong(null)}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>
          Why marked {formatExplicitStatus(explicitReasonSong?.explicitStatus)}
        </DialogTitle>
        <DialogContent>
          <Typography variant="subtitle1">
            {explicitReasonSong?.title || 'Song'}
          </Typography>
          <Typography variant="body2" paragraph>
            {explicitReasonSong?.explicitReason ||
              'No classification reason is stored. Run Classify Explicit again to generate a reason.'}
          </Typography>
          <Typography variant="body2">
            Confidence:{' '}
            {explicitReasonSong?.explicitConfidence
              ? `${explicitReasonSong.explicitConfidence}%`
              : 'Not available'}
          </Typography>
          {explicitReasonSong?.explicitProvider ? (
            <Typography variant="body2">
              Provider: {aiProviderLabel(explicitReasonSong.explicitProvider)}
            </Typography>
          ) : null}
          {explicitReasonSong?.explicitBasis ? (
            <Typography variant="body2">
              Basis: {explicitReasonSong.explicitBasis}
            </Typography>
          ) : null}
          {explicitReasonSong?.explicitEvidence?.length ? (
            <Box mt={2}>
              <Typography variant="subtitle2">
                Verified lyric evidence
              </Typography>
              {explicitReasonSong.explicitEvidence.map((evidence, index) => (
                <Typography variant="body2" key={`${evidence}-${index}`}>
                  “{evidence}”
                </Typography>
              ))}
            </Box>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setExplicitReasonSong(null)}>Close</Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={Boolean(genreTraceDetail)}
        onClose={() => setGenreTraceDetail(null)}
        fullWidth
        maxWidth="md"
        aria-labelledby="genre-developer-trace-title"
      >
        {genreTraceDetail ? (
          <>
            <DialogTitle id="genre-developer-trace-title">
              {genreTraceSourceLabel(genreTraceDetail.source)} Genre Developer
              Trace
            </DialogTitle>
            <DialogContent dividers>
              <Typography variant="subtitle1" paragraph>
                {genreTraceDetail.song?.title || 'Song'}
                {genreTraceDetail.song?.artist
                  ? ` · ${genreTraceDetail.song.artist}`
                  : ''}
              </Typography>
              {renderGenreDeveloperTrace(genreTraceDetail)}
            </DialogContent>
            <DialogActions>
              <Button onClick={() => setGenreTraceDetail(null)}>Close</Button>
            </DialogActions>
          </>
        ) : null}
      </Dialog>

      <Dialog
        open={Boolean(tokenUsageDetail)}
        onClose={() => setTokenUsageDetail(null)}
        fullWidth
        maxWidth="sm"
        aria-labelledby="token-usage-breakdown-title"
      >
        {tokenUsageDetail ? (
          <>
            <DialogTitle id="token-usage-breakdown-title">
              Token Usage Breakdown
            </DialogTitle>
            <DialogContent dividers>
              <Typography variant="subtitle1" paragraph>
                {tokenUsageDetail.song?.title || 'Song'}
                {tokenUsageDetail.song?.artist
                  ? ` · ${tokenUsageDetail.song.artist}`
                  : ''}
              </Typography>
              {renderTokenUsageBreakdown(tokenUsageDetail)}
            </DialogContent>
            <DialogActions>
              <Button onClick={() => setTokenUsageDetail(null)}>Close</Button>
            </DialogActions>
          </>
        ) : null}
      </Dialog>

      <Dialog
        open={Boolean(confidenceDetail)}
        onClose={() => setConfidenceDetail(null)}
        fullWidth
        maxWidth="sm"
      >
        {confidenceDetail ? (
          <>
            <DialogTitle>
              How the {confidenceFieldLabel(confidenceDetail.field)} confidence
              was calculated
            </DialogTitle>
            <DialogContent>
              {renderConfidenceBreakdown(confidenceDetail)}
            </DialogContent>
            <DialogActions>
              <Button onClick={() => setConfidenceDetail(null)}>Close</Button>
            </DialogActions>
          </>
        ) : null}
      </Dialog>

      <Dialog
        open={explicitRulesOpen}
        onClose={() => setExplicitRulesOpen(false)}
        fullWidth
        maxWidth="md"
      >
        <DialogTitle>Explicit word rules</DialogTitle>
        <DialogContent>
          <Typography variant="body2" paragraph>
            DeepSeek reviews these words in the context of the complete saved
            lyrics. Included words focus its review; excluded or mild words do
            not mark a song explicit by themselves. Separate entries with commas
            or new lines.
          </Typography>
          <TextField
            fullWidth
            multiline
            minRows={5}
            margin="normal"
            variant="outlined"
            label="Words categorised as explicit"
            value={explicitIncludedWords}
            inputProps={{ 'aria-label': 'Words categorised as explicit' }}
            onChange={(event) => setExplicitIncludedWords(event.target.value)}
          />
          <TextField
            fullWidth
            multiline
            minRows={5}
            margin="normal"
            variant="outlined"
            label="Words excluded from explicit categorisation"
            value={explicitExcludedWords}
            inputProps={{
              'aria-label': 'Words excluded from explicit categorisation',
            }}
            onChange={(event) => setExplicitExcludedWords(event.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={resetExplicitWordRules}>Reset defaults</Button>
          <Button onClick={() => setExplicitRulesOpen(false)}>Cancel</Button>
          <Button
            color="primary"
            variant="contained"
            onClick={saveExplicitWordRules}
          >
            Save rules
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
            {lyricsText ||
              translate('menu.aiTool.noLyrics', {
                _: 'No saved lyrics found.',
              })}
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
