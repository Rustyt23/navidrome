// BASE_URL stays empty to use same-origin proxy
const BASE_URL = ""

async function fetchFromRetail(path, method = 'GET', body) {
  const requestInit = {
    method,
    headers: {
      Accept: 'application/json',
    },
    credentials: 'include',
  }

  if (body !== undefined) {
    requestInit.headers['Content-Type'] = 'application/json'
    requestInit.body = JSON.stringify(body)
  }

  const response = await fetch(`${BASE_URL}${path}`, requestInit)

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
