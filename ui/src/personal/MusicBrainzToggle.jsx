import { useTranslate } from 'react-admin'
import { useDispatch } from 'react-redux'
import { FormControl, FormControlLabel, Switch } from '@material-ui/core'
import { setMusicBrainzVisible } from '../actions'
import { useMusicBrainzVisible } from '../common'

export const MusicBrainzToggle = () => {
  const translate = useTranslate()
  const dispatch = useDispatch()
  const visible = useMusicBrainzVisible()

  return (
    <FormControl>
      <FormControlLabel
        control={
          <Switch
            id="musicBrainzToggle"
            color="primary"
            checked={visible}
            onChange={(event) =>
              dispatch(setMusicBrainzVisible(event.target.checked))
            }
          />
        }
        label={translate('menu.personal.options.showMusicBrainz')}
      />
    </FormControl>
  )
}
