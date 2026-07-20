import React from 'react'
import { useTranslate } from 'react-admin'
import { useSelector } from 'react-redux'
import {
  CircularProgress,
  IconButton,
  Tooltip,
  makeStyles,
} from '@material-ui/core'
import { VscSync } from 'react-icons/vsc'
import subsonic from '../subsonic'

const useStyles = makeStyles({
  wrapper: {
    position: 'relative',
    display: 'inline-flex',
  },
  progress: {
    position: 'absolute',
    top: 10,
    left: 10,
    zIndex: 1,
  },
  button: {
    color: 'inherit',
    zIndex: 2,
  },
})

// Replaces react-admin's built-in AppBar refresh button (hidden via the
// RaLoadingIndicator theme override in Layout.jsx) with a quick scan trigger.
const QuickScanButton = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const scanning = useSelector((state) => state.activity.scanStatus?.scanning)

  const triggerQuickScan = () => subsonic.startScan({ fullScan: false })

  return (
    <Tooltip title={translate('activity.quickScan')}>
      <span className={classes.wrapper}>
        <IconButton
          className={classes.button}
          onClick={triggerQuickScan}
          disabled={scanning}
          data-testid="quick-scan-btn"
          aria-label={translate('activity.quickScan')}
        >
          <VscSync />
        </IconButton>
        {scanning && (
          <CircularProgress size={20} className={classes.progress} />
        )}
      </span>
    </Tooltip>
  )
}

export default QuickScanButton
