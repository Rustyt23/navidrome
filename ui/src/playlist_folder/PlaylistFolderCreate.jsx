import {
  Create,
  SimpleForm,
  TextInput,
  BooleanInput,
  required,
  useNotify,
  useRedirect,
  useRefresh,
  useTranslate,
} from 'react-admin'
import { useLocation } from 'react-router-dom'
import { Title } from '../common'

const PlaylistFolderCreate = (props) => {
  const { basePath } = props
  const location = useLocation()
  const parentId = location.state?.parentId || null
  const refresh = useRefresh()
  const notify = useNotify()
  const redirect = useRedirect()
  const translate = useTranslate()

  const resourceName = translate('resources.folder.name', { smart_count: 1 })
  const title = translate('ra.page.create', { name: resourceName })

  const onSuccess = () => {
    notify('ra.notification.created', 'info', { smart_count: 1 })
    if (parentId) redirect(`${basePath}/${parentId}/show`)
    else redirect('list', basePath)
    refresh()
  }

  const onFailure = (error) => {
    if (error?.status === 409) {
      notify('message.folderExists', 'warning')
    } else {
      notify('ra.page.error', 'warning')
    }
  }

  return (
    <Create title={<Title subTitle={title} />} {...props} onSuccess={onSuccess} onFailure={onFailure}>
      <SimpleForm redirect="list" variant="outlined">
        <TextInput source="name" validate={required()} />
        <BooleanInput source="public" initialValue />
        <TextInput source="parentId" defaultValue={parentId} style={{ display: 'none' }} />
      </SimpleForm>
    </Create>
  )
}

export default PlaylistFolderCreate
