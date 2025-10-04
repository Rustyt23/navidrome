import { AddToPlaylistDialog } from './AddToPlaylistDialog'
import DownloadMenuDialog from './DownloadMenuDialog'
import DiscoveryUploadDialog from './DiscoveryUploadDialog'
import { HelpDialog } from './HelpDialog'
import { ShareDialog } from './ShareDialog'
import { SaveQueueDialog } from './SaveQueueDialog'

export const Dialogs = (props) => (
  <>
    <AddToPlaylistDialog />
    <DiscoveryUploadDialog />
    <SaveQueueDialog />
    <DownloadMenuDialog />
    <HelpDialog />
    <ShareDialog />
  </>
)
