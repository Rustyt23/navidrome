import { useRef, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  List,
  ListItem,
  ListItemText,
  TextField,
  Typography,
  makeStyles,
} from '@material-ui/core'
import { Title, useTranslate } from 'react-admin'

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
}))

const AiToolPage = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const [messages, setMessages] = useState([])
  const [prompt, setPrompt] = useState('')
  const [importedSongs, setImportedSongs] = useState([])
  const fileInputRef = useRef(null)

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

  const importSongs = (event) => {
    const files = Array.from(event.target.files || [])
    if (!files.length) return
    setImportedSongs((prev) => [...prev, ...files.map((file) => file.name)])
    event.target.value = ''
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
          <input
            ref={fileInputRef}
            type="file"
            accept="audio/*"
            multiple
            style={{ display: 'none' }}
            onChange={importSongs}
          />
          <Button
            variant="outlined"
            color="primary"
            onClick={() => fileInputRef.current?.click()}
          >
            {translate('menu.aiTool.importAction', { _: 'Import songs' })}
          </Button>

          <List>
            {importedSongs.map((songName) => (
              <ListItem key={songName}>
                <ListItemText primary={songName} />
              </ListItem>
            ))}
          </List>
        </CardContent>
      </Card>
    </Box>
  )
}

export default AiToolPage
