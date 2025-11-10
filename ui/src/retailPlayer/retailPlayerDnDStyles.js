import { fade } from '@material-ui/core/styles/colorManipulator'

const buildRetailPlayerDnDStyles = (theme) => {
  const activeShadowColor = fade(theme.palette.primary.main, 0.45)
  const activeBackground = fade(theme.palette.primary.main, 0.14)
  const hoverBackground = fade(theme.palette.primary.main, 0.08)

  return {
    dropTarget: {
      position: 'relative',
      borderRadius: theme.shape.borderRadius,
      transition: theme.transitions.create(['background-color', 'box-shadow'], {
        duration: theme.transitions.duration.shortest,
      }),
    },
    dropTargetCanDrop: {
      backgroundColor: hoverBackground,
    },
    dropTargetActive: {
      backgroundColor: activeBackground,
      boxShadow: `0 0 0 2px ${activeShadowColor}`,
    },
    dragItem: {
      opacity: 0.55,
      cursor: 'grabbing',
    },
  }
}

export default buildRetailPlayerDnDStyles
