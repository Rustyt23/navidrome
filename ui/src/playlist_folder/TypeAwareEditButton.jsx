// ui/src/playlist_folder/TypeAwareEditButton.jsx
import React, { useCallback } from 'react'
import { useRecordContext, useTranslate } from 'react-admin'
import { Link } from 'react-router-dom'
import { makeStyles } from '@material-ui/core/styles'
import Button from '@material-ui/core/Button'
import EditIcon from '@material-ui/icons/Edit'

/**
 * Compact text button with icon that does NOT increase row height.
 * Route: /folder/:id/edit or /playlist/:id/edit based on record.type
 */
const useStyles = makeStyles({
  compactBtn: {
    padding: '0 6px',   // minimal horizontal padding
    minHeight: 0,       // kill default 36px height
    lineHeight: 1,      // keep it tight
    textTransform: 'none',
    margin: 0,
    '& .MuiButton-label': {
      lineHeight: 1,
      padding: 0,
      fontSize: 12,     // small text to avoid stretching
      display: 'inline-flex',
      alignItems: 'center',
      gap: 4,
    },
    '& .MuiButton-startIcon': {
      marginRight: 4,
    },
  },
})

const TypeAwareEditButton = () => {
  const record = useRecordContext()
  const translate = useTranslate()
  const classes = useStyles()
  const stop = useCallback((e) => e.stopPropagation(), [])

  if (!record) return null

  const type = record.type || 'playlist'
  const to = `/${type}/${record.id}/edit`

  return (
    <Button
      component={Link}
      to={to}
      onClick={stop}
      size="small"
      variant="text"
      color="default"
      startIcon={<EditIcon fontSize="small" />}
      className={classes.compactBtn}
    >
      {translate('ra.action.edit')} {/* shows "Edit" */}
    </Button>
  )
}

export default TypeAwareEditButton

