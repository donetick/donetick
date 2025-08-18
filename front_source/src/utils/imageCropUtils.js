// Utility to crop and resize an image to a square (e.g. 320x320) and return a JPEG Blob
// Usage: await getCroppedImg(imageSrc, croppedAreaPixels, width, height, mimeType)
export async function getCroppedImg(
  imageSrc,
  crop,
  width,
  height,
  mimeType = 'image/jpeg',
) {
  return new Promise((resolve, reject) => {
    const image = new window.Image()
    image.crossOrigin = 'anonymous'
    image.onload = () => {
      const canvas = document.createElement('canvas')
      canvas.width = width
      canvas.height = height
      const ctx = canvas.getContext('2d')
      // Draw the cropped image to the canvas
      ctx.drawImage(
        image,
        crop.x,
        crop.y,
        crop.width,
        crop.height,
        0,
        0,
        width,
        height,
      )
      canvas.toBlob(
        blob => {
          if (!blob) {
            reject(new Error('Canvas is empty'))
            return
          }
          resolve(blob)
        },
        mimeType,
        0.92,
      )
    }
    image.onerror = error => reject(error)
    image.src = imageSrc
  })
}
