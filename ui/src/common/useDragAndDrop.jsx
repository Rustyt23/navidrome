import { useDrag, useDrop } from 'react-dnd'

const useDragAndDrop = (type, item, accepts, onDrop) => {
    const [{ isDragging }, dragRef] = useDrag(() => ({
        type,
        item,
        collect: (monitor) => ({ isDragging: !!monitor.isDragging() }),
        options: { dropEffect: 'move' },
    }))

    const [, dropRef] = useDrop(() => ({
        accept: accepts,
        drop: onDrop,
    }))

    return {
        dragDropRef: (node) => dragRef(dropRef(node)),
        isDragging,
    }
}

export default useDragAndDrop