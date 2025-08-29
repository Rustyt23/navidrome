import { useCallback } from 'react'
import {
  Edit,
  FormDataConsumer,
  SimpleForm,
  TextInput,
  TextField,
  BooleanInput,
  required,
  useTranslate,
  usePermissions,
  ReferenceInput,
  SelectInput,
  useNotify,
  useRedirect,
  useRefresh,
  useDataProvider,
  useRecordContext,
  Toolbar,
  SaveButton,
} from 'react-admin'
import { useLocation } from 'react-router-dom'
import { useFormState } from 'react-final-form'
import { makeStyles } from '@material-ui/core/styles'
import Button from '@material-ui/core/Button'
import DeleteIcon from '@material-ui/icons/Delete'
import { isWritable, Title } from '../common'

const SyncFragment = ({ formData, ...rest }) => (
  <>
    {formData?.path && <BooleanInput source="sync" {...rest} />}
    {formData?.path && <TextField source="path" {...rest} />}
  </>
)

const PlaylistFolderTitle = ({ record }) => {
  const translate = useTranslate()
  const resourceName = translate('resources.folder.name', { smart_count: 1 })
  return <Title subTitle={`${resourceName} "${record ? record.name : ''}"`} />
}

const useToolbarStyles = makeStyles((theme) => ({
  root: { display: 'flex', alignItems: 'center', padding: theme.spacing(1, 2) },
  grow: { flex: 1 },
  delete: { color: theme.palette.error.main },
}))

const FolderEditToolbar = ({ handleSubmitWithRedirect, saving }) => {
  const classes = useToolbarStyles()
  const record = useRecordContext()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const redirect = useRedirect()
  const refresh = useRefresh()
  const location = useLocation()

  const { pristine, submitting, invalid } = useFormState({
    subscription: { pristine: true, submitting: true, invalid: true },
  })
  const saveDisabled = pristine || submitting || invalid

  const handleDelete = async () => {
    if (!record?.id) return
    try {
      await dataProvider.delete('folder', { id: record.id })
      notify('ra.notification.deleted', { type: 'info', messageArgs: { smart_count: 1 } })

      const parentId = record?.parentId ?? location.state?.parentId ?? null
      if (parentId) redirect(`/folder/${parentId}/show`)
      else redirect('list', '/folder')

      refresh()
    } catch (e) {
      notify(e?.message || 'ra.notification.http_error', { type: 'warning' })
    }
  }

  return (
    <Toolbar className={classes.root}>
      <SaveButton
        variant="contained"
        color="primary"
        disabled={saveDisabled}
        saving={saving ?? submitting}
        handleSubmitWithRedirect={handleSubmitWithRedirect}
      />
      <span className={classes.grow} />
      <Button onClick={handleDelete} startIcon={<DeleteIcon />} className={classes.delete}>
        DELETE
      </Button>
    </Toolbar>
  )
}

const PlaylistFolderEditForm = () => {
  const { permissions } = usePermissions()
  const record = useRecordContext()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const redirect = useRedirect()
  const refresh = useRefresh()
  const location = useLocation()

  const saveFolder = useCallback(
    async (values) => {
      if (!record?.id) return
      try {
        const res = await dataProvider.update('folder', { id: record.id, data: values })
        const saved = res?.data ?? values
        notify('ra.notification.updated', { type: 'info', messageArgs: { smart_count: 1 } })

        const parentId =
          saved?.parentId ?? saved?.parent_id ?? location.state?.parentId ?? null

        if (parentId) redirect(`/folder/${parentId}/show`)
        else redirect('list', '/folder')

        refresh()
      } catch (e) {
        notify(e?.message || 'ra.page.error', { type: 'warning' })
      }
    },
    [dataProvider, record?.id, notify, redirect, refresh, location]
  )

  return (
    <SimpleForm
      save={saveFolder}
      variant={'outlined'}
      redirect={false}
      toolbar={<FolderEditToolbar />}
    >
      <TextInput source="name" validate={required()} />

      {permissions === 'admin' ? (
        <ReferenceInput
          source="ownerId"
          reference="user"
          perPage={0}
          sort={{ field: 'name', order: 'ASC' }}
        >
          <SelectInput
            label="resources.folder.fields.ownerName"
            optionText="userName"
          />
        </ReferenceInput>
      ) : (
        <TextField source="ownerName" />
      )}

      <FormDataConsumer>
        {({ formData }) => (
          <BooleanInput source="public" disabled={!isWritable(formData?.ownerId)} />
        )}
      </FormDataConsumer>

      <FormDataConsumer>
        {(formDataProps) => <SyncFragment {...formDataProps} />}
      </FormDataConsumer>
    </SimpleForm>
  )
}

const PlaylistFolderEdit = (props) => (
  <Edit title={<PlaylistFolderTitle />} actions={false} {...props}>
    <PlaylistFolderEditForm />
  </Edit>
)

export default PlaylistFolderEdit
