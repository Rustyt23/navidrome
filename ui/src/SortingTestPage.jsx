import React, { useMemo } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import {
  Card,
  CardContent,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TableSortLabel,
  Typography,
  Paper,
} from '@material-ui/core'
import { useSmartSort, SortDirection, SortType } from './utils'

const useStyles = makeStyles((theme) => ({
  root: {
    maxWidth: 960,
    margin: '0 auto',
    padding: theme.spacing(4),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
  },
  tableContainer: {
    borderRadius: theme.shape.borderRadius,
    border: `1px solid ${theme.palette.divider}`,
  },
  tableHead: {
    backgroundColor: theme.palette.action.hover,
  },
  sortLabel: {
    fontWeight: theme.typography.fontWeightMedium,
    textTransform: 'uppercase',
    letterSpacing: 1,
  },
  description: {
    color: theme.palette.text.secondary,
  },
}))

const buildDemoData = () => [
  {
    id: 1,
    title: '  Zebra Crossing',
    plays: 12,
    lastPlayed: '2024-04-02T15:24:00Z',
  },
  {
    id: 2,
    title: 'The Avayas',
    plays: 98,
    lastPlayed: '2024-05-10T20:45:00Z',
  },
  {
    id: 3,
    title: 'afterglow',
    plays: 205,
    lastPlayed: '2023-11-21T09:14:00Z',
  },
  {
    id: 4,
    title: ' Midnight Drives',
    plays: '58',
    lastPlayed: new Date('2024-01-01T00:00:00Z'),
  },
  {
    id: 5,
    title: 'Numbers 101',
    plays: '1024',
    lastPlayed: '2022-12-12T12:00:00Z',
  },
]

const SortingTestPage = () => {
  const classes = useStyles()
  const data = useMemo(buildDemoData, [])

  const columns = useMemo(
    () => ({
      title: { type: SortType.STRING },
      plays: { type: SortType.NUMBER },
      lastPlayed: { type: SortType.DATE, defaultDirection: SortDirection.DESC },
    }),
    [],
  )

  const { sortedData, requestSort, getSortDirection } = useSmartSort(data, {
    initialKey: 'title',
    columns,
  })

  const handleSort = (key) => () => requestSort(key)

  const resolveDirection = (key) => getSortDirection(key) || SortDirection.ASC

  return (
    <div className={classes.root}>
      <Typography component="h1" variant="h4">
        Sorting Test Page
      </Typography>
      <Typography className={classes.description} variant="body1">
        Use the column headers to test text, number, and date sorting. This page
        exercises the shared smart sorting utility.
      </Typography>
      <TableContainer component={Paper} className={classes.tableContainer}>
        <Table aria-label="Sorting demonstration table" size="small">
          <TableHead className={classes.tableHead}>
            <TableRow>
              <TableCell sortDirection={getSortDirection('title')}>
                <TableSortLabel
                  active={Boolean(getSortDirection('title'))}
                  direction={resolveDirection('title')}
                  onClick={handleSort('title')}
                  className={classes.sortLabel}
                >
                  Title
                </TableSortLabel>
              </TableCell>
              <TableCell sortDirection={getSortDirection('plays')} align="right">
                <TableSortLabel
                  active={Boolean(getSortDirection('plays'))}
                  direction={resolveDirection('plays')}
                  onClick={handleSort('plays')}
                  className={classes.sortLabel}
                >
                  Plays
                </TableSortLabel>
              </TableCell>
              <TableCell sortDirection={getSortDirection('lastPlayed')}>
                <TableSortLabel
                  active={Boolean(getSortDirection('lastPlayed'))}
                  direction={resolveDirection('lastPlayed')}
                  onClick={handleSort('lastPlayed')}
                  className={classes.sortLabel}
                >
                  Last Played
                </TableSortLabel>
              </TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {sortedData.map((row) => (
              <TableRow key={row.id} hover>
                <TableCell component="th" scope="row">
                  {row.title}
                </TableCell>
                <TableCell align="right">{row.plays}</TableCell>
                <TableCell>{
                  row.lastPlayed instanceof Date
                    ? row.lastPlayed.toLocaleString()
                    : new Date(row.lastPlayed).toLocaleString()
                }</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      <Card elevation={0}>
        <CardContent>
          <Typography variant="subtitle1" gutterBottom>
            Sorting behaviours showcased
          </Typography>
          <Typography component="ul" variant="body2">
            <li>Strings are sorted case-insensitively and ignore leading spaces.</li>
            <li>Numbers are sorted by numeric value even when provided as strings.</li>
            <li>Dates are sorted chronologically regardless of formatting.</li>
            <li>Rows with identical values keep their original order (stable sort).</li>
          </Typography>
        </CardContent>
      </Card>
    </div>
  )
}

export default SortingTestPage
