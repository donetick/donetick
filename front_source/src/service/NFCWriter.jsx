const writeToNFC = async url => {
  if ('NDEFReader' in window) {
    try {
      const ndef = new window.NDEFReader()
      await ndef.write({
        records: [{ recordType: 'url', data: url }],
      })
      alert('URL written to NFC tag successfully!')
    } catch (error) {
      console.error('Error writing to NFC tag:', error)
      alert('Error writing to NFC tag. Please try again.')
    }
  } else {
    alert('NFC is not supported by this browser.')
  }
}

export default writeToNFC
