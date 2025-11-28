import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

const REMOTE_CONTROL_BASE_URL = 'wss://rc.jareddietch.com/remote-control'

const stringifyMessage = (message) => {
  if (typeof message === 'string') {
    return message
  }

  try {
    return JSON.stringify(message)
  } catch (error) {
    return ''
  }
}

const useRemoteControlSocket = (deviceUUID) => {
  const [isConnected, setIsConnected] = useState(false)
  const [lastMessage, setLastMessage] = useState(null)
  const [connectionError, setConnectionError] = useState(null)
  const socketRef = useRef(null)

  const socketUrl = useMemo(() => {
    if (!deviceUUID) {
      return ''
    }

    return `${REMOTE_CONTROL_BASE_URL}/${encodeURIComponent(deviceUUID)}`
  }, [deviceUUID])

  const cleanupSocket = useCallback(() => {
    if (socketRef.current) {
      socketRef.current.close()
      socketRef.current = null
    }
  }, [])

  useEffect(() => {
    if (!socketUrl) {
      cleanupSocket()
      setIsConnected(false)
      setLastMessage(null)
      return undefined
    }

    const socket = new WebSocket(socketUrl)
    socketRef.current = socket

    socket.onopen = () => {
      setIsConnected(true)
      setConnectionError(null)
    }

    socket.onmessage = (event) => {
      setLastMessage(event.data)
    }

    socket.onerror = () => {
      setConnectionError(new Error('Remote control socket error'))
    }

    socket.onclose = () => {
      setIsConnected(false)
    }

    return () => {
      cleanupSocket()
    }
  }, [cleanupSocket, socketUrl])

  const sendMessage = useCallback((message) => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false
    }

    const payload = stringifyMessage(message)
    if (!payload) {
      return false
    }

    socket.send(payload)
    return true
  }, [])

  return {
    connectionError,
    isConnected,
    lastMessage,
    sendMessage,
  }
}

export default useRemoteControlSocket
