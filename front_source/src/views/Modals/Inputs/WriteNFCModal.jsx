import { CopyAll } from '@mui/icons-material'
import { Box, Button, Checkbox, Input, ListItem, Typography } from '@mui/joy'
import { useState } from 'react'
import FadeModal from '../../../components/common/FadeModal'

function WriteNFCModal({ config }) {
  const [nfcStatus, setNfcStatus] = useState('idle') // 'idle', 'writing', 'success', 'error'
  const [errorMessage, setErrorMessage] = useState('')
  const [isAutoCompleteWhenScan, setIsAutoCompleteWhenScan] = useState(false)

  const requestNFCAccess = async () => {
    if ('NDEFReader' in window) {
      // Assuming permission request is implicit in 'write' or 'scan' methods
      setNfcStatus('idle')
    } else {
      alert('NFC is not supported by this browser.')
    }
  }

  const writeToNFC = async url => {
    if ('NDEFReader' in window) {
      try {
        const ndef = new window.NDEFReader()
        await ndef.write({
          records: [{ recordType: 'url', data: url }],
        })
        setNfcStatus('success')
      } catch (error) {
        console.error('Error writing to NFC tag:', error)
        setNfcStatus('error')
        setErrorMessage('Error writing to NFC tag. Please try again.')
      }
    } else {
      setNfcStatus('error')
      setErrorMessage(
        'NFC is not supported by this browser. You can still copy the URL and write it to an NFC tag using a compatible device.',
      )
    }
  }

  const handleClose = () => {
    config.onClose()
    setNfcStatus('idle')
    setErrorMessage('')
  }
  const getURL = () => {
    let url = config.url
    if (isAutoCompleteWhenScan) {
      url = url + '?auto_complete=true'
    }

    return url
  }
  return (
    <FadeModal open={config?.isOpen} onClose={handleClose}>
      <Typography level='h4' mb={1}>
        {nfcStatus === 'success' ? 'Success!' : 'Write to NFC'}
      </Typography>

      {nfcStatus === 'success' ? (
        <Typography level='body-md' gutterBottom>
          URL written to NFC tag successfully!
        </Typography>
      ) : (
        <>
          <Typography level='body-md' gutterBottom>
            {nfcStatus === 'error'
              ? errorMessage
              : 'Press the button below to write to NFC.'}
          </Typography>
          <Input
            value={getURL()}
            fullWidth
            readOnly
            label='URL'
            sx={{ mt: 1 }}
            endDecorator={
              <CopyAll
                sx={{ cursor: 'pointer' }}
                onClick={() => {
                  navigator.clipboard.writeText(getURL())
                  alert('URL copied to clipboard!')
                }}
              />
            }
          />
          <ListItem>
            <Checkbox
              checked={isAutoCompleteWhenScan}
              onChange={e => setIsAutoCompleteWhenScan(e.target.checked)}
              label='Auto-complete when scanned'
            />
          </ListItem>
          <Box display={'flex'} justifyContent={'space-around'} mt={1}>
            <Button
              onClick={() => writeToNFC(getURL())}
              fullWidth
              sx={{ mr: 1 }}
              disabled={nfcStatus === 'writing'}
            >
              Write NFC
            </Button>
            <Button onClick={requestNFCAccess} variant='outlined'>
              Request Access
            </Button>
          </Box>
        </>
      )}
    </FadeModal>
  )
}

export default WriteNFCModal
