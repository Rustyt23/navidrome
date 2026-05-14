import { useMemo, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItem,
  ListItemText,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
  makeStyles,
} from '@material-ui/core'
import { Title, useDataProvider, useTranslate } from 'react-admin'

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(2),
  },
  section: {
    marginBottom: theme.spacing(2),
  },
  chatHistory: {
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: theme.shape.borderRadius,
    minHeight: 240,
    maxHeight: 360,
    overflowY: 'auto',
  },
  inputRow: {
    display: 'flex',
    gap: theme.spacing(1),
    marginTop: theme.spacing(1),
  },
  tableWrap: {
    marginTop: theme.spacing(2),
    overflowX: 'auto',
  },
}))

const formatDuration = (seconds) => {
  const total = Number(seconds)
  if (!Number.isFinite(total) || total <= 0) return ''
  const mins = Math.floor(total / 60)
  const secs = Math.floor(total % 60)
  return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
}

const AiToolPage = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const dataProvider = useDataProvider()
  const [messages, setMessages] = useState([])
  const [prompt, setPrompt] = useState('')
  const [songDialogOpen, setSongDialogOpen] = useState(false)
  const [songsLoading, setSongsLoading] = useState(false)
  const [availableSongs, setAvailableSongs] = useState([])
  const [selectedSongIds, setSelectedSongIds] = useState([])
  const [addedSongs, setAddedSongs] = useState([])

  const selectedSongs = useMemo(
    () => availableSongs.filter((song) => selectedSongIds.includes(song.id)),
    [availableSongs, selectedSongIds],
  )

  const sendMessage = () => {
    const trimmed = prompt.trim()
    if (!trimmed) return

    setMessages((prev) => [
      ...prev,
      { role: 'user', text: trimmed },
      {
        role: 'assistant',
        text: translate('menu.aiTool.placeholderReply', {
          _: 'AI integration is not connected yet. This is a UI placeholder.',
        }),
      },
    ])
    setPrompt('')
  }

  const openAddSongsDialog = async () => {
    setSongDialogOpen(true)
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
    setSelectedSongIds((prev) =>
      prev.includes(songId) ? prev.filter((id) => id !== songId) : [...prev, songId],
    )
  }

  const addSelectedSongs = () => {
    setAddedSongs((prev) => {
      const existing = new Set(prev.map((song) => song.id))
      const uniqueNew = selectedSongs.filter((song) => !existing.has(song.id))
      return [...prev, ...uniqueNew]
    })
    setSongDialogOpen(false)
  }

  return (
    <Box className={classes.root}>
      <Title title={translate('menu.aiTool.name', { _: 'AI Tool' })} />

      <Card className={classes.section}>
        <CardContent>
          <Typography variant="h6">
            {translate('menu.aiTool.chat', { _: 'Chat with AI' })}
          </Typography>
          <List className={classes.chatHistory}>
            {messages.length === 0 ? (
              <ListItem>
                <ListItemText
                  primary={translate('menu.aiTool.empty', {
                    _: 'Start a conversation with AI.',
                  })}
                />
              </ListItem>
            ) : (
              messages.map((message, index) => (
                <ListItem key={`${message.role}-${index}`}>
                  <ListItemText
                    primary={`${message.role === 'user' ? 'You' : 'AI'}: ${message.text}`}
                  />
                </ListItem>
              ))
            )}
          </List>
          <Box className={classes.inputRow}>
            <TextField
              fullWidth
              variant="outlined"
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              onKeyPress={(event) => {
                if (event.key === 'Enter') sendMessage()
              }}
              placeholder={translate('menu.aiTool.inputPlaceholder', {
                _: 'Ask AI anything...',
              })}
            />
            <Button variant="contained" color="primary" onClick={sendMessage}>
              {translate('menu.aiTool.send', { _: 'Send' })}
            </Button>
          </Box>
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <Typography variant="h6">
            {translate('menu.aiTool.importSong', { _: 'Import song' })}
          </Typography>
          <Button variant="outlined" color="primary" onClick={openAddSongsDialog}>
            {translate('menu.aiTool.addSongs', { _: 'Add songs' })}
          </Button>

          <Box className={classes.tableWrap}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{translate('resources.song.fields.title', { _: 'Title' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.album', { _: 'Album' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.artist', { _: 'Artist' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.year', { _: 'Year' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.explicitStatus', { _: 'Explicit' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.duration', { _: 'Time' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.genre', { _: 'Genre' })}</TableCell>
                  <TableCell>{translate('menu.aiTool.aiTag', { _: 'AI Tag' })}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {addedSongs.map((song) => (
                  <TableRow key={song.id}>
                    <TableCell>{song.title || ''}</TableCell>
                    <TableCell>{song.album || ''}</TableCell>
                    <TableCell>{song.artist || ''}</TableCell>
                    <TableCell>{song.year || ''}</TableCell>
                    <TableCell>{song.explicitStatus || ''}</TableCell>
                    <TableCell>{formatDuration(song.duration)}</TableCell>
                    <TableCell>{song.genre || ''}</TableCell>
                    <TableCell>{song.aiTag || '-'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Box>
        </CardContent>
      </Card>

      <Dialog
        open={songDialogOpen}
        onClose={() => setSongDialogOpen(false)}
        fullWidth
        maxWidth="lg"
      >
        <DialogTitle>{translate('menu.aiTool.addSongs', { _: 'Add songs' })}</DialogTitle>
        <DialogContent>
          {songsLoading ? (
            <Typography>{translate('menu.retailPlayer.loading', { _: 'Loading devices…' })}</Typography>
          ) : (
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell padding="checkbox" />
                  <TableCell>{translate('resources.song.fields.title', { _: 'Title' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.album', { _: 'Album' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.artist', { _: 'Artist' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.year', { _: 'Year' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.explicitStatus', { _: 'Explicit' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.duration', { _: 'Time' })}</TableCell>
                  <TableCell>{translate('resources.song.fields.genre', { _: 'Genre' })}</TableCell>
                  <TableCell>{translate('menu.aiTool.aiTag', { _: 'AI Tag' })}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {availableSongs.map((song) => (
                  <TableRow key={song.id} hover onClick={() => toggleSong(song.id)}>
                    <TableCell padding="checkbox">
                      <Checkbox checked={selectedSongIds.includes(song.id)} />
                    </TableCell>
                    <TableCell>{song.title || ''}</TableCell>
                    <TableCell>{song.album || ''}</TableCell>
                    <TableCell>{song.artist || ''}</TableCell>
                    <TableCell>{song.year || ''}</TableCell>
                    <TableCell>{song.explicitStatus || ''}</TableCell>
                    <TableCell>{formatDuration(song.duration)}</TableCell>
                    <TableCell>{song.genre || ''}</TableCell>
                    <TableCell>{song.aiTag || '-'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSongDialogOpen(false)}>
            {translate('ra.action.cancel', { _: 'Cancel' })}
          </Button>
          <Button color="primary" variant="contained" onClick={addSelectedSongs}>
            {translate('menu.aiTool.addSelected', { _: 'Add selected songs' })}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

export default AiToolPage
