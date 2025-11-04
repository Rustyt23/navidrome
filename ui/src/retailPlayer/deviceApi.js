import config from '../config'
import httpClient from '../dataProvider/httpClient'
import RetailPlayerMockService from './RetailPlayerMockService'
import { buildDeviceSlug, deviceSlugKey, normalizeValue } from './deviceUtils'

export const buildDevicesUrl = () => '/api/retailplayer/devices'

export const mapRetailPlayerDevice = (device) => {
  if (!device || typeof device !== 'object') {
    return null
  }

  const rawId = normalizeValue(device.id)
  const fallbackId = rawId || normalizeValue(device.macAddress) || normalizeValue(device.ordinal)

  if (!fallbackId) {
    return null
  }

  const slug = buildDeviceSlug(device) || fallbackId
  const name = normalizeValue(device.name) || fallbackId

  return {
    id: fallbackId,
    apiId: rawId || null,
    name,
    slug,
    slugKey: deviceSlugKey(slug),
    channel: normalizeValue(device.channel),
    channelList: normalizeValue(device.channelList),
    organization:
      normalizeValue(device.organization) ||
      normalizeValue(device.orgUnit) ||
      normalizeValue(device.location),
    timeZone: normalizeValue(device.timeZone),
  }
}

export const fetchRetailPlayerDevices = async ({ signal } = {}) => {
  const url = buildDevicesUrl()
  if (!url) {
    return { devices: null, enabled: false }
  }

  try {
    const { json } = await httpClient(url, { signal })
    const payload = Array.isArray(json?.data) ? json.data : []
    const devices = payload.map(mapRetailPlayerDevice).filter(Boolean)
    return { devices, enabled: true }
  } catch (err) {
    if (err?.status === 404) {
      return { devices: null, enabled: false }
    }

    const status = typeof err?.status === 'number' ? err.status : null
    if (status) {
      throw new Error(`Retail player device request failed with status ${status}`)
    }

    throw err
  }
}

export const resolveRetailPlayerDevices = async (options = {}) => {
  if (!config.retailPlayerDevicesEnabled) {
    return { devices: RetailPlayerMockService.listDevices(), enabled: false }
  }

  const result = await fetchRetailPlayerDevices(options)
  if (!result.enabled || !Array.isArray(result.devices)) {
    return { devices: RetailPlayerMockService.listDevices(), enabled: false }
  }

  return result
}

export const getRetailPlayerDevicesSync = () => {
  if (!config.retailPlayerDevicesEnabled) {
    return RetailPlayerMockService.listDevices()
  }
  return []
}
