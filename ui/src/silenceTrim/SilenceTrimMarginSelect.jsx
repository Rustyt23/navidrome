import React, { useState } from 'react'
import TextField from '@material-ui/core/TextField'
import {
  getSilenceTrimMargin,
  SILENCE_MARGIN_OPTIONS,
  SILENCE_MARGIN_STORAGE_KEY,
} from './silenceTrimSettings'

const SilenceTrimMarginSelect = () => {
  const [margin, setMargin] = useState(getSilenceTrimMargin)

  const handleChange = (event) => {
    const next = Number(event.target.value)
    setMargin(next)
    window.localStorage.setItem(SILENCE_MARGIN_STORAGE_KEY, String(next))
  }

  return (
    <TextField
      select
      size="small"
      variant="outlined"
      label="Trim margin"
      value={margin}
      onChange={handleChange}
      SelectProps={{ native: true }}
      inputProps={{ 'aria-label': 'Trim margin in seconds' }}
      style={{ minWidth: 150, margin: '4px 8px' }}
    >
      {SILENCE_MARGIN_OPTIONS.map((value) => (
        <option key={value} value={value}>
          {value === 0 ? '0 s (exact edge)' : `${value.toFixed(2)} s`}
        </option>
      ))}
    </TextField>
  )
}

export default SilenceTrimMarginSelect
