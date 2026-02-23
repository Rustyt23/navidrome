import React, { useCallback, useEffect, useState } from 'react'
import {
  Card,
  CardContent,
  IconButton,
  Popover,
  Tooltip,
  Typography,
  makeStyles,
} from '@material-ui/core'
import PhotoIcon from '@material-ui/icons/Photo'
import { useTranslate } from 'react-admin'
import { httpClient } from '../dataProvider'
import { REST_URL } from '../consts'

const useStyles = makeStyles((theme) => ({
  button: (props) => ({
    color: props.open ? theme.palette.secondary.main : 'inherit',
  }),
  card: {
    width: '64em',
    maxWidth: 'calc(100vw - 32px)',
  },
  cardContent: {
    padding: `${theme.spacing(2)}px !important`,
    '&:last-child': {
      paddingBottom: `${theme.spacing(2)}px !important`,
    },
  },
  title: {
    fontWeight: theme.typography.fontWeightBold,
    marginBottom: theme.spacing(2),
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(4, minmax(220px, 1fr))',
    gap: theme.spacing(1.5),
  },
  statCard: {
    border: '1px solid rgba(255,255,255,0.12)',
    borderRadius: 6,
    padding: theme.spacing(1.5),
    background: 'rgba(255,255,255,0.02)',
  },
  statTitle: {
    fontSize: '1.6rem',
    marginBottom: theme.spacing(1),
  },
  statRow: {
    display: 'flex',
    justifyContent: 'space-between',
    lineHeight: 1.8,
  },
}))

const defaultFieldStats = {
  alreadyExist: 0,
  missing: 0,
  fetching: 0,
  fetched: 0,
  updated: 0,
  toBeFetch: 0,
  couldntFetch: 0,
}

const statCards = [
  { key: 'coverArt', label: 'Cover Art' },
  { key: 'album', label: 'Album' },
  { key: 'year', label: 'Year' },
  { key: 'genre', label: 'Genre' },
]

const StatCard = ({ title, stats = defaultFieldStats }) => {
  const classes = useStyles({})

  return (
    <div className={classes.statCard}>
      <div className={classes.statTitle}>{title}</div>
      {[
        ['Already exist', stats.alreadyExist],
        ['Missing', stats.missing],
        ['Fetching', stats.fetching],
        ['Fetched', stats.fetched],
        ['Updated', stats.updated],
        ['To be fetch', stats.toBeFetch],
        ["Couldn't fetched", stats.couldntFetch],
      ].map(([label, value]) => (
        <div key={label} className={classes.statRow}>
          <span>{label}</span>
          <span>{value}</span>
        </div>
      ))}
    </div>
  )
}

const CoverArtMetadataPanel = () => {
  const [anchorEl, setAnchorEl] = useState(null)
  const [job, setJob] = useState(null)
  const translate = useTranslate()
  const open = Boolean(anchorEl)
  const classes = useStyles({ open })

  const fetchStatus = useCallback(async () => {
    const response = await httpClient(`${REST_URL}/song/metadata/spotify/status`, {
      method: 'GET',
      cache: 'no-store',
    })
    setJob(response?.json || null)
  }, [])

  useEffect(() => {
    if (!open) return undefined

    fetchStatus().catch(() => {})

    const id = setInterval(() => {
      fetchStatus().catch(() => {})
    }, 1000)

    return () => clearInterval(id)
  }, [fetchStatus, open])

  const title = 'Cover Art Metadata'

  return (
    <>
      <Tooltip title={translate('resources.covertart.name', { smart_count: 2 })}>
        <IconButton
          id="btn-coverart-panel"
          aria-label="cover-art-metadata"
          className={classes.button}
          onClick={(e) => setAnchorEl(anchorEl ? null : e.currentTarget)}
        >
          <PhotoIcon />
        </IconButton>
      </Tooltip>
      <Popover
        id="panel-coverart"
        anchorEl={anchorEl}
        open={open}
        onClose={() => setAnchorEl(null)}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'center',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
      >
        <Card className={classes.card}>
          <CardContent className={classes.cardContent}>
            <Typography variant="h5" className={classes.title}>
              {title}
            </Typography>
            <div className={classes.grid}>
              {statCards.map((card) => (
                <StatCard
                  key={card.key}
                  title={card.label}
                  stats={job?.stats?.[card.key] || defaultFieldStats}
                />
              ))}
            </div>
          </CardContent>
        </Card>
      </Popover>
    </>
  )
}

export default CoverArtMetadataPanel
