import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { alpha, makeStyles } from '@material-ui/core/styles'
import {
  Button,
  ButtonBase,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Drawer,
  List,
  ListItem,
  ListItemText,
  Slider,
  TextField,
  Typography,
} from '@material-ui/core'
import Tooltip from '@material-ui/core/Tooltip'
import { Title } from 'react-admin'
import LinkIcon from '@material-ui/icons/Link'
import LinkOffIcon from '@material-ui/icons/LinkOff'
import SignalWifi4BarIcon from '@material-ui/icons/SignalWifi4Bar'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import VolumeUpIcon from '@material-ui/icons/VolumeUp'
import DescriptionIcon from '@material-ui/icons/Description'
import CachedIcon from '@material-ui/icons/Cached'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import ArrowBackIcon from '@material-ui/icons/ArrowBack'
import StopIcon from '@material-ui/icons/Stop'
import { useHistory, useParams } from 'react-router-dom'
import { BiDislike } from 'react-icons/bi'
import { MdSkipNext } from 'react-icons/md'
import useRetailPlayerDeviceStatus from './useRetailPlayerDeviceStatus'
import { normalizeValue } from './deviceUtils'
import httpClient from '../dataProvider/httpClient'
import config from '../config'
import {
  isDeviceLocked,
  isDeviceUnlockedForSession,
  markDeviceUnlockedForSession,
} from './deviceLockState'

const combineClasses = (...classNames) => classNames.filter(Boolean).join(' ')

const clamp = (value, min, max) => Math.min(Math.max(value, min), max)

const CueIcon = (props) => (
  <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" width="1em" height="1em" {...props}>
    <path d="M12 2.75a.75.75 0 0 1 .75.75v9.19l2.72-2.72a.75.75 0 1 1 1.06 1.06l-4 4a.75.75 0 0 1-1.06 0l-4-4a.75.75 0 0 1 1.06-1.06l2.72 2.72V3.5a.75.75 0 0 1 .75-.75Z" />
    <path d="M4 15.5a8 8 0 0 0 16 0h-1.5a6.5 6.5 0 0 1-13 0Z" />
  </svg>
)

const formatTime = (date, timeZone) => {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
    return '--:--'
  }

  try {
    return new Intl.DateTimeFormat([], {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
      ...(timeZone ? { timeZone } : {}),
    })
      .format(date)
      .replace(/^24:/, '00:')
  } catch (err) {
    return date
      .toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      })
      .replace(/^24:/, '00:')
  }
}

const formatDetailedTime = (date, timeZone) => {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
    return ''
  }

  try {
    const formatter = new Intl.DateTimeFormat('en-CA', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: true,
      ...(timeZone ? { timeZone } : {}),
    })

    const parts = formatter.formatToParts(date)
    const values = parts.reduce((accumulator, part) => {
      if (part.type) {
        accumulator[part.type] = part.value
      }
      return accumulator
    }, {})

    const year = values.year || ''
    const month = values.month || ''
    const day = values.day || ''
    const hour = values.hour || ''
    const minute = values.minute || ''
    const rawDayPeriod = values.dayPeriod || ''
    const normalizedDayPeriod = rawDayPeriod
      .replace(/\./g, '')
      .replace(/\s+/g, '')
      .toUpperCase()

    if (!year || !month || !day || !hour || !minute || !normalizedDayPeriod) {
      return ''
    }

    return `${year}-${month}-${day} ${hour}:${minute}${normalizedDayPeriod}`
  } catch (error) {
    const isoString = date.toISOString()
    const [isoDate, isoTime = ''] = isoString.split('T')
    const [hours = '', minutes = ''] = isoTime.split(':')
    if (!isoDate || !hours || !minutes) {
      return ''
    }

    return `${isoDate} ${hours}:${minutes}`
  }
}

const useStyles = makeStyles((theme) => {
  const headerHeight = 60
  const cueDrawerWidth = '25vw'
  const successMain =
    (theme.palette.success && theme.palette.success.main) ||
    (theme.palette.secondary && theme.palette.secondary.main) ||
    theme.palette.primary.main
  const successContrast =
    (theme.palette.success && theme.palette.success.contrastText) ||
    theme.palette.getContrastText(successMain)
  const dangerMain =
    (theme.palette.error && theme.palette.error.main) ||
    (theme.palette.secondary && theme.palette.secondary.main) ||
    theme.palette.primary.main
  const sliderMain =
    (theme.palette.secondary && theme.palette.secondary.main) || theme.palette.primary.main
  const accentColor =
    (theme.palette.secondary && theme.palette.secondary.main) || '#ff6f9f'
  const cueAccentColor = '#9c27b0'
  const disabledBackground =
    (theme.palette.action && theme.palette.action.disabledBackground) ||
    theme.palette.background.paper

  const headerOffset = theme.spacing(6)

  return {
    root: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(3.5),
      padding: `${theme.spacing(1)}px ${theme.spacing(4)}px`,
      paddingTop: headerOffset,
      width: '100%',
      maxWidth: '50vw',
      margin: '0 auto',
      boxSizing: 'border-box',
      minHeight: '100vh',
      alignItems: 'center',
      [theme.breakpoints.down('md')]: {
        padding: `${theme.spacing(4)}px ${theme.spacing(3)}px`,
        gap: theme.spacing(3),
        maxWidth: '100%',
      },
      [theme.breakpoints.down('sm')]: {
        padding: `${theme.spacing(3)}px ${theme.spacing(2.5)}px`,
        gap: theme.spacing(2.5),
      },
    },
    headerBar: {
      position: 'fixed',
      top: 0,
      left: 0,
      right: 0,
      height: headerHeight,
      display: 'flex',
      alignItems: 'center',
      padding: `0 ${theme.spacing(4)}px`,
      backgroundColor:
        alpha(theme.palette.background.paper || theme.palette.background.default, 0.92) ||
        theme.palette.background.default,
      boxShadow: '0 6px 16px rgba(0, 0, 0, 0.35)',
      zIndex: (theme.zIndex && theme.zIndex.appBar) || 1100,
      backdropFilter: 'blur(6px)',
      boxSizing: 'border-box',
      [theme.breakpoints.down('sm')]: {
        padding: `0 ${theme.spacing(2.5)}px`,
      },
    },
    headerBackButton: {
      width: 44,
      height: 44,
      borderRadius: '50%',
      border: `2px solid ${alpha(theme.palette.common.white, 0.85)}`,
      color: theme.palette.common.white,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      transition: theme.transitions.create(['color', 'border-color', 'background-color'], {
        duration: theme.transitions.duration.shorter,
        easing: theme.transitions.easing.easeInOut,
      }),
      padding: 0,
      cursor: 'pointer',
      '&:hover, &:focus-visible': {
        color: accentColor,
        borderColor: accentColor,
        backgroundColor: alpha(accentColor, 0.12),
      },
      [theme.breakpoints.down('xs')]: {
        width: 40,
        height: 40,
      },
    },
    headerBackIcon: {
      fontSize: theme.typography.pxToRem(24),
    },
    headerCenter: {
      flex: 1,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      textAlign: 'center',
      padding: `0 ${theme.spacing(2)}px`,
      minWidth: 0,
    },
    headerTitle: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(24),
      letterSpacing: theme.spacing(0.25),
      color: alpha(theme.palette.common.white, 0.9),
      textTransform: 'none',
      maxWidth: '100%',
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(20),
      },
      [theme.breakpoints.down('xs')]: {
        fontSize: theme.typography.pxToRem(18),
      },
    },
    headerStatusGroup: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(1.5),
    },
    headerStatusIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: theme.spacing(0.5),
      borderRadius: theme.shape.borderRadius,
      color: alpha(theme.palette.common.white, 0.8),
      transition: theme.transitions.create(['color', 'transform'], {
        duration: theme.transitions.duration.shorter,
        easing: theme.transitions.easing.easeInOut,
      }),
      flexShrink: 0,
      '& > svg': {
        fontSize: theme.typography.pxToRem(26),
      },
      '&:focus-visible': {
        outline: `2px solid ${alpha(accentColor, 0.85)}`,
        outlineOffset: 2,
        transform: 'scale(1.02)',
      },
    },
    headerStatusIconOnline: {
      color: successMain,
    },
    headerStatusIconOffline: {
      color: dangerMain,
    },
    headerClock: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: `${theme.spacing(0.5)}px ${theme.spacing(2)}px`,
      borderRadius: theme.shape.borderRadius * 2.5,
      border: `2px solid ${alpha(theme.palette.common.white, 0.85)}`,
      color: theme.palette.common.white,
      fontWeight: theme.typography.fontWeightMedium,
      fontSize: theme.typography.pxToRem(18),
      letterSpacing: theme.spacing(0.25),
      transition: theme.transitions.create(['color', 'border-color', 'background-color'], {
        duration: theme.transitions.duration.shorter,
        easing: theme.transitions.easing.easeInOut,
      }),
      backgroundColor: alpha(theme.palette.common.white, 0.04),
      '&:hover, &:focus-visible': {
        color: accentColor,
        borderColor: accentColor,
        backgroundColor: alpha(accentColor, 0.12),
      },
      [theme.breakpoints.down('xs')]: {
        fontSize: theme.typography.pxToRem(16),
        padding: `${theme.spacing(0.25)}px ${theme.spacing(1.5)}px`,
      },
    },
    header: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: theme.spacing(2),
      flexWrap: 'wrap',
      width: '100%',
      maxWidth: 960,
      textAlign: 'center',
      paddingLeft: 0,
    },
    title: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(52),
      letterSpacing: '-0.01em',
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(40),
      },
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(28),
      },
    },
    statusGroup: {
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(2),
      flexWrap: 'wrap',
      justifyContent: 'center',
    },
    statusIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: theme.typography.pxToRem(32),
      '& svg': {
        fontSize: 'inherit',
      },
    },
    statusIconSuccess: {
      color: successMain,
    },
    statusIconDanger: {
      color: dangerMain,
    },
    statusIconNeutral: {
      color: theme.palette.text.secondary,
    },
    timePill: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: `${theme.spacing(0.5)}px ${theme.spacing(2)}px`,
      borderRadius: theme.shape.borderRadius * 2,
      backgroundColor: successMain,
      color: successContrast,
      fontWeight: theme.typography.fontWeightMedium,
      fontSize: theme.typography.pxToRem(18),
    },
    list: {
      borderRadius: theme.shape.borderRadius * 1.5,
      backgroundColor: theme.palette.background.paper,
      overflow: 'hidden',
      border: `1px solid ${theme.palette.divider}`,
      width: '100%',
      maxWidth: 960,
      boxShadow: '0 22px 45px rgba(0, 0, 0, 0.28)',
      transition: theme.transitions.create(['box-shadow'], {
        duration: theme.transitions.duration.shorter,
      }),
      '&:hover, &:focus-within': {
        boxShadow: '0 30px 60px rgba(0, 0, 0, 0.36)',
      },
    },
    mainContent: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(3),
      width: '100%',
      maxWidth: 960,
      margin: '0 auto',
      alignItems: 'center',
      [theme.breakpoints.up('md')]: {
        marginTop: theme.spacing(2),
      },
    },
    nowPlayingCard: {
      borderRadius: theme.shape.borderRadius * 1.5,
      backgroundColor: alpha(theme.palette.background.paper, 0.3),
      border: `1px solid ${theme.palette.divider}`,
      padding: theme.spacing(3),
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: theme.spacing(3),
      width: '100%',
      maxWidth: 960,
      boxShadow: '0 26px 55px rgba(0, 0, 0, 0.32)',
      transition: theme.transitions.create(['box-shadow', 'transform'], {
        duration: theme.transitions.duration.shorter,
        easing: theme.transitions.easing.easeInOut,
      }),
      '&:hover, &:focus-within': {
        boxShadow: '0 36px 70px rgba(0, 0, 0, 0.38)',
        transform: 'translateY(-2px)',
      },
      [theme.breakpoints.down('sm')]: {
        padding: theme.spacing(2.5),
      },
    },
    nowPlayingBody: {
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'center',
      alignItems: 'center',
      textAlign: 'center',
      gap: theme.spacing(3),
      flex: 1,
      minWidth: 0,
      width: '100%',
    },
    nowPlayingHeader: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: theme.spacing(1),
      width: '100%',
      textAlign: 'center',
    },
    listItemButton: {
      display: 'block',
      width: '100%',
      textAlign: 'left',
      '&:hover $listItem, &:focus-visible $listItem': {
        backgroundColor: theme.palette.action.hover,
      },
      '&:last-child $listItem': {
        borderBottom: 'none',
      },
    },
    listItem: {
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(2),
      padding: `${theme.spacing(2.25)}px ${theme.spacing(3)}px`,
      borderBottom: `1px solid ${theme.palette.divider}`,
      transition: theme.transitions.create(['background-color'], {
        duration: theme.transitions.duration.shortest,
      }),
    },
    listIcon: {
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(24),
    },
    playlistLabel: {
      flex: 1,
      minWidth: 0,
    },
    playlistLabelActive: {
      color: accentColor,
    },
    playlistLabelInactive: {
      color: theme.palette.text.secondary,
    },
    playlistIconActive: {
      color: accentColor,
    },
    playlistIconInactive: {
      color: theme.palette.text.secondary,
    },
    dropdownWrapper: {
      display: 'flex',
      flexDirection: 'column',
      width: '100%',
      backgroundColor: theme.palette.background.paper,
    },
    dropdownTriggerButton: {
      borderBottom: `1px solid ${theme.palette.divider}`,
      display: 'flex',
      alignItems: 'center',
      width: '100%',
      transition: theme.transitions.create(['background-color'], {
        duration: theme.transitions.duration.shorter,
      }),
    },
    dropdownCaret: {
      marginLeft: 'auto',
      transition: theme.transitions.create(['transform'], {
        duration: theme.transitions.duration.shortest,
      }),
      color: accentColor,
    },
    dropdownCaretOpen: {
      transform: 'rotate(180deg)',
    },
    dropdownMenu: {
      display: 'grid',
      gridAutoRows: 'min-content',
      backgroundColor: theme.palette.background.paper,
      transition: theme.transitions.create(['max-height', 'opacity'], {
        duration: theme.transitions.duration.short,
        easing: theme.transitions.easing.easeInOut,
      }),
      maxHeight: 0,
      opacity: 0,
      pointerEvents: 'none',
      overflow: 'hidden',
      boxShadow: '0 20px 40px rgba(0, 0, 0, 0.3)',
      borderTop: `1px solid ${theme.palette.divider}`,
      justifyItems: 'center',
    },
    dropdownMenuOpen: {
      maxHeight: 320,
      opacity: 1,
      pointerEvents: 'auto',
      overflowY: 'auto',
    },
    dropdownOptionButton: {
      textAlign: 'center',
      '&:last-child $listItem': {
        borderBottom: 'none',
      },
    },
    dropdownOptionContent: {
      justifyContent: 'center',
      textAlign: 'center',
    },
    listText: {
      fontSize: theme.typography.pxToRem(18),
      fontWeight: theme.typography.fontWeightMedium,
      letterSpacing: 0.2,
    },
    dropdownOptionLabel: {
      textAlign: 'center',
    },
    artworkWrapper: {
      width: 'clamp(120px, 20vw, 180px)',
      maxWidth: '100%',
      flexShrink: 0,
      alignSelf: 'center',
      [theme.breakpoints.down('md')]: {
        width: 'clamp(120px, 32vw, 200px)',
      },
      [theme.breakpoints.down('sm')]: {
        width: 'min(160px, 70%)',
      },
    },
    artworkCircle: {
      position: 'relative',
      width: '100%',
      paddingTop: '100%',
      borderRadius: '50%',
      overflow: 'hidden',
      backgroundColor:
        (theme.palette.action && theme.palette.action.disabledBackground) ||
        theme.palette.action.hover,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      boxShadow: 'inset 0 0 0 2px rgba(255, 255, 255, 0.04)',
    },
    artworkContent: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    artworkImage: {
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      borderRadius: '50%',
      display: 'block',
    },
    discSvg: {
      width: '80%',
      height: '80%',
      maxWidth: 180,
      maxHeight: 180,
    },
    discOuter: {
      fill:
        (theme.palette.action && theme.palette.action.disabled) ||
        theme.palette.grey[400],
    },
    discInner: {
      fill:
        (theme.palette.background && theme.palette.background.paper) ||
        theme.palette.common.white,
    },
    discHighlight: {
      fill:
        (theme.palette.primary && theme.palette.primary.main) ||
        theme.palette.text.primary,
      opacity: 0.2,
    },
    nowPlayingTitle: {
      fontSize: theme.typography.pxToRem(32),
      fontWeight: 600,
      textAlign: 'center',
      width: '100%',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(28),
      },
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(22),
      },
    },
    nowPlayingArtist: {
      fontSize: theme.typography.pxToRem(18),
      textAlign: 'center',
      color: theme.palette.text.secondary,
      width: '100%',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      letterSpacing: 0.2,
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(15),
      },
    },
    nowPlayingFooter: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: theme.spacing(2.5),
      flexWrap: 'wrap',
      width: '100%',
    },
    controlsRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: theme.spacing(3),
      flexWrap: 'wrap',
    },
    controlsRowDisabled: {
      '& $controlButton': {
        color: theme.palette.text.disabled,
      },
    },
    controlButton: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 52,
      height: 52,
      borderRadius: '50%',
      transition: theme.transitions.create(['background-color', 'color'], {
        duration: theme.transitions.duration.shortest,
      }),
      color: theme.palette.text.primary,
      '&.Mui-disabled': {
        color: theme.palette.text.disabled,
      },
      '&:hover, &:focus-visible': {
        backgroundColor: theme.palette.action.hover,
      },
    },
    controlButtonCue: {
      '&:hover $controlIconCue, &:focus-visible $controlIconCue': {
        color: accentColor,
      },
    },
    controlButtonMuted: {
      color: dangerMain,
    },
    controlIcon: {
      fontSize: theme.typography.pxToRem(28),
      display: 'inline-flex',
    },
    controlIconCue: {
      transition: theme.transitions.create(['color'], {
        duration: theme.transitions.duration.shortest,
      }),
    },
    volumeSection: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(1.5),
      width: '100%',
      maxWidth: 360,
      minWidth: 0,
      alignSelf: 'center',
      margin: '0 auto',
      position: 'relative',
      [theme.breakpoints.down('md')]: {
        maxWidth: 420,
      },
      [theme.breakpoints.down('sm')]: {
        width: '100%',
        minWidth: 'auto',
      },
      '&:hover $volumeLabelRow, &:focus-within $volumeLabelRow': {
        opacity: 1,
        transform: 'translateY(0)',
      },
    },
    volumeSectionDisabled: {
      '& $volumeLabelRow': {
        opacity: 1,
        transform: 'translateY(0)',
        color: theme.palette.text.disabled,
      },
    },
    volumeLabelRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      color: theme.palette.text.secondary,
      textTransform: 'lowercase',
      opacity: 0,
      pointerEvents: 'none',
      transform: 'translateY(-6px)',
      transition: theme.transitions.create(['opacity', 'transform'], {
        duration: theme.transitions.duration.shortest,
      }),
    },
    slider: {
      color: sliderMain,
      width: '100%',
      margin: '0 auto',
    },
    sliderTrack: {
      backgroundColor: sliderMain,
    },
    sliderThumb: {
      backgroundColor: sliderMain,
    },
    sliderRail: {
      backgroundColor: theme.palette.action.disabled,
    },
    sliderDisabled: {
      color: theme.palette.action.disabled,
      '& $sliderTrack': {
        backgroundColor: theme.palette.action.disabled,
      },
      '& $sliderThumb': {
        backgroundColor: theme.palette.action.disabled,
      },
    },
    volumeValue: {
      minWidth: 32,
      textAlign: 'right',
      fontVariantNumeric: 'tabular-nums',
      fontWeight: theme.typography.fontWeightMedium,
    },
    dislikeMessage: {
      marginTop: theme.spacing(1),
      textAlign: 'center',
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(14),
      width: '100%',
    },
    notFoundWrapper: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(2),
      padding: theme.spacing(5),
      maxWidth: 720,
      width: '100%',
      margin: '0 auto',
      [theme.breakpoints.down('sm')]: {
        padding: theme.spacing(3),
      },
    },
    notFoundTitle: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(36),
    },
    notFoundMessage: {
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(18),
    },
    refreshButton: {
      borderRadius: theme.shape.borderRadius * 2,
      padding: theme.spacing(0.75),
      border: `1px solid ${theme.palette.divider}`,
      '&:hover, &:focus-visible': {
        backgroundColor: theme.palette.action.hover,
      },
    },
    cueDrawer: {
      width: cueDrawerWidth,
      maxWidth: cueDrawerWidth,
      minWidth: 240,
      flexShrink: 0,
    },
    cueDrawerOpen: {
      paddingRight: cueDrawerWidth,
      transition: theme.transitions.create('padding-right', {
        duration: theme.transitions.duration.standard,
        easing: theme.transitions.easing.easeInOut,
      }),
      [theme.breakpoints.down('sm')]: {
        paddingRight: 0,
      },
    },
    cueDrawerPaper: {
      width: cueDrawerWidth,
      maxWidth: cueDrawerWidth,
      minWidth: 240,
      boxSizing: 'border-box',
      padding: theme.spacing(2.5),
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(2),
    },
    cueDrawerHeader: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: theme.spacing(1.5),
    },
    cueDrawerTitle: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(20),
    },
    cueControls: {
      display: 'flex',
      justifyContent: 'flex-end',
    },
    cueStopButton: {
      borderRadius: theme.shape.borderRadius,
      padding: `${theme.spacing(1)}px ${theme.spacing(1.5)}px`,
      border: `1px solid ${theme.palette.divider}`,
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(1),
      color: theme.palette.text.primary,
      transition: theme.transitions.create(['color', 'background-color', 'border-color'], {
        duration: theme.transitions.duration.shorter,
        easing: theme.transitions.easing.easeInOut,
      }),
      '&:hover, &:focus-visible': {
        backgroundColor: theme.palette.action.hover,
      },
    },
    cueStopButtonActive: {
      borderColor: cueAccentColor,
      color: cueAccentColor,
      backgroundColor: alpha(cueAccentColor, 0.12),
      '&:hover, &:focus-visible': {
        backgroundColor: alpha(cueAccentColor, 0.2),
      },
    },
    cueStopIcon: {
      fontSize: theme.typography.pxToRem(22),
    },
    cueStopLabel: {
      fontWeight: theme.typography.fontWeightMedium,
      textTransform: 'uppercase',
      letterSpacing: 0.5,
      fontSize: theme.typography.pxToRem(12),
    },
    cueList: {
      flex: 1,
      overflowY: 'auto',
    },
    cueListItem: {
      border: `1px solid ${theme.palette.divider}`,
      borderRadius: theme.shape.borderRadius,
      marginBottom: theme.spacing(1),
    },
    cueListItemActive: {
      borderColor: cueAccentColor,
      backgroundColor: alpha(cueAccentColor, 0.08),
      boxShadow: `0 0 0 1px ${alpha(cueAccentColor, 0.2)}`,
    },
    cueListPrimary: {
      fontWeight: theme.typography.fontWeightMedium,
    },
    cueListPrimaryActive: {
      color: cueAccentColor,
    },
    cueListSecondary: {
      color: theme.palette.text.secondary,
    },
    cueListSecondaryActive: {
      color: alpha(cueAccentColor, 0.85),
    },
    cueError: {
      color: theme.palette.error.main,
    },
    cueEmpty: {
      color: theme.palette.text.secondary,
      fontStyle: 'italic',
    },
    nowPlayingTitleCue: {
      color: cueAccentColor,
    },
    controlButtonCueActive: {
      color: cueAccentColor,
      backgroundColor: alpha(cueAccentColor, 0.12),
      '&:hover, &:focus-visible': {
        color: cueAccentColor,
        backgroundColor: alpha(cueAccentColor, 0.2),
      },
      '&:hover $controlIconCue, &:focus-visible $controlIconCue': {
        color: cueAccentColor,
      },
    },
    controlIconCueActive: {
      color: cueAccentColor,
    },
    lockDialogPaper: {
      minWidth: 360,
      maxWidth: '90vw',
    },
    lockDialogError: {
      color: theme.palette.error.main,
      marginTop: theme.spacing(1),
    },
  }
})

const RetailPlayerDashboard = () => {
  const classes = useStyles()
  const history = useHistory()
  const { deviceSlug } = useParams()
  const {
    device: resolvedDevice,
    baseDevice,
    error: integrationError,
    statusError,
    devicesError,
    isLoading: retailLoading,
    isStatusLoading: statusLoading,
    refresh: refreshStatus,
    notFound,
    isApiEnabled,
    buttonTriggers,
    hasButtonTriggers,
    isTriggerListLoading,
    sendRemoteControlCommand,
  } = useRetailPlayerDeviceStatus(deviceSlug)
  const device = resolvedDevice || null
  const [deviceTime, setDeviceTime] = useState(() => new Date())
  const [isMuted, setIsMuted] = useState(false)
  const [volume, setVolume] = useState(null)
  const [displayVolume, setDisplayVolume] = useState(null)
  const volumeTimeoutRef = useRef(null)
  const previousVolumeRef = useRef(null)
  const volumeSyncReadyRef = useRef(false)
  const lastTrackSignatureRef = useRef('')
  const dislikeTimeoutRef = useRef(null)
  const dislikeDisableTimeoutRef = useRef(null)
  const dislikeRequestControllerRef = useRef(null)
  const dislikeRetryTimeoutRef = useRef(null)
  const latestNowPlayingRef = useRef(null)
  const latestScheduleLabelRef = useRef('')
  const channelRequestControllerRef = useRef(null)
  const [showDislikeMessage, setShowDislikeMessage] = useState(false)
  const [isDislikeDisabled, setIsDislikeDisabled] = useState(false)
  const [previousNowPlaying, setPreviousNowPlaying] = useState(null)
  const [currentTrackIndex, setCurrentTrackIndex] = useState(0)
  const [isScheduleMenuOpen, setScheduleMenuOpen] = useState(false)
  const [isCueDrawerOpen, setCueDrawerOpen] = useState(false)
  const [cueError, setCueError] = useState(null)
  const [activeCueTriggerId, setActiveCueTriggerId] = useState('')
  const [activeCueTriggerOrdinal, setActiveCueTriggerOrdinal] = useState(null)
  const [lockPasswordInput, setLockPasswordInput] = useState('')
  const [lockError, setLockError] = useState('')
  const scheduleDropdownRef = useRef(null)
  const isBusy = retailLoading || statusLoading
  const combinedError = integrationError || statusError || devicesError

  useEffect(() => {
    const metaDescription = document.querySelector('meta[name="description"]')
    if (!metaDescription) {
      return undefined
    }

    const defaultDescription =
      metaDescription.getAttribute('content') || 'MusicMatters Music Server'
    const deviceName = normalizeValue(device?.name)
    const organization = normalizeValue(device?.organization)

    if (!deviceName) {
      metaDescription.setAttribute('content', defaultDescription)
      return () => {
        metaDescription.setAttribute('content', defaultDescription)
      }
    }

    const organizationText = organization || 'Retail Player'
    metaDescription.setAttribute(
      'content',
      `MusicMatters
Device: ${deviceName} • Property: ${organizationText}`,
    )

    return () => {
      metaDescription.setAttribute('content', defaultDescription)
    }
  }, [device?.name, device?.organization])

  const deviceTrackKey = useMemo(() => {
    if (!device) {
      return ''
    }

    return (
      normalizeValue(device.apiId) ||
      normalizeValue(device.id) ||
      normalizeValue(device.slug) ||
      normalizeValue(device.macAddress)
    )
  }, [device])

  useEffect(() => {
    setPreviousNowPlaying(null)
    lastTrackSignatureRef.current = ''
    setIsDislikeDisabled(false)
    if (dislikeDisableTimeoutRef.current) {
      window.clearTimeout(dislikeDisableTimeoutRef.current)
      dislikeDisableTimeoutRef.current = null
    }
  }, [deviceTrackKey])

  const cueStorageKey = useMemo(() => {
    if (!deviceTrackKey) {
      return null
    }

    return `retailPlayerCue:${deviceTrackKey}`
  }, [deviceTrackKey])

  useEffect(() => {
    if (!cueStorageKey) {
      setActiveCueTriggerId('')
      setActiveCueTriggerOrdinal(null)
      return
    }

    try {
      const storedValue = window.localStorage.getItem(cueStorageKey)
      if (!storedValue) {
        setActiveCueTriggerId('')
        setActiveCueTriggerOrdinal(null)
        return
      }

      const parsed = JSON.parse(storedValue)
      const storedTriggerId = normalizeValue(parsed?.triggerId) || ''
      const parsedOrdinal = Number(parsed?.triggerOrdinal)
      const storedTriggerOrdinal = Number.isFinite(parsedOrdinal) ? parsedOrdinal : null

      setActiveCueTriggerId(storedTriggerId)
      setActiveCueTriggerOrdinal(storedTriggerOrdinal)
    } catch (error) {
      setActiveCueTriggerId('')
      setActiveCueTriggerOrdinal(null)
    }
  }, [cueStorageKey])

  const persistActiveCueState = useCallback(
    (triggerId, triggerOrdinal) => {
      setActiveCueTriggerId(triggerId)
      setActiveCueTriggerOrdinal(triggerOrdinal)

      if (!cueStorageKey) {
        return
      }

      try {
        if (triggerId || Number.isFinite(triggerOrdinal)) {
          window.localStorage.setItem(
            cueStorageKey,
            JSON.stringify({ triggerId, triggerOrdinal }),
          )
        } else {
          window.localStorage.removeItem(cueStorageKey)
        }
      } catch (error) {
        // Ignore storage errors
      }
    },
    [cueStorageKey],
  )

  useEffect(() => {
    if (!isApiEnabled) {
      return undefined
    }

    const intervalId = window.setInterval(() => {
      refreshStatus()
    }, 3000)

    return () => {
      window.clearInterval(intervalId)
    }
  }, [isApiEnabled, refreshStatus])

  const deviceApiId = useMemo(() => {
    if (resolvedDevice?.apiId) {
      return resolvedDevice.apiId
    }
    if (resolvedDevice?.id) {
      return resolvedDevice.id
    }
    if (baseDevice?.apiId) {
      return baseDevice.apiId
    }
    if (baseDevice?.id) {
      return baseDevice.id
    }
    if (device?.apiId) {
      return device.apiId
    }
    if (device?.id) {
      return device.id
    }
    return ''
  }, [
    baseDevice?.apiId,
    baseDevice?.id,
    device?.apiId,
    device?.id,
    resolvedDevice?.apiId,
    resolvedDevice?.id,
  ])

  const canControlDevice = useMemo(
    () => Boolean(isApiEnabled && deviceApiId),
    [deviceApiId, isApiEnabled],
  )

  const resolveDeviceTime = useCallback((sourceDevice) => {
    if (!sourceDevice) {
      return new Date()
    }

    const directLocalTime =
      typeof sourceDevice.localTime === 'string' ? sourceDevice.localTime : null
    if (directLocalTime) {
      const parsedDirect = new Date(directLocalTime)
      if (!Number.isNaN(parsedDirect.getTime())) {
        return parsedDirect
      }
    }

    const status =
      sourceDevice.status && typeof sourceDevice.status === 'object'
        ? sourceDevice.status
        : {}

    const localTimeValue =
      typeof status.writeDate === 'string'
        ? status.writeDate
        : typeof status.localTime === 'string'
          ? status.localTime
          : null
    if (localTimeValue) {
      const parsedLocal = new Date(localTimeValue)
      if (!Number.isNaN(parsedLocal.getTime())) {
        return parsedLocal
      }
    }

    const systemTimeValue =
      typeof status.writeDate === 'string'
        ? status.writeDate
        : typeof status.localTime === 'string'
          ? status.localTime
          : null
    if (systemTimeValue) {
      const parsedSystem = new Date(systemTimeValue)
      if (!Number.isNaN(parsedSystem.getTime())) {
        return parsedSystem
      }
    }

    return new Date()
  }, [])

  useEffect(() => {
    setDeviceTime(resolveDeviceTime(resolvedDevice))
  }, [resolvedDevice, resolveDeviceTime])

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setDeviceTime((previous) => {
        if (!(previous instanceof Date) || Number.isNaN(previous.getTime())) {
          return new Date()
        }
        return new Date(previous.getTime() + 1000)
      })
    }, 1000)

    return () => window.clearInterval(intervalId)
  }, [])

  const schedules = useMemo(() => device?.schedules || [], [device])

  const activeChannelKey = useMemo(() => {
    const activeSchedule = schedules.find((schedule) => schedule.isActive)
    return activeSchedule ? activeSchedule.key : null
  }, [schedules])

  const [pendingActiveChannelKey, setPendingActiveChannelKey] = useState(null)

  useEffect(() => {
    if (pendingActiveChannelKey && activeChannelKey === pendingActiveChannelKey) {
      setPendingActiveChannelKey(null)
    }
  }, [activeChannelKey, pendingActiveChannelKey])

  const effectiveActiveChannelKey = pendingActiveChannelKey || activeChannelKey

  const availableSchedules = useMemo(
    () => schedules.filter((schedule) => schedule.key !== effectiveActiveChannelKey),
    [effectiveActiveChannelKey, schedules],
  )
  const availableSchedulesCount = availableSchedules.length

  const cueTriggers = useMemo(
    () => (Array.isArray(buttonTriggers) ? buttonTriggers.filter(Boolean) : []),
    [buttonTriggers],
  )
  const hasCueTriggers = hasButtonTriggers || cueTriggers.length > 0
  const isCueLoading = isTriggerListLoading && !hasCueTriggers

  const getTriggerIdentifier = useCallback((trigger) => {
    if (!trigger || typeof trigger !== 'object') {
      return ''
    }

    return (
      normalizeValue(trigger.id) ||
      normalizeValue(trigger.ID) ||
      normalizeValue(trigger.name) ||
      ''
    )
  }, [])

  const getTriggerOrdinal = useCallback((trigger) => {
    if (!trigger || typeof trigger !== 'object') {
      return null
    }

    const rawOrdinal =
      trigger.ordinal ?? trigger.Ordinal ?? trigger.button ?? trigger.buttonNumber
    const parsedOrdinal = Number(rawOrdinal)
    return Number.isFinite(parsedOrdinal) ? parsedOrdinal : null
  }, [])

  const sortedCueTriggers = useMemo(() => {
    if (!cueTriggers || !cueTriggers.length) {
      return []
    }

    return [...cueTriggers].sort((a, b) => {
      const aOrdinal = typeof a?.ordinal === 'number' ? a.ordinal : 0
      const bOrdinal = typeof b?.ordinal === 'number' ? b.ordinal : 0
      return aOrdinal - bOrdinal
    })
  }, [cueTriggers])

  const nowPlayingCueMetadata = useMemo(() => {
    if (device?.nowPlaying && typeof device.nowPlaying === 'object') {
      const metadata = device.nowPlaying.metadata
      if (metadata && typeof metadata === 'object') {
        return metadata
      }
    }

    return null
  }, [device])

  const detectedCuePlayback = useMemo(() => {
    const triggerIdCandidates = nowPlayingCueMetadata
      ? [
          nowPlayingCueMetadata.triggerId,
          nowPlayingCueMetadata.trigger_id,
          nowPlayingCueMetadata.triggerID,
          nowPlayingCueMetadata.cueId,
          nowPlayingCueMetadata.cue_id,
          nowPlayingCueMetadata.cueID,
          nowPlayingCueMetadata.trigger,
          nowPlayingCueMetadata.cueTriggerId,
          nowPlayingCueMetadata.cueTrigger_id,
        ]
      : []

    const metadataTriggerId = triggerIdCandidates.map(normalizeValue).find(Boolean) || ''
    const metadataOrdinalCandidate = nowPlayingCueMetadata
      ?
          nowPlayingCueMetadata.triggerOrdinal ??
          nowPlayingCueMetadata.trigger_ordinal ??
          nowPlayingCueMetadata.ordinal ??
          nowPlayingCueMetadata.button ??
          nowPlayingCueMetadata.buttonNumber
      : null
    const parsedMetadataOrdinal = Number(metadataOrdinalCandidate)
    const resolvedMetadataOrdinal = Number.isFinite(parsedMetadataOrdinal)
      ? parsedMetadataOrdinal
      : null

    let matchedTriggerId = metadataTriggerId
    let matchedTriggerOrdinal = resolvedMetadataOrdinal

    if (!matchedTriggerId && resolvedMetadataOrdinal !== null) {
      const matchedTrigger = sortedCueTriggers.find((trigger) => {
        const triggerOrdinal = getTriggerOrdinal(trigger)
        return triggerOrdinal !== null && triggerOrdinal === resolvedMetadataOrdinal
      })

      if (matchedTrigger) {
        matchedTriggerId = getTriggerIdentifier(matchedTrigger)
        matchedTriggerOrdinal = resolvedMetadataOrdinal
      }
    }

    if (matchedTriggerId) {
      return { triggerId: matchedTriggerId, triggerOrdinal: matchedTriggerOrdinal }
    }

    return { triggerId: '', triggerOrdinal: matchedTriggerOrdinal }
  }, [getTriggerIdentifier, getTriggerOrdinal, nowPlayingCueMetadata, sortedCueTriggers])

  useEffect(() => {
    if (detectedCuePlayback.triggerId || Number.isFinite(detectedCuePlayback.triggerOrdinal)) {
      persistActiveCueState(
        detectedCuePlayback.triggerId || '',
        Number.isFinite(detectedCuePlayback.triggerOrdinal)
          ? detectedCuePlayback.triggerOrdinal
          : null,
      )
    }
  }, [detectedCuePlayback, persistActiveCueState])

  useEffect(() => {
    if (!isScheduleMenuOpen) {
      return undefined
    }

    const handleClickOutside = (event) => {
      if (
        scheduleDropdownRef.current &&
        !scheduleDropdownRef.current.contains(event.target)
      ) {
        setScheduleMenuOpen(false)
      }
    }

    const handleEscape = (event) => {
      if (event.key === 'Escape') {
        setScheduleMenuOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleEscape)

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleEscape)
    }
  }, [isScheduleMenuOpen])

  useEffect(() => {
    setScheduleMenuOpen(false)
  }, [activeChannelKey])

  useEffect(() => {
    if (availableSchedulesCount === 0) {
      setScheduleMenuOpen(false)
    }
  }, [availableSchedulesCount])

  const sendCueTriggerAction = useCallback(
    (trigger) => {
      const triggerId = getTriggerIdentifier(trigger)
      const triggerOrdinal = getTriggerOrdinal(trigger)

      const resolvedOrdinal = Number.isFinite(triggerOrdinal) ? triggerOrdinal : null
      if (triggerId || Number.isFinite(resolvedOrdinal)) {
        persistActiveCueState(triggerId || '', resolvedOrdinal)
      }

      if (!deviceApiId || !triggerId) {
        return
      }

      const headers = new Headers({ 'Content-Type': 'application/json' })

      httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/triggers`, {
        method: 'POST',
        headers,
        body: JSON.stringify({ action: 'PLAY', value: triggerId }),
      }).catch((err) => {
        if (err?.name !== 'AbortError') {
          setCueError(err)
        }
      })
    },
    [deviceApiId, getTriggerIdentifier, getTriggerOrdinal, persistActiveCueState],
  )

  const stopCuePlayback = useCallback(() => {
    const triggerId = normalizeValue(activeCueTriggerId)

    if (!deviceApiId) {
      persistActiveCueState('', null)
      return
    }

    const headers = new Headers({ 'Content-Type': 'application/json' })
    const stopValue = triggerId || 'ALL'

    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/triggers`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ action: 'STOP', value: stopValue }),
    }).catch((err) => {
      if (err?.name !== 'AbortError') {
        setCueError(err)
      }
    })

    persistActiveCueState('', null)
  }, [activeCueTriggerId, deviceApiId, persistActiveCueState])

  const activeSchedule = useMemo(() => {
    if (!schedules.length) {
      return null
    }
    const matched = schedules.find((schedule) => schedule.key === effectiveActiveChannelKey)
    return matched || schedules[0]
  }, [effectiveActiveChannelKey, schedules])

  const rootClassName = classes.root

  useEffect(() => {
    latestNowPlayingRef.current = device?.nowPlaying
  }, [device?.nowPlaying])

  useEffect(() => {
    latestScheduleLabelRef.current = activeSchedule?.label || ''
  }, [activeSchedule?.label])

  const resolveDislikePayload = useCallback((nowPlaying, scheduleLabel) => {
    const nowPlayingData = nowPlaying && typeof nowPlaying === 'object' ? nowPlaying : {}
    const metadata =
      nowPlayingData?.metadata && typeof nowPlayingData.metadata === 'object'
        ? nowPlayingData.metadata
        : {}

    const titleCandidates = [
      typeof nowPlayingData?.title === 'string' ? nowPlayingData.title.trim() : '',
      typeof metadata?.title === 'string' ? metadata.title.trim() : '',
    ]
    const trackTitle = titleCandidates.find((value) => value) || ''

    const playlistName = typeof scheduleLabel === 'string' ? scheduleLabel.trim() : ''

    if (!trackTitle && !playlistName) {
      return null
    }

    return { trackTitle, playlistName }
  }, [])

  const sendDislikeRequest = useCallback(
    (payload) => {
      if (!payload) {
        return
      }

      if (dislikeRequestControllerRef.current) {
        dislikeRequestControllerRef.current.abort()
      }

      const abortController = new AbortController()
      dislikeRequestControllerRef.current = abortController

      const headers = new Headers({ 'Content-Type': 'application/json' })

      httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/dislike`, {
        method: 'POST',
        headers,
        body: JSON.stringify(payload),
        signal: abortController.signal,
      })
        .catch((err) => {
          if (err?.name !== 'AbortError') {
            // eslint-disable-next-line no-console
            console.error('Failed to send dislike notification', err)
          }
        })
        .finally(() => {
          if (dislikeRequestControllerRef.current === abortController) {
            dislikeRequestControllerRef.current = null
          }
        })
    },
    [deviceApiId],
  )

  const sendDislikeNotification = useCallback(() => {
    if (!isApiEnabled || !deviceApiId) {
      return
    }

    const payload = resolveDislikePayload(device?.nowPlaying, activeSchedule?.label)
    if (!payload) {
      return
    }

    if (payload.trackTitle.toLowerCase() === 'loading') {
      if (dislikeRetryTimeoutRef.current) {
        window.clearTimeout(dislikeRetryTimeoutRef.current)
      }
      dislikeRetryTimeoutRef.current = window.setTimeout(() => {
        const retryPayload = resolveDislikePayload(
          latestNowPlayingRef.current,
          latestScheduleLabelRef.current,
        )
        if (retryPayload && retryPayload.trackTitle.toLowerCase() !== 'loading') {
          sendDislikeRequest(retryPayload)
        }
        dislikeRetryTimeoutRef.current = null
      }, 3000)
      return
    }

    sendDislikeRequest(payload)
  }, [
    activeSchedule?.label,
    device?.nowPlaying,
    deviceApiId,
    isApiEnabled,
    resolveDislikePayload,
    sendDislikeRequest,
  ])

  const dropdownLabel = activeSchedule ? activeSchedule.label : 'No playlists available'

  const deviceVolume = useMemo(() => {
    if (typeof device?.volume === 'number' && !Number.isNaN(device.volume)) {
      return device.volume
    }

    return null
  }, [device?.volume])

  useEffect(() => {
    const resolvedDeviceVolume =
      typeof deviceVolume === 'number' && !Number.isNaN(deviceVolume)
        ? deviceVolume
        : null

    setIsMuted(Boolean(device?.isMuted) || resolvedDeviceVolume === 0)

    if (resolvedDeviceVolume !== null) {
      setVolume(resolvedDeviceVolume)
      setDisplayVolume(resolvedDeviceVolume)
      if (resolvedDeviceVolume > 0) {
        previousVolumeRef.current = resolvedDeviceVolume
      }
    }
    volumeSyncReadyRef.current = false
  }, [device?.apiId, device?.id, device?.isMuted, deviceVolume])

  useEffect(() => {
    volumeSyncReadyRef.current = false
  }, [deviceApiId, isApiEnabled])

  useEffect(() => {
    if (!canControlDevice) {
      return undefined
    }

    if (typeof volume !== 'number' || Number.isNaN(volume)) {
      return undefined
    }

    if (!deviceApiId) {
      return undefined
    }

    if (!volumeSyncReadyRef.current) {
      volumeSyncReadyRef.current = true
      return undefined
    }

    sendRemoteControlCommand({
      type: 'set_volume',
      payload: { volume },
    })

    const abortController = new AbortController()
    const headers = new Headers({ 'Content-Type': 'application/json' })

    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/volume`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ volume }),
      signal: abortController.signal,
    }).catch((err) => {
      if (err?.name !== 'AbortError') {
        // eslint-disable-next-line no-console
        console.error('Failed to update retail player volume', err)
      }
    })

    return () => {
      abortController.abort()
    }
  }, [canControlDevice, deviceApiId, sendRemoteControlCommand, volume])

  const normalizedDeviceTrack = useMemo(() => {
    if (!device?.nowPlaying) {
      return null
    }

    if (typeof device.nowPlaying === 'object' && device.nowPlaying !== null) {
      const nowPlaying = device.nowPlaying
      const normalizedTitle = normalizeValue(nowPlaying.title)
      const normalizedArtist = normalizeValue(nowPlaying.artist)
      const normalizedArtwork = normalizeValue(nowPlaying.artworkUrl)
      const isLoading = Boolean(nowPlaying.isLoading)

      return {
        title:
          normalizedTitle ||
          (isLoading ? 'Loading' : device.channel || nowPlaying.streamName || 'Now Playing'),
        artist:
          normalizedArtist || (isLoading ? '' : device.channel || 'Retail Player'),
        artworkUrl: normalizedArtwork || null,
        isLoading,
      }
    }

    if (typeof device.nowPlaying === 'string') {
      const [titlePart, artistPart] = device.nowPlaying.split('|')
      return {
        title: titlePart ? titlePart.trim() : device.nowPlaying,
        artist: artistPart ? artistPart.trim() : device.channel || 'Retail Player',
        artworkUrl: null,
        isLoading: false,
      }
    }

    return null
  }, [device])

  useEffect(() => {
    if (!normalizedDeviceTrack || normalizedDeviceTrack.isLoading) {
      return
    }

    const trackSignature = [
      normalizedDeviceTrack.title || '',
      normalizedDeviceTrack.artist || device?.channel || '',
      normalizedDeviceTrack.artworkUrl || '',
    ].join('::')

    if (lastTrackSignatureRef.current === trackSignature) {
      return
    }

    lastTrackSignatureRef.current = trackSignature

    const nextTrack = {
      title: normalizedDeviceTrack.title || 'Now Playing',
      artist: normalizedDeviceTrack.artist || device?.channel || 'Retail Player',
      artworkUrl: normalizedDeviceTrack.artworkUrl || null,
    }

    setPreviousNowPlaying((previous) => {
      if (
        previous &&
        previous.title === nextTrack.title &&
        previous.artist === nextTrack.artist &&
        previous.artworkUrl === nextTrack.artworkUrl
      ) {
        return previous
      }
      return nextTrack
    })
  }, [device?.channel, normalizedDeviceTrack])

  const effectiveNowPlaying = useMemo(() => {
    if (normalizedDeviceTrack) {
      if (normalizedDeviceTrack.isLoading) {
        if (previousNowPlaying) {
          return previousNowPlaying
        }
        return {
          title: normalizedDeviceTrack.title || 'Loading',
          artist: normalizedDeviceTrack.artist || '',
          artworkUrl: normalizedDeviceTrack.artworkUrl || null,
        }
      }
      return normalizedDeviceTrack
    }

    return previousNowPlaying
  }, [normalizedDeviceTrack, previousNowPlaying])

  const effectiveTrackSignature = useMemo(() => {
    if (!effectiveNowPlaying) {
      return ''
    }

    return [
      effectiveNowPlaying.title || '',
      effectiveNowPlaying.artist || '',
      effectiveNowPlaying.artworkUrl || '',
    ].join('::')
  }, [effectiveNowPlaying])

  const trackPool = useMemo(() => {
    return effectiveNowPlaying ? [effectiveNowPlaying] : []
  }, [effectiveNowPlaying])

  useEffect(() => {
    if (!effectiveTrackSignature) {
      return
    }

    setCurrentTrackIndex(0)
  }, [effectiveTrackSignature])

  const currentTrack = useMemo(() => {
    if (!trackPool.length) {
      return {
        title: 'Now Playing',
        artist: device?.online === false ? '' : 'Retail Player',
        artworkUrl: null,
      }
    }
    const index = ((currentTrackIndex % trackPool.length) + trackPool.length) % trackPool.length
    return trackPool[index]
  }, [currentTrackIndex, device?.online, trackPool])

  const artworkUrl =
    (effectiveNowPlaying && effectiveNowPlaying.artworkUrl) ||
    normalizeValue(device?.nowPlaying?.artworkUrl) ||
    null
  const resolvedArtworkUrl = artworkUrl || currentTrack?.artworkUrl || null

  const isCuePlaybackActive = useMemo(
    () => Boolean(activeCueTriggerId || Number.isFinite(activeCueTriggerOrdinal)),
    [activeCueTriggerId, activeCueTriggerOrdinal],
  )

  const deviceTimeZone = useMemo(() => {
    if (device && typeof status.timeZone === 'string') {
      const trimmed = status.timeZone.trim()
      if (trimmed) {
        return trimmed
      }
    }

    const statusZone =
      device?.status && typeof device.status === 'object' && typeof device.status.timeZone === 'string'
        ? device.status.timeZone.trim()
        : ''

    return statusZone
  }, [device])

  const currentTimeLabel = useMemo(
    () => formatTime(deviceTime, deviceTimeZone || undefined),
    [deviceTime, deviceTimeZone],
  )

  const headerTimeLabel = currentTimeLabel

  const headerClockTooltip = useMemo(() => {
    const formatted = formatDetailedTime(deviceTime, deviceTimeZone || undefined)
    if (!formatted) {
      return ''
    }

    return deviceTimeZone ? `${formatted} ${deviceTimeZone}` : formatted
  }, [deviceTime, deviceTimeZone])

  const statusUpTime = device?.status?.upTime
  const hasStatusValue = useMemo(() => {
    if (statusUpTime === undefined || statusUpTime === null) {
      return false
    }
    if (typeof statusUpTime === 'string') {
      return statusUpTime.trim() !== ''
    }
    return true
  }, [statusUpTime])
  const statusUpTimeSeconds = useMemo(() => {
    if (statusUpTime === undefined || statusUpTime === null) {
      return null
    }
    if (typeof statusUpTime === 'number' && Number.isFinite(statusUpTime)) {
      return Math.max(0, Math.round(statusUpTime))
    }
    if (typeof statusUpTime === 'string') {
      const trimmed = statusUpTime.trim()
      if (trimmed === '') {
        return null
      }
      const parsed = Number.parseFloat(trimmed)
      if (Number.isFinite(parsed)) {
        return Math.max(0, Math.round(parsed))
      }
    }
    return null
  }, [statusUpTime])
  const hasRealtimeOnlineState = typeof device?.online === 'boolean'
  const isDeviceOnline = hasRealtimeOnlineState ? device.online : hasStatusValue
  const isOfflineUi = !isDeviceOnline
  const areNowPlayingControlsDisabled = !canControlDevice || isOfflineUi
  const formattedUpTime = useMemo(() => {
    if (statusUpTimeSeconds !== null) {
      const totalSeconds = statusUpTimeSeconds
      const days = Math.floor(totalSeconds / 86400)
      const hours = Math.floor((totalSeconds % 86400) / 3600)
      const minutes = Math.floor((totalSeconds % 3600) / 60)
      const seconds = totalSeconds % 60
      const pad = (value) => value.toString().padStart(2, '0')
      return `${days}d ${pad(hours)}h ${pad(minutes)}m ${pad(seconds)}s`
    }
    if (typeof statusUpTime === 'string' && statusUpTime.trim() !== '') {
      return statusUpTime.trim()
    }
    if (typeof statusUpTime === 'number' && Number.isFinite(statusUpTime)) {
      return String(statusUpTime)
    }
    return ''
  }, [statusUpTime, statusUpTimeSeconds])
  const statusTooltipTitle = isDeviceOnline
    ? `Uptime: ${formattedUpTime}`
    : 'Device offline'
  const statusAriaLabel = isDeviceOnline ? 'Device online' : 'Device offline'

  const statusItems = useMemo(() => {
    if (!device) {
      return []
    }

    return [
      {
        key: 'connected',
        icon: LinkIcon,
        intent: device.isConnected ? 'success' : 'danger',
        label: 'Connected',
      },
      { key: 'time', label: currentTimeLabel, labelForAria: 'Time' },
      {
        key: 'signal',
        icon: SignalWifi4BarIcon,
        intent: device.hasSignal ? 'success' : 'danger',
        label: 'Signal',
      },
      {
        key: 'muted',
        icon: isMuted ? VolumeOffIcon : VolumeUpIcon,
        intent: isMuted ? 'danger' : 'success',
        label: isMuted ? 'Muted' : 'Audio Enabled',
      },
    ]
  }, [currentTimeLabel, device, isMuted])

  const isDevicePasswordConfigured = useMemo(
    () =>
      typeof config.retailPlayerDeviceLockPassword === 'string' &&
      config.retailPlayerDeviceLockPassword.length > 0,
    [],
  )

  const isAccessBlockedByLock =
    Boolean(device) && isDeviceLocked(device) && !isDeviceUnlockedForSession(device)

  const handleLockDialogBack = useCallback(() => {
    if (history.length > 1) {
      history.goBack()
      return
    }

    history.push('/retailplayer/devices')
  }, [history])

  const handleLockDialogSubmit = useCallback(() => {
    if (!device) {
      return
    }

    if (!isDevicePasswordConfigured) {
      setLockError('Device lock password is not configured. Please contact an administrator.')
      return
    }

    if (lockPasswordInput === config.retailPlayerDeviceLockPassword) {
      markDeviceUnlockedForSession(device)
      setLockPasswordInput('')
      setLockError('')
      return
    }

    setLockError('Incorrect password. Please try again.')
  }, [device, isDevicePasswordConfigured, lockPasswordInput])

  useEffect(() => {
    if (!isAccessBlockedByLock) {
      setLockPasswordInput('')
      setLockError('')
    }
  }, [isAccessBlockedByLock])

  const handleToggleScheduleMenu = useCallback(() => {
    if (!availableSchedulesCount) {
      return
    }
    setScheduleMenuOpen((prev) => !prev)
  }, [availableSchedulesCount])

  const handleOpenCueDrawer = useCallback(() => {
    setCueDrawerOpen(true)
  }, [])

  const handleCloseCueDrawer = useCallback(() => {
    setCueDrawerOpen(false)
  }, [])

  const handleSelectChannel = useCallback(
    (schedule) => {
      if (!schedule) {
        return
      }

      const metadata =
        schedule && typeof schedule === 'object' && schedule.metadata && typeof schedule.metadata === 'object'
          ? schedule.metadata
          : {}

      const rawSchedule = schedule && typeof schedule === 'object' && schedule.raw && typeof schedule.raw === 'object'
        ? schedule.raw
        : {}

      const channelIdCandidates = [
        metadata.channelId,
        metadata.channel_id,
        metadata.channel,
        metadata.id,
        schedule.channelId,
        schedule.channel_id,
        schedule.id,
        rawSchedule.id,
        rawSchedule.channelId,
        rawSchedule.channel_id,
      ]

      const selectedChannelId = channelIdCandidates
        .map((candidate) => normalizeValue(candidate))
        .find((value) => value)

      if (!selectedChannelId) {
        // eslint-disable-next-line no-console
        console.error('Unable to determine channel id for selection', schedule)
        return
      }

      if (!canControlDevice || !deviceApiId) {
        return
      }

      if (channelRequestControllerRef.current) {
        channelRequestControllerRef.current.abort()
      }

      const abortController = new AbortController()
      channelRequestControllerRef.current = abortController

      const headers = new Headers({ 'Content-Type': 'application/json' })
      const selectedKey = schedule.key

      setPendingActiveChannelKey(selectedKey)

      httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/channel`, {
        method: 'POST',
        headers,
        body: JSON.stringify({ channel: selectedChannelId }),
        signal: abortController.signal,
      })
        .then(() => {
          refreshStatus()
        })
        .catch((err) => {
          if (err?.name !== 'AbortError') {
            // eslint-disable-next-line no-console
            console.error('Failed to update retail player channel', err)
          }
          setPendingActiveChannelKey(null)
        })
        .finally(() => {
          if (channelRequestControllerRef.current === abortController) {
            channelRequestControllerRef.current = null
          }
        })
    },
    [canControlDevice, deviceApiId, refreshStatus],
  )

  const handleSelectFromDropdown = useCallback(
    (schedule) => {
      setScheduleMenuOpen(false)
      handleSelectChannel(schedule)
    },
    [handleSelectChannel],
  )

  const clearVolumeTimeout = useCallback(() => {
    if (volumeTimeoutRef.current) {
      window.clearTimeout(volumeTimeoutRef.current)
      volumeTimeoutRef.current = null
    }
  }, [])

  const updateVolume = useCallback(
    (nextValue) => {
      setDisplayVolume((previous) => {
        const safePrevious = typeof previous === 'number' && !Number.isNaN(previous)
          ? previous
          : 0
        const rawNext =
          typeof nextValue === 'function' ? nextValue(safePrevious) : nextValue
        const clamped = clamp(Math.round(rawNext), 0, 100)
        clearVolumeTimeout()
        setIsMuted(clamped === 0)
        if (clamped > 0) {
          previousVolumeRef.current = clamped
        }
        volumeTimeoutRef.current = window.setTimeout(() => {
          setVolume(clamped)
          volumeTimeoutRef.current = null
        }, 150)
        return clamped
      })
    },
    [clearVolumeTimeout, setIsMuted],
  )

  const handleToggleMute = useCallback(() => {
    if (areNowPlayingControlsDisabled) {
      return
    }

    if (isMuted) {
      sendRemoteControlCommand({
        type: 'set_mute',
        payload: { muted: false },
      })
      const restoredVolumeCandidate =
        typeof previousVolumeRef.current === 'number'
        && !Number.isNaN(previousVolumeRef.current)
          ? previousVolumeRef.current
          : null
      const restoredVolume =
        restoredVolumeCandidate && restoredVolumeCandidate > 0
          ? restoredVolumeCandidate
          : typeof volume === 'number' && !Number.isNaN(volume) && volume > 0
            ? volume
            : typeof displayVolume === 'number' && !Number.isNaN(displayVolume)
              ? displayVolume
              : 0
      updateVolume(restoredVolume)
      return
    }

    sendRemoteControlCommand({
      type: 'set_mute',
      payload: { muted: true },
    })
    updateVolume((current) => {
      if (current > 0) {
        previousVolumeRef.current = current
      }
      return 0
    })
  }, [areNowPlayingControlsDisabled, isMuted, sendRemoteControlCommand, updateVolume])

  const handleVolumeChange = useCallback((_, newValue) => {
    if (areNowPlayingControlsDisabled) {
      return
    }

    const resolvedValue = Array.isArray(newValue) ? newValue[0] : newValue
    if (typeof resolvedValue !== 'number' || Number.isNaN(resolvedValue)) {
      return
    }
    updateVolume(resolvedValue)
  }, [areNowPlayingControlsDisabled, updateVolume])

  const handleRefresh = useCallback(() => {
    refreshStatus()
    setDeviceTime(new Date())
  }, [refreshStatus])

  const handleAdjustVolume = useCallback(
    (delta) => {
      updateVolume((prev) => prev + delta)
    },
    [updateVolume],
  )

  const handleBack = useCallback(() => {
    if (history.length > 1) {
      history.goBack()
      return
    }

    history.push('/retailplayer/devices')
  }, [history])

  const handleDislike = useCallback(() => {
    if (areNowPlayingControlsDisabled || isDislikeDisabled) {
      return
    }

    sendDislikeNotification()
    setShowDislikeMessage(true)
    if (dislikeTimeoutRef.current) {
      window.clearTimeout(dislikeTimeoutRef.current)
    }
    dislikeTimeoutRef.current = window.setTimeout(() => {
      setShowDislikeMessage(false)
      dislikeTimeoutRef.current = null
    }, 2000)

    if (dislikeDisableTimeoutRef.current) {
      window.clearTimeout(dislikeDisableTimeoutRef.current)
    }
    setIsDislikeDisabled(true)
    dislikeDisableTimeoutRef.current = window.setTimeout(() => {
      setIsDislikeDisabled(false)
      dislikeDisableTimeoutRef.current = null
    }, 5000)
  }, [areNowPlayingControlsDisabled, isDislikeDisabled, sendDislikeNotification])

  const handleSkip = useCallback(() => {
    if (areNowPlayingControlsDisabled || !trackPool.length) {
      return
    }

    setCurrentTrackIndex((previous) => (previous + 1) % trackPool.length)

    if (!canControlDevice || !deviceApiId) {
      return
    }

    const activeMetadata =
      activeSchedule && typeof activeSchedule === 'object' && activeSchedule.metadata
        ? activeSchedule.metadata
        : {}

    const channelIdCandidates = [
      normalizeValue(activeMetadata?.channelId),
      normalizeValue(activeMetadata?.channel_id),
      normalizeValue(activeMetadata?.channel),
      normalizeValue(device?.channel),
    ]
    const channelListCandidates = [
      normalizeValue(baseDevice?.channelList),
      normalizeValue(device?.channelList),
    ]

    const resolvedChannelId = channelIdCandidates.find((candidate) => candidate) || ''
    const resolvedChannelListId = channelListCandidates.find((candidate) => candidate) || ''

    if (!resolvedChannelId && !resolvedChannelListId) {
      return
    }

    if (channelRequestControllerRef.current) {
      channelRequestControllerRef.current.abort()
    }

    const abortController = new AbortController()
    channelRequestControllerRef.current = abortController

    const headers = new Headers({ 'Content-Type': 'application/json' })
    const body = {}

    if (resolvedChannelId) {
      body.channel = resolvedChannelId
    }
    if (resolvedChannelListId) {
      body.channelList = resolvedChannelListId
    }

    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/channel/toggle`, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: abortController.signal,
    })
      .then(() => {
        refreshStatus()
      })
      .catch((err) => {
        if (err?.name !== 'AbortError') {
          // eslint-disable-next-line no-console
          console.error('Failed to toggle retail player channel', err)
        }
      })
      .finally(() => {
        if (channelRequestControllerRef.current === abortController) {
          channelRequestControllerRef.current = null
        }
      })
  }, [
    activeSchedule,
    areNowPlayingControlsDisabled,
    baseDevice?.channelList,
    canControlDevice,
    device?.channel,
    device?.channelList,
    deviceApiId,
    refreshStatus,
    trackPool,
  ])

  const handleShortcutChannel = useCallback(
    (index) => {
      const schedule = availableSchedules[index]
      if (!schedule) {
        return
      }
      handleSelectChannel(schedule)
    },
    [availableSchedules, handleSelectChannel],
  )

  useEffect(() => {
    const handleKeyDown = (event) => {
      if (!device || isAccessBlockedByLock) {
        return
      }

      const target = event.target
      const tagName = target && target.tagName
      if (
        tagName === 'INPUT' ||
        tagName === 'TEXTAREA' ||
        (target && target.isContentEditable)
      ) {
        return
      }

      switch (event.key) {
        case 'm':
        case 'M':
          event.preventDefault()
          handleToggleMute()
          break
        case '+':
        case '=':
          event.preventDefault()
          handleAdjustVolume(5)
          break
        case '-':
          event.preventDefault()
          handleAdjustVolume(-5)
          break
        case '1':
          event.preventDefault()
          handleShortcutChannel(0)
          break
        case '2':
          event.preventDefault()
          handleShortcutChannel(1)
          break
        case '3':
          event.preventDefault()
          handleShortcutChannel(2)
          break
        default:
          break
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [device, handleAdjustVolume, handleShortcutChannel, handleToggleMute, isAccessBlockedByLock])

  useEffect(() => () => {
    clearVolumeTimeout()
    if (dislikeTimeoutRef.current) {
      window.clearTimeout(dislikeTimeoutRef.current)
    }
    if (dislikeDisableTimeoutRef.current) {
      window.clearTimeout(dislikeDisableTimeoutRef.current)
      dislikeDisableTimeoutRef.current = null
    }
    if (dislikeRetryTimeoutRef.current) {
      window.clearTimeout(dislikeRetryTimeoutRef.current)
      dislikeRetryTimeoutRef.current = null
    }
    if (dislikeRequestControllerRef.current) {
      dislikeRequestControllerRef.current.abort()
      dislikeRequestControllerRef.current = null
    }
    if (channelRequestControllerRef.current) {
      channelRequestControllerRef.current.abort()
      channelRequestControllerRef.current = null
    }
  }, [clearVolumeTimeout])

  if (!device) {
    const heading = notFound
      ? 'Device not found'
      : isBusy
        ? 'Loading device…'
        : 'Unable to load device'
    const message = notFound
      ? 'The device you are looking for is unavailable. Choose a device from the list to continue.'
      : isBusy
        ? 'Fetching the latest device details. This will only take a moment.'
        : 'We could not load this device right now. Please refresh and try again.'

    return (
      <div className={classes.notFoundWrapper}>
        <Title title="Retail Player" />
        <Typography component="h1" className={classes.notFoundTitle}>
          {heading}
        </Typography>
        <Typography className={classes.notFoundMessage}>{message}</Typography>
        {combinedError && !isBusy ? (
          <Typography className={classes.notFoundMessage} component="p">
            {combinedError.message || String(combinedError)}
          </Typography>
        ) : null}
      </div>
    )
  }

  return (
    <div
      className={rootClassName}
      aria-hidden={isAccessBlockedByLock ? 'true' : undefined}
      style={isAccessBlockedByLock ? { pointerEvents: 'none', userSelect: 'none' } : undefined}
    >
      <Title title="Retail Player" />

      <Dialog
        open={Boolean(device && isAccessBlockedByLock)}
        disableEscapeKeyDown
        classes={{ paper: classes.lockDialogPaper }}
        aria-labelledby="retail-player-lock-dialog-title"
      >
        <DialogTitle id="retail-player-lock-dialog-title">Unlock device</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            This device is locked. Enter the password to continue.
          </Typography>
          <TextField
            fullWidth
            margin="dense"
            variant="outlined"
            type="password"
            label="Password"
            autoFocus
            value={lockPasswordInput}
            onChange={(event) => {
              setLockPasswordInput(event.target.value)
              if (lockError) {
                setLockError('')
              }
            }}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault()
                handleLockDialogSubmit()
              }
            }}
          />
          {lockError ? <Typography className={classes.lockDialogError}>{lockError}</Typography> : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleLockDialogBack}>Back</Button>
          <Button color="primary" variant="contained" onClick={handleLockDialogSubmit}>
            Unlock
          </Button>
        </DialogActions>
      </Dialog>

      <header className={classes.headerBar}>
        <ButtonBase
          className={classes.headerBackButton}
          onClick={handleBack}
          aria-label="Go back"
          focusRipple
        >
          <ArrowBackIcon className={classes.headerBackIcon} />
        </ButtonBase>
        <div className={classes.headerCenter}>
          <Typography
            component="h1"
            className={classes.headerTitle}
            noWrap
            title={device?.name || 'Retail Player'}
          >
            {device?.name || 'Retail Player'}
          </Typography>
        </div>
        <div className={classes.headerStatusGroup}>
          <Tooltip title={statusTooltipTitle} placement="bottom">
            <span
              tabIndex={0}
              className={combineClasses(
                classes.headerStatusIcon,
                isDeviceOnline
                  ? classes.headerStatusIconOnline
                  : classes.headerStatusIconOffline,
              )}
              role="status"
              aria-label={statusAriaLabel}
            >
              {isDeviceOnline ? (
                <LinkIcon />
              ) : (
                <LinkOffIcon />
              )}
            </span>
          </Tooltip>
          <Tooltip
            title={headerClockTooltip || 'Device time unavailable'}
            placement="bottom"
          >
            <div
              className={classes.headerClock}
              aria-live="polite"
              aria-label={`Local time ${
                headerClockTooltip || headerTimeLabel || 'unavailable'
              }`}
            >
              {headerTimeLabel}
            </div>
          </Tooltip>
        </div>
      </header>

      <div className={classes.mainContent}>
        <section className={classes.nowPlayingCard} aria-label="Now playing">
          <div className={classes.artworkWrapper} aria-label="Artwork">
            <div className={classes.artworkCircle}>
              <div className={classes.artworkContent}>
                {resolvedArtworkUrl ? (
                  <img
                    src={resolvedArtworkUrl}
                    alt={`Artwork for ${currentTrack.title}`}
                    className={classes.artworkImage}
                  />
                ) : (
                  <svg
                    viewBox="0 0 200 200"
                    className={classes.discSvg}
                    role="img"
                    aria-hidden="true"
                  >
                    <circle cx="100" cy="100" r="98" className={classes.discOuter} />
                    <circle cx="100" cy="100" r="48" className={classes.discInner} />
                    <path
                      d="M150 50c-18-14-40-22-62-20"
                      className={classes.discHighlight}
                    />
                  </svg>
                )}
              </div>
            </div>
          </div>

          <div className={classes.nowPlayingBody}>
            <div className={classes.nowPlayingHeader}>
              <Typography
                component="h2"
                className={combineClasses(
                  classes.nowPlayingTitle,
                  isCuePlaybackActive ? classes.nowPlayingTitleCue : null,
                )}
                noWrap
                title={currentTrack.title}
              >
                {currentTrack.title}
              </Typography>
              {!isOfflineUi && currentTrack.artist ? (
                <Typography
                  className={classes.nowPlayingArtist}
                  noWrap
                  title={currentTrack.artist}
                >
                  {currentTrack.artist}
                </Typography>
              ) : null}
            </div>

            <div className={classes.nowPlayingFooter}>
              <section
                className={combineClasses(
                  classes.controlsRow,
                  areNowPlayingControlsDisabled ? classes.controlsRowDisabled : null,
                )}
                aria-label="Now playing controls"
              >
                <ButtonBase
                  className={classes.controlButton}
                  aria-label="Dislike"
                  onClick={handleDislike}
                  focusRipple
                  disabled={isDislikeDisabled || areNowPlayingControlsDisabled}
                >
                  <span className={classes.controlIcon} role="img" aria-hidden="true">
                    <BiDislike fontSize="inherit" />
                  </span>
                </ButtonBase>
                <ButtonBase
                  className={combineClasses(
                    classes.controlButton,
                    isMuted ? classes.controlButtonMuted : null,
                  )}
                  aria-label="Mute/Unmute"
                  onClick={handleToggleMute}
                  focusRipple
                  disabled={areNowPlayingControlsDisabled}
                >
                  <span className={classes.controlIcon} role="img" aria-hidden="true">
                    {isMuted ? <VolumeOffIcon fontSize="inherit" /> : <VolumeUpIcon fontSize="inherit" />}
                  </span>
                </ButtonBase>
                <ButtonBase
                  className={classes.controlButton}
                  aria-label="Skip"
                  onClick={handleSkip}
                  focusRipple
                  disabled={areNowPlayingControlsDisabled}
                >
                  <span className={classes.controlIcon} role="img" aria-hidden="true">
                    <MdSkipNext fontSize="inherit" />
                  </span>
                </ButtonBase>
                {hasCueTriggers ? (
                  <ButtonBase
                    className={combineClasses(
                      classes.controlButton,
                      classes.controlButtonCue,
                      isCuePlaybackActive ? classes.controlButtonCueActive : null,
                    )}
                    aria-label="Open cue controls"
                    onClick={handleOpenCueDrawer}
                    focusRipple
                    disabled={!deviceApiId || areNowPlayingControlsDisabled}
                  >
                    <span
                      className={combineClasses(
                        classes.controlIcon,
                        classes.controlIconCue,
                        isCuePlaybackActive ? classes.controlIconCueActive : null,
                      )}
                      role="img"
                      aria-hidden="true"
                    >
                      <CueIcon fontSize="inherit" />
                    </span>
                  </ButtonBase>
                ) : null}
              </section>

              <section
                className={combineClasses(
                  classes.volumeSection,
                  areNowPlayingControlsDisabled ? classes.volumeSectionDisabled : null,
                )}
                aria-label="Volume"
              >
                <div className={classes.volumeLabelRow}>
                  <Typography component="span">volume</Typography>
                  <Typography className={classes.volumeValue} aria-live="polite">
                    {typeof displayVolume === 'number' && !Number.isNaN(displayVolume)
                      ? displayVolume
                      : 0}
                  </Typography>
                </div>
                <Slider
                  classes={{
                    root: combineClasses(
                      classes.slider,
                      areNowPlayingControlsDisabled ? classes.sliderDisabled : null,
                    ),
                    track: classes.sliderTrack,
                    thumb: classes.sliderThumb,
                    rail: classes.sliderRail,
                  }}
                  value={typeof displayVolume === 'number' && !Number.isNaN(displayVolume)
                    ? displayVolume
                    : 0}
                  min={0}
                  max={100}
                  aria-label="Volume"
                  onChange={handleVolumeChange}
                  disabled={areNowPlayingControlsDisabled}
                />
              </section>
            </div>

            {showDislikeMessage ? (
              <Typography className={classes.dislikeMessage} aria-live="polite">
                Marked as disliked
              </Typography>
            ) : null}
          </div>
        </section>

        <section
          className={classes.list}
          aria-label="Available schedules"
          ref={scheduleDropdownRef}
        >
          <div className={classes.dropdownWrapper}>
            <ButtonBase
              className={combineClasses(
                classes.listItemButton,
                classes.dropdownTriggerButton,
              )}
              onClick={handleToggleScheduleMenu}
              focusRipple
              aria-haspopup="listbox"
              aria-expanded={isScheduleMenuOpen && Boolean(availableSchedulesCount)}
              aria-controls="schedule-menu"
              disabled={!availableSchedulesCount}
            >
              <div className={classes.listItem}>
                <DescriptionIcon
                  className={combineClasses(
                    classes.listIcon,
                    activeSchedule
                      ? classes.playlistIconActive
                      : classes.playlistIconInactive,
                  )}
                  aria-hidden="true"
                />
                <Typography
                  className={combineClasses(
                    classes.listText,
                    classes.playlistLabel,
                    activeSchedule
                      ? classes.playlistLabelActive
                      : classes.playlistLabelInactive,
                  )}
                  noWrap
                >
                  {dropdownLabel}
                </Typography>
                <ExpandMoreIcon
                  className={combineClasses(
                    classes.dropdownCaret,
                    isScheduleMenuOpen ? classes.dropdownCaretOpen : null,
                  )}
                  aria-hidden="true"
                />
              </div>
            </ButtonBase>
            <div
              className={combineClasses(
                classes.dropdownMenu,
                isScheduleMenuOpen ? classes.dropdownMenuOpen : null,
              )}
              role="listbox"
              id="schedule-menu"
              aria-hidden={!isScheduleMenuOpen}
            >
              {availableSchedules.map((schedule) => {
                const isActive = schedule.key === effectiveActiveChannelKey
                return (
                  <ButtonBase
                    key={schedule.key}
                    className={combineClasses(
                      classes.listItemButton,
                      classes.dropdownOptionButton,
                    )}
                    onClick={() => handleSelectFromDropdown(schedule)}
                    focusRipple
                    role="option"
                    aria-selected={isActive}
                  >
                    <div
                      className={combineClasses(
                        classes.listItem,
                        classes.dropdownOptionContent,
                      )}
                    >
                      <DescriptionIcon
                        className={combineClasses(
                          classes.listIcon,
                          isActive
                            ? classes.playlistIconActive
                            : classes.playlistIconInactive,
                        )}
                        aria-hidden="true"
                      />
                      <Typography
                        className={combineClasses(
                          classes.listText,
                          classes.playlistLabel,
                          classes.dropdownOptionLabel,
                          isActive
                            ? classes.playlistLabelActive
                            : classes.playlistLabelInactive,
                        )}
                        noWrap
                      >
                        {schedule.label}
                      </Typography>
                    </div>
                  </ButtonBase>
                )
              })}
            </div>
          </div>
        </section>
      </div>
      <Drawer
        anchor="right"
        open={isCueDrawerOpen}
        onClose={handleCloseCueDrawer}
        variant="temporary"
        ModalProps={{ keepMounted: true }}
        classes={{ paper: classes.cueDrawerPaper }}
        className={classes.cueDrawer}
      >
        <div className={classes.cueDrawerHeader}>
          <Typography component="h2" className={classes.cueDrawerTitle}>
            Cue Buttons
          </Typography>
        </div>
        <div className={classes.cueControls}>
          <ButtonBase
            onClick={stopCuePlayback}
            aria-label="Stop cue playback"
            focusRipple
            disabled={!deviceApiId}
            className={combineClasses(
              classes.cueStopButton,
              isCuePlaybackActive ? classes.cueStopButtonActive : null,
            )}
          >
            <StopIcon className={classes.cueStopIcon} />
            <span className={classes.cueStopLabel}>Stop</span>
          </ButtonBase>
        </div>
        <div className={classes.cueList} role="region" aria-label="Cue buttons list">
          {isCueLoading ? (
            <Typography>Loading cue buttons…</Typography>
          ) : cueError ? (
            <Typography className={classes.cueError}>
              {cueError.message || 'Unable to load cue buttons'}
            </Typography>
          ) : sortedCueTriggers.length ? (
            <List disablePadding>
              {sortedCueTriggers.map((trigger) => {
                const primaryText = trigger?.name || trigger?.id || 'Unnamed trigger'
                const secondaryParts = []
                if (trigger?.asset?.name) {
                  secondaryParts.push(trigger.asset.name)
                }

                const triggerId = getTriggerIdentifier(trigger)
                const triggerOrdinal = getTriggerOrdinal(trigger)
                const isTriggerActive =
                  (triggerId && triggerId === activeCueTriggerId) ||
                  (Number.isFinite(triggerOrdinal) &&
                    Number.isFinite(activeCueTriggerOrdinal) &&
                    triggerOrdinal === activeCueTriggerOrdinal)

                return (
                  <ListItem
                    key={trigger?.id || primaryText}
                    className={combineClasses(
                      classes.cueListItem,
                      isTriggerActive ? classes.cueListItemActive : null,
                    )}
                    button
                    onClick={() => sendCueTriggerAction(trigger)}
                  >
                    <ListItemText
                      primary={primaryText}
                      secondary={secondaryParts.join(' • ')}
                      primaryTypographyProps={{
                        className: combineClasses(
                          classes.cueListPrimary,
                          isTriggerActive ? classes.cueListPrimaryActive : null,
                        ),
                      }}
                      secondaryTypographyProps={{
                        className: combineClasses(
                          classes.cueListSecondary,
                          isTriggerActive ? classes.cueListSecondaryActive : null,
                        ),
                      }}
                    />
                  </ListItem>
                )
              })}
            </List>
          ) : (
            <Typography className={classes.cueEmpty}>No cue buttons available</Typography>
          )}
        </div>
      </Drawer>
    </div>
  )
}

export default RetailPlayerDashboard
