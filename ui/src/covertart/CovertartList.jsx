import React, { useEffect, useMemo, useRef, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Grid,
  Typography,
  makeStyles,
} from '@material-ui/core'
import RefreshIcon from '@material-ui/icons/Refresh'
import {
  Datagrid,
  Filter,
  FunctionField,
  List,
  SearchInput,
  TextField,
  useListContext,
  useRefresh,
} from 'react-admin'
import { DurationField } from '../common'
import subsonic from '../subsonic'

const useStyles = makeStyles((theme) => ({
  summaryContainer: {
    marginBottom: theme.spacing(2),
  },
  metricCard: {
    height: '100%',
  },
  metricRow: {
    display: 'flex',
    justifyContent: 'space-between',
    marginTop: theme.spacing(0.5),
  },
  cardTitle: {
    fontWeight: 600,
    marginBottom: theme.spacing(1),
  },
  buttonRow: {
    display: 'flex',
    justifyContent: 'flex-end',
    marginBottom: theme.spacing(1),
  },
}))

const CovertartFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const isMissingAlbum = (record) => !record?.album || record.album === '[Unknown Album]'
const isMissingYear = (record) => {
  const year = Number(record?.year || 0)
  return !year
}
const isMissingGenre = (record) => !record?.genre || !String(record.genre).trim()
const isMissingCoverArt = (record) => !record?.hasCoverArt

const buildMetrics = (records, missingFn, baselineMissing = null) => {
  const totalSongs = records.length
  const missing = records.filter(missingFn).length
  const initialMissing = baselineMissing == null ? missing : baselineMissing
  const updated = Math.max(0, initialMissing - missing)
  return {
    totalSongs,
    missing,
    totalUpdated: updated,
    totalLeft: missing,
    missingAfterUpdate: missing,
  }
}

const MetricCard = ({ title, metrics, missingLabel }) => {
  const classes = useStyles()

  return (
    <Card className={classes.metricCard}>
      <CardContent>
        <Typography variant="subtitle1" className={classes.cardTitle}>
          {title}
        </Typography>
        <Box className={classes.metricRow}>
          <Typography variant="body2">Total songs</Typography>
          <Typography variant="body2">{metrics.totalSongs}</Typography>
        </Box>
        <Box className={classes.metricRow}>
          <Typography variant="body2">{missingLabel}</Typography>
          <Typography variant="body2">{metrics.missing}</Typography>
        </Box>
        <Box className={classes.metricRow}>
          <Typography variant="body2">Total updated (live)</Typography>
          <Typography variant="body2">{metrics.totalUpdated}</Typography>
        </Box>
        <Box className={classes.metricRow}>
          <Typography variant="body2">Total left</Typography>
          <Typography variant="body2">{metrics.totalLeft}</Typography>
        </Box>
        <Box className={classes.metricRow}>
          <Typography variant="body2">Missing after update</Typography>
          <Typography variant="body2">{metrics.missingAfterUpdate}</Typography>
        </Box>
      </CardContent>
    </Card>
  )
}

const CovertartSummary = () => {
  const classes = useStyles()
  const refresh = useRefresh()
  const { ids, data } = useListContext()
  const [baseline, setBaseline] = useState(null)
  const hasInitializedBaseline = useRef(false)

  const records = useMemo(() => {
    if (!ids || !data) return []
    return ids.map((id) => data[id]).filter(Boolean)
  }, [ids, data])

  const albumMissingNow = useMemo(
    () => records.filter(isMissingAlbum).length,
    [records],
  )
  const yearMissingNow = useMemo(() => records.filter(isMissingYear).length, [records])
  const genreMissingNow = useMemo(
    () => records.filter(isMissingGenre).length,
    [records],
  )
  const coverMissingNow = useMemo(
    () => records.filter(isMissingCoverArt).length,
    [records],
  )

  useEffect(() => {
    if (hasInitializedBaseline.current || records.length === 0) {
      return
    }
    setBaseline({
      album: albumMissingNow,
      year: yearMissingNow,
      genre: genreMissingNow,
      coverart: coverMissingNow,
    })
    hasInitializedBaseline.current = true
  }, [albumMissingNow, coverMissingNow, genreMissingNow, records.length, yearMissingNow])

  const metrics = useMemo(() => {
    return {
      coverart: buildMetrics(records, isMissingCoverArt, baseline?.coverart ?? null),
      year: buildMetrics(records, isMissingYear, baseline?.year ?? null),
      album: buildMetrics(records, isMissingAlbum, baseline?.album ?? null),
      genre: buildMetrics(records, isMissingGenre, baseline?.genre ?? null),
    }
  }, [baseline, records])

  const handleFetch = () => {
    if (baseline == null) {
      setBaseline({
        album: albumMissingNow,
        year: yearMissingNow,
        genre: genreMissingNow,
        coverart: coverMissingNow,
      })
      hasInitializedBaseline.current = true
    }
    refresh()
  }

  return (
    <Box className={classes.summaryContainer}>
      <Box className={classes.buttonRow}>
        <Button
          variant="contained"
          color="primary"
          onClick={handleFetch}
          startIcon={<RefreshIcon />}
        >
          Fetch Phase 1 Updates
        </Button>
      </Box>
      <Grid container spacing={2}>
        <Grid item xs={12} md={3}>
          <MetricCard
            title="Cover Art"
            metrics={metrics.coverart}
            missingLabel="Missing cover art"
          />
        </Grid>
        <Grid item xs={12} md={3}>
          <MetricCard
            title="Year"
            metrics={metrics.year}
            missingLabel="Missing year"
          />
        </Grid>
        <Grid item xs={12} md={3}>
          <MetricCard
            title="Album"
            metrics={metrics.album}
            missingLabel="Missing album"
          />
        </Grid>
        <Grid item xs={12} md={3}>
          <MetricCard
            title="Genre"
            metrics={metrics.genre}
            missingLabel="Missing genre"
          />
        </Grid>
      </Grid>
    </Box>
  )
}

const CovertartContent = (props) => (
  <>
    <CovertartSummary />
    <Datagrid rowClick={false} {...props}>
      <FunctionField
        label="Cover Art"
        sortable={false}
        render={(record) => (
          <img
            src={subsonic.getCoverArtUrl(record, 64, true)}
            alt={record.title || 'cover art'}
            width="64"
            height="64"
          />
        )}
      />
      <TextField source="title" />
      <TextField source="artist" label="Artist" />
      <TextField source="album" label="Album" />
      <TextField source="year" label="Release Year" />
      <TextField source="genre" label="Genre" />
      <FunctionField
        label="Metadata Phase"
        sortable={false}
        render={() => 'Phase 1'}
      />
      <DurationField source="duration" />
    </Datagrid>
  </>
)

const CovertartList = (props) => (
  <List
    {...props}
    sort={{ field: 'title', order: 'ASC' }}
    filters={<CovertartFilter />}
    exporter={false}
    bulkActionButtons={false}
    perPage={50}
  >
    <CovertartContent />
  </List>
)

export default CovertartList
