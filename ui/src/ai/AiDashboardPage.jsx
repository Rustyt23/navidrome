import { useCallback, useEffect, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  LinearProgress,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
  makeStyles,
} from '@material-ui/core'
import RefreshIcon from '@material-ui/icons/Refresh'
import GetAppIcon from '@material-ui/icons/GetApp'
import PropTypes from 'prop-types'
import { Title } from 'react-admin'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(2.5),
    color: theme.palette.text.primary,
  },
  header: {
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    flexWrap: 'wrap',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(2.5),
  },
  subtitle: {
    marginTop: theme.spacing(0.5),
    color: theme.palette.text.secondary,
  },
  actions: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(1),
    '& .MuiButton-root': {
      minHeight: 34,
      padding: theme.spacing(0.5, 1.25),
      borderRadius: 8,
      fontSize: 12,
      fontWeight: 600,
      textTransform: 'none',
    },
  },
  summaryGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
    gap: theme.spacing(1.5),
    marginBottom: theme.spacing(2),
  },
  summaryCard: {
    minHeight: 126,
    border: '1px solid rgba(255, 255, 255, 0.10)',
    borderRadius: 12,
    background:
      theme.palette.type === 'dark'
        ? '#151f2d'
        : theme.palette.background.paper,
    boxShadow: '0 10px 30px rgba(0, 0, 0, 0.12)',
  },
  summaryLabel: {
    color: theme.palette.text.secondary,
    fontSize: 12,
    fontWeight: 600,
    letterSpacing: 0.3,
    textTransform: 'uppercase',
  },
  summaryValue: {
    margin: theme.spacing(0.75, 0),
    color:
      theme.palette.type === 'dark' ? '#f7f8fb' : theme.palette.text.primary,
    fontSize: 30,
    fontWeight: 700,
    lineHeight: 1.1,
  },
  dashboardGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(330px, 1fr))',
    gap: theme.spacing(1.5),
  },
  panel: {
    border: '1px solid rgba(255, 255, 255, 0.10)',
    borderRadius: 12,
    background:
      theme.palette.type === 'dark'
        ? '#151f2d'
        : theme.palette.background.paper,
    boxShadow: 'none',
  },
  panelHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(1.5),
  },
  panelTitle: {
    fontWeight: 650,
  },
  progressRow: {
    marginBottom: theme.spacing(1.25),
  },
  progressMeta: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: theme.spacing(1),
    marginBottom: theme.spacing(0.5),
    color: theme.palette.text.secondary,
    fontSize: 12,
  },
  progress: {
    height: 7,
    borderRadius: 8,
    backgroundColor: 'rgba(255, 255, 255, 0.10)',
    '& .MuiLinearProgress-bar': {
      borderRadius: 8,
      backgroundColor: '#ff2a8e',
    },
  },
  metricTable: {
    '& .MuiTableCell-root': {
      padding: theme.spacing(0.75, 0.5),
      borderBottom: '1px solid rgba(255, 255, 255, 0.07)',
      fontSize: 12,
    },
  },
  banner: {
    marginBottom: theme.spacing(2),
    padding: theme.spacing(1.25, 1.5),
    borderRadius: 9,
    border: '1px solid rgba(255, 203, 107, 0.35)',
    color: '#ffcb6b',
    background: 'rgba(255, 203, 107, 0.08)',
  },
  empty: {
    padding: theme.spacing(5),
    textAlign: 'center',
    borderRadius: 12,
    color: theme.palette.text.secondary,
    background:
      theme.palette.type === 'dark'
        ? '#151f2d'
        : theme.palette.background.paper,
  },
  error: {
    padding: theme.spacing(3),
    borderRadius: 12,
    textAlign: 'center',
    color: '#ff8fc6',
    background: 'rgba(255, 42, 142, 0.08)',
  },
  loading: {
    minHeight: 300,
    display: 'grid',
    placeItems: 'center',
  },
}))

const clampPercent = (value) => Math.max(0, Math.min(100, Number(value) || 0))
const coverage = (withValue, total) =>
  total > 0 ? Math.round((Number(withValue || 0) / total) * 1000) / 10 : 0

const statusFor = (value, inverse = false) => {
  const normalized = inverse ? 100 - clampPercent(value) : clampPercent(value)
  if (normalized >= 80) return { label: 'Good', color: '#3ddc84' }
  if (normalized >= 50) return { label: 'Warning', color: '#ffcb6b' }
  return { label: 'Critical', color: '#ff5c8a' }
}

const StatusChip = ({ value, inverse = false, label }) => {
  const status = label
    ? { label, color: label === 'Good' ? '#3ddc84' : '#ffcb6b' }
    : statusFor(value, inverse)
  return (
    <Chip
      size="small"
      label={status.label}
      style={{
        color: status.color,
        borderColor: status.color,
        background: `${status.color}12`,
      }}
      variant="outlined"
    />
  )
}

StatusChip.propTypes = {
  value: PropTypes.number,
  inverse: PropTypes.bool,
  label: PropTypes.string,
}

const SummaryCard = ({
  label,
  value,
  suffix = '',
  statusValue,
  inverse = false,
}) => {
  const classes = useStyles()
  return (
    <Card className={classes.summaryCard}>
      <CardContent>
        <Typography className={classes.summaryLabel}>{label}</Typography>
        <Typography className={classes.summaryValue}>
          {value}
          {suffix}
        </Typography>
        <StatusChip value={statusValue ?? Number(value)} inverse={inverse} />
      </CardContent>
    </Card>
  )
}

SummaryCard.propTypes = {
  label: PropTypes.string.isRequired,
  value: PropTypes.oneOfType([PropTypes.number, PropTypes.string]).isRequired,
  suffix: PropTypes.string,
  statusValue: PropTypes.number,
  inverse: PropTypes.bool,
}

const Panel = ({ title, statusValue, inverse = false, children }) => {
  const classes = useStyles()
  return (
    <Card className={classes.panel}>
      <CardContent>
        <Box className={classes.panelHeader}>
          <Typography className={classes.panelTitle} variant="subtitle1">
            {title}
          </Typography>
          <StatusChip value={statusValue} inverse={inverse} />
        </Box>
        {children}
      </CardContent>
    </Card>
  )
}

Panel.propTypes = {
  title: PropTypes.string.isRequired,
  statusValue: PropTypes.number.isRequired,
  inverse: PropTypes.bool,
  children: PropTypes.node.isRequired,
}

const CoverageRow = ({ label, value, total }) => {
  const classes = useStyles()
  const percent = coverage(value, total)
  return (
    <Box className={classes.progressRow}>
      <Box className={classes.progressMeta}>
        <span>{label}</span>
        <span>
          {value}/{total} · {percent}%
        </span>
      </Box>
      <LinearProgress
        className={classes.progress}
        variant="determinate"
        value={percent}
      />
    </Box>
  )
}

CoverageRow.propTypes = {
  label: PropTypes.string.isRequired,
  value: PropTypes.number.isRequired,
  total: PropTypes.number.isRequired,
}

const MetricsTable = ({ rows }) => {
  const classes = useStyles()
  return (
    <Table size="small" className={classes.metricTable}>
      <TableHead>
        <TableRow>
          <TableCell>Metric</TableCell>
          <TableCell align="right">Count</TableCell>
        </TableRow>
      </TableHead>
      <TableBody>
        {rows.map(([label, value]) => (
          <TableRow key={label}>
            <TableCell>{label}</TableCell>
            <TableCell align="right">{value}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

MetricsTable.propTypes = {
  rows: PropTypes.arrayOf(PropTypes.array).isRequired,
}

const apiErrorMessage = (error) =>
  error?.body?.error ||
  error?.json?.error ||
  error?.message ||
  'Could not load AI dashboard'

const exportEntries = (data) =>
  Object.entries(data).filter(([, value]) =>
    ['string', 'number', 'boolean'].includes(typeof value),
  )

const exportDashboard = (data, format) => {
  const json = format === 'json'
  const content = json
    ? JSON.stringify(data, null, 2)
    : [
        'metric,value',
        ...exportEntries(data).map(
          ([key, value]) => `${JSON.stringify(key)},${JSON.stringify(value)}`,
        ),
      ].join('\n')
  const blob = new Blob([content], {
    type: json ? 'application/json' : 'text/csv;charset=utf-8',
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `ai-dashboard.${format}`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

const AiDashboardPage = () => {
  const classes = useStyles()
  const [report, setReport] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const loadDashboard = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const { json } = await httpClient('/api/ai/rag/reports/dashboard')
      setReport(json)
    } catch (loadError) {
      setError(apiErrorMessage(loadError))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadDashboard()
  }, [loadDashboard])

  if (loading && !report) {
    return (
      <Box className={classes.root}>
        <Title title="AI Dashboard" />
        <Box className={classes.loading}>
          <CircularProgress />
        </Box>
      </Box>
    )
  }

  if (error && !report) {
    return (
      <Box className={classes.root}>
        <Title title="AI Dashboard" />
        <Box className={classes.error}>
          <Typography variant="h6">Dashboard unavailable</Typography>
          <Typography variant="body2" paragraph>
            {error}
          </Typography>
          <Button variant="outlined" color="primary" onClick={loadDashboard}>
            Try again
          </Button>
        </Box>
      </Box>
    )
  }

  const total = Number(report?.totalSongs) || 0
  const playlistIssuePercent = coverage(
    report?.playlistsWithIssues,
    report?.playlistsTotal,
  )
  const metadataAverage =
    (coverage(report?.songsWithGenre, total) +
      coverage(report?.songsWithYear, total) +
      coverage(report?.songsWithBpm, total) +
      coverage(report?.songsWithLufs, total)) /
    4

  return (
    <Box className={classes.root}>
      <Title title="AI Dashboard" />
      <Box className={classes.header}>
        <Box>
          <Typography variant="h5">AI Dashboard</Typography>
          <Typography className={classes.subtitle} variant="body2">
            Read-only health, quality, risk, and recommendation insights for
            your library.
          </Typography>
        </Box>
        <Box className={classes.actions}>
          <Button
            variant="outlined"
            color="primary"
            startIcon={<RefreshIcon />}
            onClick={loadDashboard}
            disabled={loading}
          >
            {loading ? 'Refreshing…' : 'Refresh Dashboard'}
          </Button>
          <Button
            variant="outlined"
            color="primary"
            startIcon={<GetAppIcon />}
            onClick={() => exportDashboard(report, 'json')}
          >
            Export JSON
          </Button>
          <Button
            variant="outlined"
            color="primary"
            startIcon={<GetAppIcon />}
            onClick={() => exportDashboard(report, 'csv')}
          >
            Export CSV
          </Button>
        </Box>
      </Box>

      {!report.ragEnabled ||
      !report.vectorDbOnline ||
      !report.collectionExists ? (
        <Typography className={classes.banner} component="div">
          {!report.ragEnabled
            ? 'RAG is disabled. Indexed coverage is shown as 0%.'
            : report.ragError ||
              'Qdrant is offline or the collection is unavailable. Indexed coverage is shown as 0%.'}
        </Typography>
      ) : null}
      {error ? (
        <Typography className={classes.banner}>{error}</Typography>
      ) : null}

      <Box className={classes.summaryGrid}>
        <SummaryCard
          label="Library Health Score"
          value={report.libraryHealthScore || 0}
          suffix="/100"
          statusValue={report.libraryHealthScore || 0}
        />
        <SummaryCard
          label="Total Songs"
          value={total}
          statusValue={total ? 100 : 0}
        />
        <SummaryCard
          label="RAG Indexed Songs"
          value={report.indexedSongs || 0}
          statusValue={report.indexedCoveragePercent || 0}
        />
        <SummaryCard
          label="Indexed Coverage"
          value={Number(report.indexedCoveragePercent || 0).toFixed(1)}
          suffix="%"
          statusValue={report.indexedCoveragePercent || 0}
        />
        <SummaryCard
          label="Playlists With Issues"
          value={report.playlistsWithIssues || 0}
          statusValue={playlistIssuePercent}
          inverse
        />
      </Box>

      {total === 0 ? (
        <Box className={classes.empty}>
          <Typography variant="h6">Your library is empty</Typography>
          <Typography variant="body2">
            Add and scan music to populate dashboard health metrics.
          </Typography>
        </Box>
      ) : (
        <Box className={classes.dashboardGrid}>
          <Panel
            title="RAG Index Coverage"
            statusValue={report.indexedCoveragePercent || 0}
          >
            <CoverageRow
              label="Indexed songs"
              value={Number(report.indexedSongs) || 0}
              total={total}
            />
            <MetricsTable
              rows={[
                ['RAG enabled', report.ragEnabled ? 'Yes' : 'No'],
                [
                  'Vector database',
                  report.vectorDbOnline ? 'Online' : 'Offline',
                ],
                [
                  'Collection',
                  report.collectionExists ? 'Available' : 'Unavailable',
                ],
              ]}
            />
          </Panel>

          <Panel title="Metadata Quality" statusValue={metadataAverage}>
            <CoverageRow
              label="Genre"
              value={report.songsWithGenre || 0}
              total={total}
            />
            <CoverageRow
              label="Year"
              value={report.songsWithYear || 0}
              total={total}
            />
            <CoverageRow
              label="BPM"
              value={report.songsWithBpm || 0}
              total={total}
            />
            <CoverageRow
              label="LUFS"
              value={report.songsWithLufs || 0}
              total={total}
            />
          </Panel>

          <Panel
            title="Lyrics Coverage"
            statusValue={coverage(report.songsWithLyrics, total)}
          >
            <CoverageRow
              label="Songs with lyrics"
              value={report.songsWithLyrics || 0}
              total={total}
            />
            <MetricsTable
              rows={[
                ['Available', report.songsWithLyrics || 0],
                ['Missing', report.songsMissingLyrics || 0],
              ]}
            />
          </Panel>

          <Panel
            title="Explicit / Retail Risk"
            statusValue={coverage(
              (report.explicitSongs || 0) + (report.reviewNeededSongs || 0),
              total,
            )}
            inverse
          >
            <MetricsTable
              rows={[
                ['Clean songs', report.cleanSongs || 0],
                ['Explicit songs', report.explicitSongs || 0],
                ['Review needed', report.reviewNeededSongs || 0],
                ['Explicit risk', report.explicitRiskSongs || 0],
              ]}
            />
          </Panel>

          <Panel
            title="Playlist Quality"
            statusValue={playlistIssuePercent}
            inverse
          >
            <MetricsTable
              rows={[
                ['Total playlists', report.playlistsTotal || 0],
                ['Analyzed', report.playlistsAnalyzed || 0],
                ['With issues', report.playlistsWithIssues || 0],
                ['Duplicate risks', report.duplicateRiskCount || 0],
              ]}
            />
          </Panel>

          <Panel
            title="Audio Quality"
            statusValue={coverage(report.songsWithLufs, total)}
          >
            <CoverageRow
              label="Songs with LUFS"
              value={report.songsWithLufs || 0}
              total={total}
            />
            <MetricsTable
              rows={[
                ['Missing LUFS', report.songsMissingLufs || 0],
                ['Playlist loudness issues', report.loudnessIssueCount || 0],
              ]}
            />
          </Panel>

          <Panel
            title="Usage Insights"
            statusValue={coverage(report.overplayedSongs, total)}
            inverse
          >
            <MetricsTable
              rows={[
                ['Underused songs', report.underusedSongs || 0],
                ['Overplayed songs', report.overplayedSongs || 0],
              ]}
            />
          </Panel>

          <Panel
            title="Recommendation Summary"
            statusValue={report.recommendationsAvailable ? 70 : 100}
          >
            <Typography variant="h4">
              {report.recommendationsAvailable || 0}
            </Typography>
            <Typography variant="body2" color="textSecondary">
              Read-only recommendations currently available from metadata, risk,
              and playlist-fit signals.
            </Typography>
          </Panel>
        </Box>
      )}
    </Box>
  )
}

export default AiDashboardPage
