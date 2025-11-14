import { AddToPlaylistDialog } from './AddToPlaylistDialog'
import { EditCommentsDialog } from './EditCommentsDialog'
import DownloadMenuDialog from './DownloadMenuDialog'
import { HelpDialog } from './HelpDialog'
import { ShareDialog } from './ShareDialog'
import { SaveQueueDialog } from './SaveQueueDialog'

export const Dialogs = (props) => (
  <>
    <AddToPlaylistDialog />
    <EditCommentsDialog />
    <SaveQueueDialog />
    <DownloadMenuDialog />
    <HelpDialog />
    <ShareDialog />
  </>
)
