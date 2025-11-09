import { useCallback, useEffect } from 'react'
import { useDrag, useDrop } from 'react-dnd'
import { getEmptyImage } from 'react-dnd-html5-backend'

export const RETAIL_PLAYER_DND_TYPES = {
  DEVICE: 'RETAIL_PLAYER_DEVICE',
}

export const useRetailPlayerDeviceDrag = ({
  deviceId,
  deviceName,
  origin,
}) => {
  const [{ isDragging }, dragRef, previewRef] = useDrag(
    () => ({
      type: RETAIL_PLAYER_DND_TYPES.DEVICE,
      item: {
        deviceId,
        deviceName,
        origin,
      },
      canDrag: Boolean(deviceId),
      collect: (monitor) => ({ isDragging: monitor.isDragging() }),
      options: { dropEffect: 'move' },
    }),
    [deviceId, deviceName, origin],
  )

  useEffect(() => {
    previewRef(getEmptyImage(), { captureDraggingState: true })
  }, [previewRef])

  return { dragRef, isDragging }
}

export const useRetailPlayerFolderDrop = ({
  folderId,
  onDrop,
  allowRootDrop = false,
}) => {
  const normalizedFolderId = folderId ?? null
  const handleDrop = useCallback(
    (item) => {
      if (!item?.deviceId) {
        return
      }
      if (!allowRootDrop && !normalizedFolderId) {
        return
      }
      if (onDrop) {
        onDrop(item.deviceId, normalizedFolderId, item)
      }
    },
    [normalizedFolderId, onDrop, allowRootDrop],
  )

  const [{ isOver, canDrop }, dropRef] = useDrop(
    () => ({
      accept: RETAIL_PLAYER_DND_TYPES.DEVICE,
      canDrop: (item) =>
        Boolean(item?.deviceId) && (allowRootDrop || Boolean(normalizedFolderId)),
      drop: (item, monitor) => {
        if (monitor.didDrop()) {
          return undefined
        }
        handleDrop(item)
        return { folderId: normalizedFolderId }
      },
      collect: (monitor) => ({
        isOver: monitor.isOver({ shallow: true }),
        canDrop: monitor.canDrop(),
      }),
    }),
    [normalizedFolderId, handleDrop, allowRootDrop],
  )

  return { dropRef, isOver, canDrop }
}
