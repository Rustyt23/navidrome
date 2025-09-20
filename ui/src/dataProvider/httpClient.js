import { fetchUtils } from 'react-admin'
import { v4 as uuidv4 } from 'uuid'
import { baseUrl } from '../utils'

const addLimitAndOffset = (url) => {
  try {
    const parsedUrl = new URL(url, window.location.origin)
    const startParam = parsedUrl.searchParams.get('_start')
    const endParam = parsedUrl.searchParams.get('_end')

    if (startParam === null || endParam === null) {
      return parsedUrl.href
    }

    const start = Number.parseInt(startParam, 10)
    const end = Number.parseInt(endParam, 10)

    if (!Number.isNaN(start) && start >= 0) {
      parsedUrl.searchParams.set('offset', start)
    }

    const limit = end - start
    if (!Number.isNaN(limit) && Number.isFinite(limit) && limit > 0) {
      parsedUrl.searchParams.set('limit', limit)
    }

    return parsedUrl.href
  } catch (error) {
    return url
  }
}
import config from '../config'
import { jwtDecode } from 'jwt-decode'
import { removeHomeCache } from '../utils/removeHomeCache'

const customAuthorizationHeader = 'X-ND-Authorization'
const clientUniqueIdHeader = 'X-ND-Client-Unique-Id'
const clientUniqueId = uuidv4()

const httpClient = (url, options = {}) => {
  url = addLimitAndOffset(baseUrl(url))
  if (!options.headers) {
    options.headers = new Headers({ Accept: 'application/json' })
  }
  options.headers.set(clientUniqueIdHeader, clientUniqueId)
  const token = localStorage.getItem('token')
  if (token) {
    options.headers.set(customAuthorizationHeader, `Bearer ${token}`)
  }
  return fetchUtils.fetchJson(url, options).then((response) => {
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
  })
}

export default httpClient
