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
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(1.5),
  },
  actions: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(1),
    marginTop: theme.spacing(1.5),
  },
  error: {
    color: theme.palette.error.main,
    marginTop: theme.spacing(1),
  },
  stale: {
    marginTop: theme.spacing(1.5),
    padding: theme.spacing(1.5),
    borderRadius: 6,
    border: `1px solid ${theme.palette.warning.main}`,
    color: theme.palette.warning.main,
  },
  diffGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
    gap: theme.spacing(1.5),
    marginTop: theme.spacing(1.5),
  },
  added: { color: '#43a047' },
  removed: { color: theme.palette.error.main },
  moved: { color: '#fb8c00' },
  draftRow: { cursor: 'pointer' },
  selectedRow: {
    background:
      theme.palette.type === 'dark'
        ? 'rgba(255,255,255,0.08)'
        : 'rgba(0,0,0,0.05)',
  },
}))

// A draft is only publishable once a reviewer has approved it, so the buttons
// mirror the server's state machine rather than offering every action always.
const STATUS_LABEL = {
  draft: 'Draft',
  ready_for_review: 'Ready for review',
  approved: 'Approved',
  published: 'Published',
  discarded: 'Discarded',
  conflicted: 'Conflicted',
}

const errorMessage = (error, fallback) =>
  error?.body?.error || error?.message || fallback

const DiffList = ({ title, entries, className }) => (
  <Box>
    <Typography variant="subtitle2" className={className}>
      {title} ({entries.length})
    </Typography>
    {entries.length ? (
      <ul>
        {entries.map((entry) => (
          <li key={`${entry.mediaFileId}-${entry.position}`}>
            <Typography variant="body2" component="span">
              {entry.title || entry.mediaFileId}
              {entry.artist ? ` — ${entry.artist}` : ''}
            </Typography>
            {entry.reason ? (
              <Typography
                variant="caption"
                color="textSecondary"
                component="div"
              >
                {entry.reason}
              </Typography>
            ) : null}
          </li>
        ))}
      </ul>
    ) : (
      <Typography variant="body2" color="textSecondary">
        None
      </Typography>
    )}
  </Box>
)

DiffList.propTypes = {
  title: PropTypes.string.isRequired,
  entries: PropTypes.array.isRequired,
  className: PropTypes.string,
}

const PlaylistDraftPanel = ({
  playlist,
  refreshToken,
  preferredDraftId,
  onDraftSelection,
  onDraftChanged,
}) => {
  const classes = useStyles()
  const [drafts, setDrafts] = useState([])
  const [selectedId, setSelectedId] = useState('')
  const [diff, setDiff] = useState(null)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')

  const playlistId = playlist?.id || ''

  const loadDrafts = useCallback(async () => {
    if (!playlistId) {
      setDrafts([])
      return
    }
    try {
      const { json } = await httpClient(
        `/api/playlist-draft?playlistId=${encodeURIComponent(playlistId)}`,
      )
      const nextDrafts = json?.drafts || []
      setDrafts(nextDrafts)
      setSelectedId((current) => {
        if (nextDrafts.some((draft) => draft.id === current)) return current
        return (
          nextDrafts.find((draft) => draft.status === 'draft')?.id ||
          nextDrafts[0]?.id ||
          ''
        )
      })
    } catch (err) {
      setError(errorMessage(err, 'Could not load playlist drafts'))
    }
  }, [playlistId])

  useEffect(() => {
    setDrafts([])
    setSelectedId('')
    setDiff(null)
  }, [playlistId])

  useEffect(() => {
    void loadDrafts()
  }, [loadDrafts, refreshToken])

  // The diff is always fetched from the server rather than computed here, so
  // the staleness the reviewer sees is the server's own judgement.
  const loadDiff = useCallback(async (draftId) => {
    setDiff(null)
    if (!draftId) {
      return
    }
    try {
      const { json } = await httpClient(
        `/api/playlist-draft/${encodeURIComponent(draftId)}/diff`,
      )
      setDiff(json)
    } catch (err) {
      setDiff(null)
      setError(errorMessage(err, 'Could not load the draft diff'))
    }
  }, [])

  useEffect(() => {
    void loadDiff(selectedId)
  }, [loadDiff, selectedId, refreshToken])

  useEffect(() => {
    if (
      preferredDraftId &&
      drafts.some((draft) => draft.id === preferredDraftId)
    ) {
      setSelectedId(preferredDraftId)
    }
  }, [drafts, preferredDraftId])

  const selected = drafts.find((draft) => draft.id === selectedId) || null

  useEffect(() => {
    onDraftSelection?.(selected, diff)
  }, [diff, onDraftSelection, selected])

  const runAction = async (action, request) => {
    setError('')
    setBusy(action)
    try {
      await request()
      await loadDrafts()
      await loadDiff(selectedId)
    } catch (err) {
      setError(errorMessage(err, `Could not ${action} the draft`))
      // A refused publish still changes server state (the draft becomes
      // conflicted), so the view is reloaded even on failure.
      await loadDrafts()
      await loadDiff(selectedId)
    } finally {
      setBusy('')
      onDraftChanged?.()
    }
  }

  const setStatus = (status, label) =>
    runAction(label, () =>
      httpClient(
        `/api/playlist-draft/${encodeURIComponent(selectedId)}/status`,
        {
          method: 'POST',
          body: JSON.stringify({ status }),
        },
      ),
    )

  const publish = () =>
    runAction('publish', () =>
      httpClient(
        `/api/playlist-draft/${encodeURIComponent(selectedId)}/publish`,
        { method: 'POST' },
      ),
    )

  if (!playlistId) {
    return null
  }

  const status = selected?.status
  return (
    <Card className={classes.panel} variant="outlined">
      <CardContent>
        <Box className={classes.header}>
          <Box>
            <Typography variant="h6">Playlist drafts</Typography>
            <Typography variant="body2" color="textSecondary">
              Proposed changes for {playlist.name}. Nothing reaches the live
              playlist until a draft is approved and published.
            </Typography>
          </Box>
          {busy ? <CircularProgress size={24} /> : null}
        </Box>

        {drafts.length ? (
          <TableContainer>
            <Table size="small" aria-label="Playlist drafts">
              <TableHead>
                <TableRow>
                  {['Name', 'Status', 'Created by', 'Tracks', 'Changes'].map(
                    (label) => (
                      <TableCell key={label}>{label}</TableCell>
                    ),
                  )}
                </TableRow>
              </TableHead>
              <TableBody>
                {drafts.map((draft) => (
                  <TableRow
                    key={draft.id}
                    hover
                    className={`${classes.draftRow} ${
                      draft.id === selectedId ? classes.selectedRow : ''
                    }`}
                    onClick={() => setSelectedId(draft.id)}
                  >
                    <TableCell>{draft.name || 'Untitled draft'}</TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        variant="outlined"
                        label={STATUS_LABEL[draft.status] || draft.status}
                      />
                    </TableCell>
                    <TableCell>{draft.createdBy || '—'}</TableCell>
                    <TableCell>{draft.proposedTrackIds?.length ?? 0}</TableCell>
                    <TableCell>{draft.changes?.length ?? 0}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        ) : (
          <Typography variant="body2" color="textSecondary">
            No drafts yet. Create a draft before accepting an AI recommendation.
          </Typography>
        )}

        {error ? (
          <Typography className={classes.error} variant="body2">
            {error}
          </Typography>
        ) : null}

        {selected ? (
          <>
            {diff?.stale ? (
              <Box className={classes.stale}>
                <Typography variant="body2">
                  The live playlist changed after this draft was created.
                  Publishing is blocked. Reopen the draft to rebuild it against
                  the current playlist.
                </Typography>
              </Box>
            ) : null}
            {selected.conflictDetail ? (
              <Typography className={classes.error} variant="body2">
                {selected.conflictDetail}
              </Typography>
            ) : null}

            {diff ? (
              <Box className={classes.diffGrid}>
                <DiffList
                  title="Added"
                  entries={diff.added || []}
                  className={classes.added}
                />
                <DiffList
                  title="Removed"
                  entries={diff.removed || []}
                  className={classes.removed}
                />
                <DiffList
                  title="Moved"
                  entries={diff.moved || []}
                  className={classes.moved}
                />
                <Box>
                  <Typography variant="subtitle2">Unchanged</Typography>
                  <Typography variant="body2" color="textSecondary">
                    {diff.unchanged ?? 0} tracks
                  </Typography>
                  <Typography variant="caption" color="textSecondary">
                    {diff.before?.length ?? 0} before →{' '}
                    {diff.after?.length ?? 0} after
                  </Typography>
                </Box>
              </Box>
            ) : null}

            <Box className={classes.actions}>
              <Button
                variant="outlined"
                onClick={() => setStatus('ready_for_review', 'submit')}
                disabled={Boolean(busy) || status !== 'draft'}
              >
                Submit for review
              </Button>
              <Button
                variant="outlined"
                color="primary"
                onClick={() => setStatus('approved', 'approve')}
                disabled={Boolean(busy) || status !== 'ready_for_review'}
              >
                Approve
              </Button>
              <Button
                variant="contained"
                color="primary"
                onClick={publish}
                disabled={
                  Boolean(busy) || status !== 'approved' || Boolean(diff?.stale)
                }
              >
                Publish to playlist
              </Button>
              <Button
                variant="outlined"
                onClick={() => setStatus('draft', 'reopen')}
                disabled={
                  Boolean(busy) ||
                  !['ready_for_review', 'approved', 'conflicted'].includes(
                    status,
                  )
                }
              >
                Reject / reopen
              </Button>
              <Button
                variant="outlined"
                onClick={() => setStatus('discarded', 'discard')}
                disabled={
                  Boolean(busy) ||
                  ['published', 'discarded'].includes(status || '')
                }
              >
                Discard
              </Button>
            </Box>
          </>
        ) : null}
      </CardContent>
    </Card>
  )
}

PlaylistDraftPanel.propTypes = {
  playlist: PropTypes.shape({
    id: PropTypes.string,
    name: PropTypes.string,
  }),
  refreshToken: PropTypes.number,
  preferredDraftId: PropTypes.string,
  onDraftSelection: PropTypes.func,
  onDraftChanged: PropTypes.func,
}

export default PlaylistDraftPanel
