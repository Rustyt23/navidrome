import { v4 as uuidv4 } from 'uuid'
import { baseUrl } from '../utils/urls'

const AUTH_HEADER = 'X-ND-Authorization'
const CLIENT_ID_HEADER = 'X-ND-Client-Unique-Id'
const clientUniqueId = uuidv4()

async function fetchFromRetail(path, method = 'GET', body) {
  const headers = new Headers({ Accept: 'application/json' })

  headers.set(CLIENT_ID_HEADER, clientUniqueId)

  const token = localStorage.getItem('token')
  if (token) {
    headers.set(AUTH_HEADER, `Bearer ${token}`)
  }

  if (body !== undefined) {
    headers.set('Content-Type', 'application/json')
  }

  const requestInit = {
    method,
    headers,
    credentials: 'include',
  }

  if (body !== undefined) {
    requestInit.body = JSON.stringify(body)
  }

  const response = await fetch(baseUrl(path), requestInit)

  if (!response.ok) {
    const error = new Error(
      `Retail Player request failed with status ${response.status}`,
    )
    error.status = response.status
    error.statusText = response.statusText
    throw error
  }

  if (response.status === 204) {
    return null
  }

  const contentType = response.headers.get('Content-Type') || ''
  if (!contentType.includes('application/json')) {
    return null
  }

  return response.json()
}

export async function getDevices() {
  const payload = await fetchFromRetail(`/api/retailplayer/devices`, 'GET')
  return payload?.data ?? []
}

export async function getDevice(id) {
  return fetchFromRetail(`/api/retailplayer/devices/${id}`, 'GET')
}

export async function getDeviceStatus(id) {
  return fetchFromRetail(`/api/retailplayer/devices/${id}/status`, 'GET')
}

export async function postDeviceCommand(id, payload) {
  return fetchFromRetail(`/api/retailplayer/devices/${id}/command`, 'POST', payload)
}

export { fetchFromRetail }
