import { useCallback, useEffect, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  makeStyles,
} from '@material-ui/core'
import PropTypes from 'prop-types'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles((theme) => ({
  panel: {
    marginBottom: theme.spacing(2),
    border: `1px solid ${theme.palette.divider}`,
    boxShadow: 'none',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(1.5),
  },
  row: { cursor: 'pointer' },
  selected: {
    background:
      theme.palette.type === 'dark'
        ? 'rgba(255,255,255,0.08)'
        : 'rgba(0,0,0,0.05)',
  },
  chips: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(0.5),
  },
  comparison: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
    gap: theme.spacing(2),
    marginTop: theme.spacing(2),
  },
  trackList: {
    maxHeight: 300,
    overflow: 'auto',
    margin: theme.spacing(1, 0, 0),
    paddingLeft: theme.spacing(3),
  },
  audit: {
    marginTop: theme.spacing(2),
  },
  error: {
    color: theme.palette.error.main,
    marginTop: theme.spacing(1),
  },
}))

const eventLabel = (value) =>
  String(value || '')
    .split('_')
    .map((part) => `${part.charAt(0).toUpperCase()}${part.slice(1)}`)
    .join(' ')

const formatDate = (value) => {
  const date = value ? new Date(value) : null
  return date && !Number.isNaN(date.getTime()) ? date.toLocaleString() : '—'
}

const TrackList = ({ title, tracks }) => (
  <Box>
    <Typography variant="subtitle2">
      {title} ({tracks.length})
    </Typography>
    <ol>
      {tracks.map((track) => (
        <li key={`${track.mediaFileId}-${track.position}`}>
          <Typography variant="body2">
            {track.title || track.mediaFileId}
            {track.artist ? ` — ${track.artist}` : ''}
          </Typography>
        </li>
      ))}
    </ol>
  </Box>
)

TrackList.propTypes = {
  title: PropTypes.string.isRequired,
  tracks: PropTypes.array.isRequired,
}

const PlaylistHistoryPanel = ({
  playlist,
  refreshToken,
  onRollbackCreated,
}) => {
  const classes = useStyles()
  const [versions, setVersions] = useState([])
  const [events, setEvents] = useState([])
  const [selectedId, setSelectedId] = useState('')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const playlistId = playlist?.id || ''

  const loadHistory = useCallback(async () => {
    if (!playlistId) {
      setVersions([])
      setEvents([])
      return
    }
    setLoading(true)
    setError('')
    try {
      const query = `playlistId=${encodeURIComponent(playlistId)}`
      const [historyResponse, auditResponse] = await Promise.all([
        httpClient(`/api/playlist-history?${query}`),
        httpClient(`/api/playlist-history/audit?${query}`),
      ])
      const nextVersions = historyResponse.json?.versions || []
      setVersions(nextVersions)
      setEvents(auditResponse.json?.events || [])
      setSelectedId((current) =>
        nextVersions.some((version) => version.id === current)
          ? current
          : nextVersions[0]?.id || '',
      )
    } catch (err) {
      setError(
        err?.body?.error || err?.message || 'Could not load playlist history',
      )
    } finally {
      setLoading(false)
    }
  }, [playlistId])

  useEffect(() => {
    setSelectedId('')
    void loadHistory()
  }, [loadHistory, refreshToken])

  const selected = versions.find((version) => version.id === selectedId) || null

  const createRollback = async (version) => {
    if (!version || busy) return
    setBusy(version.id)
    setError('')
    try {
      const { json } = await httpClient(
        `/api/playlist-history/${encodeURIComponent(version.id)}/rollback`,
        { method: 'POST' },
      )
      onRollbackCreated?.(json)
      await loadHistory()
    } catch (err) {
      setError(
        err?.body?.error || err?.message || 'Could not create rollback draft',
      )
    } finally {
      setBusy('')
    }
  }

  if (!playlistId) return null

  return (
    <Card className={classes.panel} variant="outlined">
      <CardContent>
        <Box className={classes.header}>
          <Box>
            <Typography variant="h6">Playlist history and audit</Typography>
            <Typography variant="body2" color="textSecondary">
              Published versions are immutable. Rollback always creates a new
              draft that must be reviewed and approved.
            </Typography>
          </Box>
          {loading ? <CircularProgress size={24} /> : null}
        </Box>

        {versions.length ? (
          <TableContainer>
            <Table size="small" aria-label="Playlist version history">
              <TableHead>
                <TableRow>
                  {[
                    'Version',
                    'Date and user',
                    'Changes',
                    'Selection source',
                    'Approval',
                    'AI provenance',
                    'Action',
                  ].map((label) => (
                    <TableCell key={label}>{label}</TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {versions.map((version) => {
                  const summary = version.changeSummary || {}
                  return (
                    <TableRow
                      key={version.id}
                      hover
                      className={`${classes.row} ${
                        selectedId === version.id ? classes.selected : ''
                      }`}
                      onClick={() => setSelectedId(version.id)}
                    >
                      <TableCell>
                        <Typography variant="body2">
                          Version {version.version}
                        </Typography>
                        {version.rollbackFromVersion ? (
                          <Typography variant="caption" color="textSecondary">
                            Rollback from v{version.rollbackFromVersion}
                          </Typography>
                        ) : null}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {formatDate(version.publishedAt)}
                        </Typography>
                        <Typography variant="caption" color="textSecondary">
                          {version.publishedBy || 'Unknown user'}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Box className={classes.chips}>
                          <Chip
                            size="small"
                            label={`${summary.added || 0} added`}
                          />
                          <Chip
                            size="small"
                            label={`${summary.removed || 0} removed`}
                          />
                          <Chip
                            size="small"
                            label={`${summary.replaced || 0} replaced`}
                          />
                          <Chip
                            size="small"
                            label={`${summary.reordered || 0} reordered`}
                          />
                        </Box>
                      </TableCell>
                      <TableCell>
                        {summary.aiSelected || 0} AI /{' '}
                        {summary.manualSelected || 0} manual
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {version.approvedBy || '—'}
                        </Typography>
                        <Typography variant="caption" color="textSecondary">
                          {formatDate(version.approvedAt)}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="caption" component="div">
                          Model: {version.aiModelVersion || '—'}
                        </Typography>
                        <Typography variant="caption" component="div">
                          Index: {version.aiIndexVersion || '—'}
                        </Typography>
                        <Typography variant="caption" component="div">
                          Rules: {version.rulesetVersion || '—'}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Button
                          size="small"
                          variant="outlined"
                          disabled={Boolean(busy)}
                          onClick={(event) => {
                            event.stopPropagation()
                            void createRollback(version)
                          }}
                        >
                          Create rollback draft
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </TableContainer>
        ) : !loading ? (
          <Typography variant="body2" color="textSecondary">
            No published versions yet. The first snapshot is created when an
            approved draft is published.
          </Typography>
        ) : null}

        {error ? (
          <Typography className={classes.error} variant="body2">
            {error}
          </Typography>
        ) : null}

        {selected ? (
          <>
            <Box className={classes.comparison}>
              <TrackList title="Before" tracks={selected.before || []} />
              <TrackList title="After" tracks={selected.after || []} />
            </Box>
            <Typography variant="subtitle2">
              Change details ({selected.changeDetails?.length || 0})
            </Typography>
            <ul>
              {(selected.changeDetails || []).map((change, index) => (
                <li
                  key={`${change.kind}-${change.mediaFileId}-${change.replacedId}-${index}`}
                >
                  <Typography variant="body2">
                    {eventLabel(change.kind)}:{' '}
                    {change.replacedTitle ? `${change.replacedTitle} → ` : ''}
                    {change.title || change.mediaFileId || 'Track'} ·{' '}
                    {change.aiSelected
                      ? 'AI-generated'
                      : change.source || 'manual'}
                  </Typography>
                  {change.reason ? (
                    <Typography variant="caption" color="textSecondary">
                      {change.reason}
                    </Typography>
                  ) : null}
                </li>
              ))}
            </ul>
          </>
        ) : null}

        <Box className={classes.audit}>
          <Typography variant="subtitle1">Audit events</Typography>
          {events.length ? (
            <TableContainer>
              <Table size="small" aria-label="Playlist audit events">
                <TableHead>
                  <TableRow>
                    {['Event', 'Date', 'Actor', 'Summary', 'Version'].map(
                      (label) => (
                        <TableCell key={label}>{label}</TableCell>
                      ),
                    )}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {events.map((event) => (
                    <TableRow key={event.id}>
                      <TableCell>{eventLabel(event.eventType)}</TableCell>
                      <TableCell>{formatDate(event.createdAt)}</TableCell>
                      <TableCell>{event.actor || '—'}</TableCell>
                      <TableCell>{event.summary}</TableCell>
                      <TableCell>
                        {event.versionNumber
                          ? `Version ${event.versionNumber}`
                          : '—'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          ) : (
            <Typography variant="body2" color="textSecondary">
              No audit events yet.
            </Typography>
          )}
        </Box>
      </CardContent>
    </Card>
  )
}

PlaylistHistoryPanel.propTypes = {
  playlist: PropTypes.shape({
    id: PropTypes.string,
    name: PropTypes.string,
  }),
  refreshToken: PropTypes.number,
  onRollbackCreated: PropTypes.func,
}

export default PlaylistHistoryPanel
