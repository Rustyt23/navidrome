import { fetchUtils } from 'react-admin'
import { v4 as uuidv4 } from 'uuid'
import { baseUrl } from '../utils'
import config from '../config'
import { jwtDecode } from 'jwt-decode'
import { removeHomeCache } from '../utils/removeHomeCache'

const customAuthorizationHeader = 'X-ND-Authorization'
const clientUniqueIdHeader = 'X-ND-Client-Unique-Id'
const clientUniqueId = uuidv4()

const buildHeaders = (headers, { acceptJson = true } = {}) => {
  const providedHeaders = Boolean(headers)
  const builtHeaders = new Headers(headers || {})
  if (acceptJson && !providedHeaders && !builtHeaders.has('Accept')) {
    builtHeaders.set('Accept', 'application/json')
  }
  builtHeaders.set(clientUniqueIdHeader, clientUniqueId)
  const token = localStorage.getItem('token')
  if (token) {
    builtHeaders.set(customAuthorizationHeader, `Bearer ${token}`)
  }
  return builtHeaders
}

export const withAuthHeaders = (options = {}, config = {}) => ({
  ...options,
  headers: buildHeaders(options.headers, config),
})

const processAuthToken = (response) => {
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

export const httpClientRaw = (url, options = {}) =>
  fetch(baseUrl(url), withAuthHeaders(options, { acceptJson: false })).then(
    processAuthToken,
  )

const httpClient = (url, options = {}) => {
  const finalUrl = baseUrl(url)
  const requestOptions = withAuthHeaders(options)
  return fetchUtils.fetchJson(finalUrl, requestOptions).then(processAuthToken)
}

export default httpClient
