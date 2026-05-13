import React, { useState } from 'react'
import {
  Box,
  Button,
  CircularProgress,
  Paper,
  TextField,
  Typography,
  makeStyles,
} from '@material-ui/core'

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(3),
    height: 'calc(100vh - 140px)',
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
  },
  messages: {
    flex: 1,
    overflowY: 'auto',
    padding: theme.spacing(2),
    background: theme.palette.background.default,
  },
  message: {
    marginBottom: theme.spacing(1),
    padding: theme.spacing(1.5),
  },
  user: { background: theme.palette.grey[900] },
  assistant: { background: theme.palette.grey[800] },
  inputRow: { display: 'flex', gap: theme.spacing(1), alignItems: 'flex-end' },
}))

const AIToolPage = () => {
  const classes = useStyles()
  const [messages, setMessages] = useState([])
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const sendMessage = async () => {
    if (!message.trim() || loading) return
    const userMessage = message.trim()
    setMessages((m) => [...m, { role: 'user', text: userMessage }])
    setMessage('')
    setError('')
    setLoading(true)

    try {
      const response = await fetch('/api/ai/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({ message: userMessage }),
      })
      if (!response.ok) throw new Error('Request failed')
      const data = await response.json()
      setMessages((m) => [...m, { role: 'assistant', text: data.answer || '' }])
    } catch (e) {
      setError('Failed to send message. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Paper className={classes.root}>
      <Typography variant="h5">AI Tool</Typography>
      <Box className={classes.messages}>
        {messages.map((msg, idx) => (
          <Paper
            key={`${msg.role}-${idx}`}
            className={`${classes.message} ${msg.role === 'user' ? classes.user : classes.assistant}`}
          >
            <Typography variant="body2" color="textSecondary">
              {msg.role === 'user' ? 'You' : 'Assistant'}
            </Typography>
            <Typography variant="body1">{msg.text}</Typography>
          </Paper>
        ))}
        {loading && <CircularProgress size={24} />}
      </Box>
      {error && <Typography color="error">{error}</Typography>}
      <Box className={classes.inputRow}>
        <TextField
          fullWidth
          multiline
          minRows={3}
          variant="outlined"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Type your message"
        />
        <Button variant="contained" color="primary" onClick={sendMessage} disabled={loading}>
          Send
        </Button>
      </Box>
    </Paper>
  )
}

export default AIToolPage
