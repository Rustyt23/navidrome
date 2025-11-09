import { useDrag, useDrop } from 'react-dnd'

const useDragAndDrop = (type, item, accepts, onDrop) => {
    const [{ isDragging }, dragRef, previewRef] = useDrag(() => ({
        type,
        item,
        collect: (monitor) => ({ isDragging: !!monitor.isDragging() }),
        options: { dropEffect: 'move' },
    }))

    const [{ isOver, canDrop }, dropRef] = useDrop(() => ({
        accept: accepts,
        drop: onDrop,
        collect: (monitor) => ({
            isOver: monitor.isOver({ shallow: true }),
            canDrop: monitor.canDrop(),
        }),
    }))

    return {
        dragDropRef: (node) => dragRef(dropRef(node)),
        dragPreviewRef: previewRef,
        isDragging,
        isOver,
        canDrop,
    }
}

export default useDragAndDrop