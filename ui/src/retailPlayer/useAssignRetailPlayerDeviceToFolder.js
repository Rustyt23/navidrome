import { useCallback } from 'react'
import { useRetailPlayerDeviceStore } from './RetailPlayerDeviceStoreContext'

const normalizeFolderIds = (value) => {
  if (!value) {
    return []
  }
  if (Array.isArray(value)) {
    return value.filter(Boolean)
  }
  if (typeof value === 'string') {
    return [value].filter(Boolean)
  }
  if (Array.isArray(value.folderIds)) {
    return value.folderIds.filter(Boolean)
  }
  if (value.folderId) {
    return [value.folderId].filter(Boolean)
  }
  return []
}

const useAssignRetailPlayerDeviceToFolder = () => {
  const {
    state: { devices },
    actions: { assignDeviceToFolder },
  } = useRetailPlayerDeviceStore()

  return useCallback(
    (deviceId, folderId) => {
      if (!deviceId) {
        return false
      }

      const targetDevice = devices.find((device) => device.id === deviceId)
      if (!targetDevice) {
        return false
      }

      const currentFolderIds = normalizeFolderIds(targetDevice)
      const nextFolderIds = normalizeFolderIds(folderId)
      const unchanged =
        currentFolderIds.length === nextFolderIds.length &&
        currentFolderIds.every((id, index) => id === nextFolderIds[index])

      if (unchanged) {
        return false
      }

      assignDeviceToFolder({ id: deviceId, folderIds: nextFolderIds })
      return true
    },
    [assignDeviceToFolder, devices],
  )
}

export default useAssignRetailPlayerDeviceToFolder
