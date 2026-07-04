import { useTranslate } from 'react-admin'
import { useDispatch, useSelector } from 'react-redux'
import { FormControl, FormControlLabel, Switch } from '@material-ui/core'
import { setAppBarIcons } from '../actions'

const IconToggle = ({ id, labelKey, settingKey }) => {
  const translate = useTranslate()
  const dispatch = useDispatch()
  const visible = useSelector(
    (state) => state.settings?.appBarIcons?.[settingKey] !== false,
  )

  return (
    <FormControl>
      <FormControlLabel
        control={
          <Switch
            id={id}
            color="primary"
            checked={visible}
            onChange={(event) =>
              dispatch(setAppBarIcons({ [settingKey]: event.target.checked }))
            }
          />
        }
        label={translate(labelKey)}
      />
    </FormControl>
  )
}

export const NowPlayingIconToggle = () => (
  <IconToggle
    id="nowPlayingIconToggle"
    labelKey="menu.personal.options.showNowPlayingIcon"
    settingKey="nowPlaying"
  />
)

export const MissingTracksIconToggle = () => (
  <IconToggle
    id="missingTracksIconToggle"
    labelKey="menu.personal.options.showMissingTracksIcon"
    settingKey="missingTracks"
  />
)
