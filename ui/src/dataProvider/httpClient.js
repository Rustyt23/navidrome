import { fetchUtils } from 'react-admin'
import { v4 as uuidv4 } from 'uuid'
import { baseUrl } from '../utils'
import config from '../config'
import { jwtDecode } from 'jwt-decode'
import { removeHomeCache } from '../utils/removeHomeCache'

const customAuthorizationHeader = 'X-ND-Authorization'
const clientUniqueIdHeader = 'X-ND-Client-Unique-Id'
const clientUniqueId = uuidv4()

const shouldRetryInactiveTab = (error) => {
  const message = error?.message || ''
  return message.includes('Failed to fetch') || message.includes('NetworkError')
}

const waitForTabVisible = () => {
  if (typeof document === 'undefined') {
    return Promise.resolve()
  }
  if (document.visibilityState === 'visible') {
    return Promise.resolve()
  }
  return new Promise((resolve) => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        document.removeEventListener('visibilitychange', handleVisibility)
        resolve()
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
  })
}

const httpClient = (url, options = {}) => {
  url = baseUrl(url)
  if (!options.headers) {
    options.headers = new Headers({ Accept: 'application/json' })
  }
  options.headers.set(clientUniqueIdHeader, clientUniqueId)
  const token = localStorage.getItem('token')
  if (token) {
    options.headers.set(customAuthorizationHeader, `Bearer ${token}`)
  }
  const handleResponse = (response) => {
    const token = response.headers.get(customAuthorizationHeader)
    if (token) {
      const decoded = jwtDecode(token)
      localStorage.setItem('token', token)
      localStorage.setItem('userId', decoded.uid)
      // Avoid going to create admin dialog after logout/login without a refresh
      config.firstTime = false
      removeHomeCache()
    }
    return response
  }

  const executeRequest = () => fetchUtils.fetchJson(url, options).then(handleResponse)

  return executeRequest().catch(async (error) => {
    if (
      shouldRetryInactiveTab(error) &&
      typeof document !== 'undefined' &&
      document.visibilityState === 'hidden'
    ) {
      await waitForTabVisible()
      return executeRequest()
    }
    throw error
  })
}

export default httpClient
