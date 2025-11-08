import { useCallback } from 'react'
import { useRetailPlayerDeviceStore } from './RetailPlayerDeviceStoreContext'

const normalizeFolderIds = (device) => {
  if (!device) {
    return []
  }
  if (Array.isArray(device.folderIds)) {
    return device.folderIds.filter(Boolean)
  }
  if (device.folderId) {
    return [device.folderId].filter(Boolean)
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
      if (!deviceId || !folderId) {
        return false
      }

      const targetDevice = devices.find((device) => device.id === deviceId)
      if (!targetDevice) {
        return false
      }

      const currentFolderIds = normalizeFolderIds(targetDevice)
      const nextFolderIds = [folderId]
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
