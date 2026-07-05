import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  makeStyles,
} from '@material-ui/core'
import AssessmentOutlinedIcon from '@material-ui/icons/AssessmentOutlined'
import RefreshIcon from '@material-ui/icons/Refresh'
import { Title, useDataProvider } from 'react-admin'
import PropTypes from 'prop-types'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(2.5),
    color: theme.palette.text.primary,
  },
  heading: {
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(2),
  },
  subtitle: {
    color: theme.palette.text.secondary,
    marginTop: theme.spacing(0.5),
  },
  tableContainer: {
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: 8,
    background: theme.palette.background.paper,
  },
  table: {
    minWidth: 1900,
  },
  tableHead: {
    background: theme.palette.type === 'dark' ? '#202b3b' : '#f4f6f9',
  },
  name: {
    fontWeight: 600,
    minWidth: 180,
  },
  summary: {
    minWidth: 260,
    maxWidth: 360,
    whiteSpace: 'normal',
  },
  status: {
    fontWeight: 600,
    whiteSpace: 'nowrap',
  },
  statusReady: { color: '#43a047' },
  statusError: { color: theme.palette.error.main },
  actions: {
    display: 'flex',
    gap: theme.spacing(0.75),
    minWidth: 250,
  },
  empty: {
    padding: theme.spacing(6),
    textAlign: 'center',
    color: theme.palette.text.secondary,
  },
  report: {
    display: 'grid',
    gap: theme.spacing(2),
    paddingTop: theme.spacing(1),
  },
  reportGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))',
    gap: theme.spacing(1.5),
  },
  reportCard: {
    border: `1px solid ${theme.palette.divider}`,
    boxShadow: 'none',
  },
  reportTitle: {
    fontWeight: 600,
    marginBottom: theme.spacing(1),
  },
  chips: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(0.75),
  },
  issueList: {
    margin: 0,
    paddingLeft: theme.spacing(2.25),
    '& li': { marginBottom: theme.spacing(0.75) },
  },
  replacement: {
    marginTop: theme.spacing(1),
    paddingTop: theme.spacing(1),
    borderTop: `1px solid ${theme.palette.divider}`,
  },
  error: {
    color: theme.palette.error.main,
    marginBottom: theme.spacing(1.5),
  },
}))

const formatDuration = (value) => {
  const total = Math.max(0, Math.round(Number(value) || 0))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  return hours
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
    : `${minutes}:${String(seconds).padStart(2, '0')}`
}

const formatDate = (value) => {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString()
}

const count = (value) => (Array.isArray(value) ? value.length : 0)

const SummaryChips = ({ values }) => (
  <Box display="flex" flexWrap="wrap" style={{ gap: 6 }}>
    {values?.length ? (
      values.map((item) => (
        <Chip
          key={item.name}
          size="small"
          variant="outlined"
          label={`${item.name} · ${item.count} (${item.percent}%)`}
        />
      ))
    ) : (
      <Typography variant="body2" color="textSecondary">
        No reliable data
      </Typography>
    )}
  </Box>
)

SummaryChips.propTypes = {
  values: PropTypes.arrayOf(
    PropTypes.shape({
      name: PropTypes.string.isRequired,
      count: PropTypes.number.isRequired,
      percent: PropTypes.number.isRequired,
    }),
  ),
}

const ReportCard = ({ title, children }) => {
  const classes = useStyles()
  return (
    <Card className={classes.reportCard} variant="outlined">
      <CardContent>
        <Typography className={classes.reportTitle} variant="subtitle1">
          {title}
        </Typography>
        {children}
      </CardContent>
    </Card>
  )
}

ReportCard.propTypes = {
  title: PropTypes.string.isRequired,
  children: PropTypes.node.isRequired,
}

const PlaylistReport = ({ analysis }) => {
  const classes = useStyles()
  const explicitSongs = analysis?.explicitRisk?.songs || []
  return (
    <Box className={classes.report}>
      <ReportCard title="Playlist summary">
        <Typography variant="body1">{analysis.summary}</Typography>
        <Typography variant="body2" color="textSecondary">
          {analysis.songCount} songs · {formatDuration(analysis.duration)}
        </Typography>
      </ReportCard>

      <Box className={classes.reportGrid}>
        <ReportCard title="Genre summary">
          <SummaryChips values={analysis.genreSummary} />
        </ReportCard>
        <ReportCard title="Artist summary">
          <SummaryChips values={analysis.artistSummary} />
        </ReportCard>
        <ReportCard title="Explicit risk">
          <Typography variant="body1">
            {analysis.explicitRisk?.level || 'none'} ·{' '}
            {analysis.explicitRisk?.count || 0} songs (
            {analysis.explicitRisk?.percent || 0}%)
          </Typography>
          {explicitSongs.length ? (
            <ul className={classes.issueList}>
              {explicitSongs.map((song) => (
                <li key={song.songId}>
                  {song.title} — {song.artist}
                </li>
              ))}
            </ul>
          ) : null}
        </ReportCard>
      </Box>

      <Box className={classes.reportGrid}>
        <ReportCard
          title={`Metadata issues (${count(analysis.metadataIssues)})`}
        >
          {count(analysis.metadataIssues) ? (
            <ul className={classes.issueList}>
              {analysis.metadataIssues.map((issue, index) => (
                <li key={`${issue.songId}-${index}`}>
                  {issue.title} — missing {issue.missing.join(', ')}
                </li>
              ))}
            </ul>
          ) : (
            <Typography variant="body2">
              No missing metadata detected.
            </Typography>
          )}
        </ReportCard>
        <ReportCard
          title={`Duplicate songs (${count(analysis.duplicateSongs)})`}
        >
          {count(analysis.duplicateSongs) ? (
            <ul className={classes.issueList}>
              {analysis.duplicateSongs.map((issue, index) => (
                <li key={`${issue.songId}-${index}`}>
                  {issue.title} — {issue.count} copies at positions{' '}
                  {issue.positions.join(', ')}
                </li>
              ))}
            </ul>
          ) : (
            <Typography variant="body2">No duplicates detected.</Typography>
          )}
        </ReportCard>
        <ReportCard
          title={`Loudness issues (${count(analysis.loudnessIssues)})`}
        >
          {count(analysis.loudnessIssues) ? (
            <ul className={classes.issueList}>
              {analysis.loudnessIssues.map((issue, index) => (
                <li key={`${issue.songId}-${index}`}>
                  {issue.title}: {issue.reason}
                </li>
              ))}
            </ul>
          ) : (
            <Typography variant="body2">No LUFS outliers detected.</Typography>
          )}
        </ReportCard>
        <ReportCard title={`Bad-fit songs (${count(analysis.badFitSongs)})`}>
          {count(analysis.badFitSongs) ? (
            <ul className={classes.issueList}>
              {analysis.badFitSongs.map((issue, index) => (
                <li key={`${issue.songId}-${index}`}>
                  {issue.title} — {issue.reasons.join('; ')}
                </li>
              ))}
            </ul>
          ) : (
            <Typography variant="body2">
              No profile outliers detected.
            </Typography>
          )}
        </ReportCard>
      </Box>

      <ReportCard title="Suggested replacements">
        {count(analysis.suggestedReplacements) ? (
          analysis.suggestedReplacements.map((replacement, index) => (
            <Box
              className={index ? classes.replacement : undefined}
              key={`${replacement.forSong.songId}-${index}`}
            >
              <Typography variant="subtitle2">
                Replace {replacement.forSong.title} —{' '}
                {replacement.reasons.join('; ')}
              </Typography>
              {replacement.suggestions?.length ? (
                <ul className={classes.issueList}>
                  {replacement.suggestions.map((song) => (
                    <li key={song.songId}>
                      {song.title} — {song.artist} ·{' '}
                      {song.genre || 'unknown genre'} · {song.bpm || '—'} BPM ·{' '}
                      {song.lufs || '—'} LUFS
                    </li>
                  ))}
                </ul>
              ) : (
                <Typography variant="body2" color="textSecondary">
                  No suitable indexed clean replacement was found.
                </Typography>
              )}
            </Box>
          ))
        ) : (
          <Typography variant="body2">
            No replacements are currently needed.
          </Typography>
        )}
      </ReportCard>

      <ReportCard title="Recommendations">
        <ul className={classes.issueList}>
          {(analysis.recommendations || []).map((recommendation, index) => (
            <li key={`${recommendation}-${index}`}>{recommendation}</li>
          ))}
        </ul>
      </ReportCard>
    </Box>
  )
}

PlaylistReport.propTypes = {
  analysis: PropTypes.object.isRequired,
}

const PlaylistAiToolPage = () => {
  const classes = useStyles()
  const dataProvider = useDataProvider()
  const dataProviderRef = useRef(dataProvider)
  const [playlists, setPlaylists] = useState([])
  const [selected, setSelected] = useState([])
  const [analyses, setAnalyses] = useState({})
  const [analyzing, setAnalyzing] = useState({})
  const [errors, setErrors] = useState({})
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [report, setReport] = useState(null)
  const [isIndexing, setIsIndexing] = useState(false)
  const [indexMessage, setIndexMessage] = useState('')
  const [indexError, setIndexError] = useState('')

  const loadPlaylists = useCallback(async () => {
    setLoading(true)
    setLoadError('')
    try {
      const response = await dataProviderRef.current.getList('playlist', {
        pagination: { page: 1, perPage: 500 },
        sort: { field: 'name', order: 'ASC' },
        filter: {},
      })
      setPlaylists(Array.isArray(response?.data) ? response.data : [])
    } catch (error) {
      setLoadError(error?.message || 'Could not load playlists')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadPlaylists()
  }, [loadPlaylists])

  const selectedSet = useMemo(() => new Set(selected), [selected])
  const allSelected =
    playlists.length > 0 && selected.length === playlists.length

  const togglePlaylist = (playlistId) => {
    setSelected((current) =>
      current.includes(playlistId)
        ? current.filter((id) => id !== playlistId)
        : [...current, playlistId],
    )
  }

  const analyzePlaylist = async (playlist) => {
    if (analyzing[playlist.id]) return
    setAnalyzing((current) => ({ ...current, [playlist.id]: true }))
    setErrors((current) => ({ ...current, [playlist.id]: '' }))
    try {
      const { json } = await httpClient('/api/ai/rag/playlist/analyze', {
        method: 'POST',
        body: JSON.stringify({ playlistId: playlist.id }),
      })
      setAnalyses((current) => ({ ...current, [playlist.id]: json }))
      setReport(json)
    } catch (error) {
      setErrors((current) => ({
        ...current,
        [playlist.id]: error?.message || 'Analysis failed',
      }))
    } finally {
      setAnalyzing((current) => ({ ...current, [playlist.id]: false }))
    }
  }

  const indexPlaylists = async (force = false) => {
    if (isIndexing || playlists.length === 0) return
    setIsIndexing(true)
    setIndexMessage('')
    setIndexError('')
    try {
      const { json } = await httpClient('/api/ai/rag/index', {
        method: 'POST',
        body: JSON.stringify({
          includeSongs: false,
          includePlaylists: true,
          playlistLimit: Math.min(playlists.length, 500),
          force,
        }),
      })
      const result = json?.playlists || {}
      setIndexMessage(
        `${force ? 'Re-indexed' : 'Indexed'} ${Number(result.indexed) || 0} playlists, skipped ${
          Number(result.skipped) || 0
        }, failed ${Number(result.failed) || 0}.`,
      )
      if (result.error) setIndexError(result.error)
    } catch (error) {
      setIndexError(error?.message || 'Could not index playlists')
    } finally {
      setIsIndexing(false)
    }
  }

  return (
    <Box className={classes.root}>
      <Title title="Playlist-Ai-Tool" />
      <Box className={classes.heading}>
        <Box>
          <Typography variant="h5">Playlist-Ai-Tool</Typography>
          <Typography className={classes.subtitle} variant="body2">
            Analyze playlist consistency and review replacement suggestions. No
            playlist changes are applied.
          </Typography>
        </Box>
        <Box className={classes.actions}>
          <Button
            variant="outlined"
            color="primary"
            onClick={() => indexPlaylists(false)}
            disabled={loading || isIndexing || playlists.length === 0}
          >
            {isIndexing ? 'Indexing…' : 'Index playlists'}
          </Button>
          <Button
            variant="outlined"
            color="primary"
            onClick={() => indexPlaylists(true)}
            disabled={loading || isIndexing || playlists.length === 0}
          >
            Re-index playlists
          </Button>
          <Button
            variant="outlined"
            color="primary"
            startIcon={<RefreshIcon />}
            onClick={loadPlaylists}
            disabled={loading}
          >
            Refresh
          </Button>
        </Box>
      </Box>

      {loadError ? (
        <Typography className={classes.error}>{loadError}</Typography>
      ) : null}
      {indexMessage ? (
        <Typography className={classes.subtitle}>{indexMessage}</Typography>
      ) : null}
      {indexError ? (
        <Typography className={classes.error}>{indexError}</Typography>
      ) : null}
      <TableContainer className={classes.tableContainer}>
        <Table
          size="small"
          className={classes.table}
          aria-label="Playlist AI analysis"
        >
          <TableHead className={classes.tableHead}>
            <TableRow>
              <TableCell padding="checkbox">
                <Checkbox
                  checked={allSelected}
                  indeterminate={selected.length > 0 && !allSelected}
                  onChange={() =>
                    setSelected(
                      allSelected
                        ? []
                        : playlists.map((playlist) => playlist.id),
                    )
                  }
                  inputProps={{ 'aria-label': 'Select all playlists' }}
                />
              </TableCell>
              {[
                'Playlist name',
                'Owner',
                'Visibility',
                'Song count',
                'Total duration',
                'Last updated',
                'AI analysis status',
                'AI result / summary',
                'Explicit risk',
                'Metadata issues',
                'Duplicates',
                'Loudness issues',
                'Bad-fit songs',
                'Recommendations',
                'Actions',
              ].map((label) => (
                <TableCell key={label}>{label}</TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={16} className={classes.empty}>
                  <CircularProgress size={28} />
                </TableCell>
              </TableRow>
            ) : playlists.length === 0 ? (
              <TableRow>
                <TableCell colSpan={16} className={classes.empty}>
                  No playlists found.
                </TableCell>
              </TableRow>
            ) : (
              playlists.map((playlist) => {
                const analysis = analyses[playlist.id]
                const isAnalyzing = Boolean(analyzing[playlist.id])
                const error = errors[playlist.id]
                const status = isAnalyzing
                  ? 'Analyzing…'
                  : error
                    ? 'Failed'
                    : analysis
                      ? 'Analyzed'
                      : 'Not analyzed'
                return (
                  <TableRow key={playlist.id} hover>
                    <TableCell padding="checkbox">
                      <Checkbox
                        checked={selectedSet.has(playlist.id)}
                        onChange={() => togglePlaylist(playlist.id)}
                        inputProps={{ 'aria-label': `Select ${playlist.name}` }}
                      />
                    </TableCell>
                    <TableCell className={classes.name}>
                      {playlist.name}
                    </TableCell>
                    <TableCell>{playlist.ownerName || '—'}</TableCell>
                    <TableCell>
                      {playlist.public ? 'Public' : 'Private'}
                    </TableCell>
                    <TableCell>{playlist.songCount || 0}</TableCell>
                    <TableCell>{formatDuration(playlist.duration)}</TableCell>
                    <TableCell>{formatDate(playlist.updatedAt)}</TableCell>
                    <TableCell
                      className={`${classes.status} ${analysis ? classes.statusReady : ''} ${error ? classes.statusError : ''}`}
                    >
                      {status}
                    </TableCell>
                    <TableCell className={classes.summary}>
                      {error || analysis?.summary || '—'}
                    </TableCell>
                    <TableCell>
                      {analysis
                        ? `${analysis.explicitRisk?.level || 'none'} (${analysis.explicitRisk?.count || 0})`
                        : '—'}
                    </TableCell>
                    <TableCell>
                      {analysis ? count(analysis.metadataIssues) : '—'}
                    </TableCell>
                    <TableCell>
                      {analysis ? count(analysis.duplicateSongs) : '—'}
                    </TableCell>
                    <TableCell>
                      {analysis ? count(analysis.loudnessIssues) : '—'}
                    </TableCell>
                    <TableCell>
                      {analysis ? count(analysis.badFitSongs) : '—'}
                    </TableCell>
                    <TableCell>
                      {analysis ? count(analysis.recommendations) : '—'}
                    </TableCell>
                    <TableCell>
                      <Box className={classes.actions}>
                        <Button
                          size="small"
                          color="primary"
                          variant="outlined"
                          startIcon={
                            isAnalyzing ? (
                              <CircularProgress size={14} />
                            ) : (
                              <AssessmentOutlinedIcon />
                            )
                          }
                          onClick={() => analyzePlaylist(playlist)}
                          disabled={isAnalyzing}
                        >
                          {analysis
                            ? 'Re-analyze playlist'
                            : 'Analyze playlist'}
                        </Button>
                        <Button
                          size="small"
                          onClick={() => setReport(analysis)}
                          disabled={!analysis}
                        >
                          View AI report
                        </Button>
                      </Box>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog
        open={Boolean(report)}
        onClose={() => setReport(null)}
        maxWidth="lg"
        fullWidth
        aria-labelledby="playlist-ai-report-title"
      >
        <DialogTitle id="playlist-ai-report-title">
          Playlist AI report{report?.name ? ` — ${report.name}` : ''}
        </DialogTitle>
        <DialogContent dividers>
          {report ? <PlaylistReport analysis={report} /> : null}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setReport(null)}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

export default PlaylistAiToolPage
