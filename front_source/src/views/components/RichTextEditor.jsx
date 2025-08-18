import imageCompression from 'browser-image-compression'
import Quill from 'quill'
import 'quill/dist/quill.snow.css'
import QuillMarkdown from 'quilljs-markdown'
import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
} from 'react'
import { useUserProfile } from '../../queries/UserQueries'
import { useNotification } from '../../service/NotificationProvider'
import { isPlusAccount, resolvePhotoURL } from '../../utils/Helpers'
import { UploadFile } from '../../utils/TokenManager'
import './RichTextEditor.css'

const RichTextEditor = forwardRef(
  (
    {
      value = '',
      onChange,
      isEditable = true,
      placeholder = 'Enter description...',
      variant = 'outlined',
      entityId,
      entityType,
    },
    ref,
  ) => {
    const { showError } = useNotification()
    const { data: userProfile } = useUserProfile()
    const quillRef = useRef(null)
    const editorRef = useRef(null)

    // Expose focus method to parent components
    useImperativeHandle(
      ref,
      () => ({
        focus: () => {
          if (editorRef.current) {
            editorRef.current.focus()
          }
        },
        blur: () => {
          if (editorRef.current) {
            editorRef.current.blur()
          }
        },
      }),
      [],
    )

    // Image upload handler - wrapped in useCallback to avoid recreating on every render
    const handleImageUpload = useCallback(() => {
      // Check if user has plus account
      if (!isPlusAccount(userProfile)) {
        showError({
          title: 'Plus Feature',
          message:
            'Image uploads are not available in the Basic plan. Upgrade to Plus to add images to your content.',
        })
        return
      }

      const input = document.createElement('input')
      input.setAttribute('type', 'file')
      input.setAttribute('accept', 'image/*')
      input.click()
      input.onchange = async () => {
        const file = input.files[0]
        if (!file) return

        try {
          // Define compression options based on entity type ( this need a revist later)
          const compressionOptions = {
            maxSizeMB: entityType === 'profile' ? 0.5 : 1, // Smaller size for profile images
            maxWidthOrHeight: entityType === 'profile' ? 320 : 1200, // Smaller dimensions for profile images
            useWebWorker: true,
            fileType: 'image/jpeg',
          }

          // Compress the image
          const compressedFile = await imageCompression(
            file,
            compressionOptions,
          )

          // Create new file with .jpg extension to ensure it's treated as JPEG
          const compressedJpegFile = new File(
            [compressedFile],
            `${file.name.split('.')[0]}.jpg`,
            { type: 'image/jpeg' },
          )

          console.log(
            `Original size: ${(file.size / 1024 / 1024).toFixed(2)} MB`,
          )
          console.log(
            `Compressed size: ${(compressedJpegFile.size / 1024 / 1024).toFixed(2)} MB`,
          )

          // Upload compressed image to backend
          const formData = new FormData()
          formData.append('file', compressedJpegFile)
          formData.append('entityId', entityId)
          formData.append('entityType', entityType)

          const response = await UploadFile('/assets/chore', {
            method: 'POST',
            body: formData,
          })

          if (response.status === 507) {
            showError({
              title: 'Storage Quota Exceeded',
              message: 'You have exceeded your quota for uploading files.',
            })
            return
          } else if (response.status === 413) {
            showError({
              title: 'File Too Large',
              message: 'The file you are trying to upload is too large.',
            })
            return
          } else if (response.status === 403 && !isPlusAccount()) {
            showError({
              title: 'Upgrade Required',
              message:
                'Image uploads are only available for Plus accounts. Please ',
            })
            return
          } else if (response.status === 403) {
            showError({
              title: 'Permission Denied',
              message: 'You do not have permission to upload files.',
            })
            return
          } else if (!response.ok) {
            showError({
              title: 'Upload Failed',
              message: 'Failed to upload image.',
            })
            return
          }
          const data = await response.json()
          const url = resolvePhotoURL(data.url || data.sign)
          // Insert image into Quill
          const quill = editorRef.current
          const range = quill.getSelection()
          quill.insertEmbed(range ? range.index : 0, 'image', url)
        } catch (error) {
          console.error('Error during image processing or upload:', error)
          showError({
            title: 'Upload Failed',
            message: 'An error occurred while processing the image.',
          })
        }
      }
    }, [entityId, entityType, showError, userProfile]) // Dependencies for useCallback

    useEffect(() => {
      if (!quillRef.current) return
      if (!editorRef.current && isEditable) {
        editorRef.current = new Quill(quillRef.current, {
          theme: variant === 'bubble' ? 'bubble' : 'snow',
          modules: {
            toolbar: {
              container: [
                [{ header: [1, 2, 3, 4, false] }],
                ['bold', 'italic', 'underline', 'strike'],
                ['blockquote', 'code-block'],
                [{ list: 'ordered' }, { list: 'bullet' }],
                ['link', 'image'],
                ['clean'],
              ],
              handlers: {
                image: handleImageUpload,
              },
            },
          },
          placeholder: placeholder,
        })
        new QuillMarkdown(editorRef.current, {})
        editorRef.current.root.innerHTML = value
        editorRef.current.on('text-change', () => {
          if (onChange) {
            onChange(editorRef.current.root.innerHTML)
          }
        })
      }
      // If switching to read-only mode, disable Quill instance
      if (editorRef.current && !isEditable) {
        // editorRef.current.disable()
        editorRef.current.readOnly = true

        // If switching back to editable, enable Quill
        if (editorRef.current && isEditable) {
          // editorRef.current.enable()
          editorRef.current.readOnly = false
        }
      }
    }, [onChange, value, isEditable, variant, handleImageUpload, userProfile]) // Added handleImageUpload and userProfile to dependency array

    useEffect(() => {
      if (editorRef.current && isEditable) {
        if (editorRef.current.root.innerHTML !== value) {
          editorRef.current.root.innerHTML = value || ''
        }
      }
    }, [value, isEditable])

    if (!isEditable) {
      // Display-only mode: render HTML
      return (
        <div
          className='editor-view-mode'
          style={{
            minHeight: 120,
            overflow: 'scroll',
            //   border:
            //     '1px solid var(--joy-palette-neutral-outlinedBorder, #DDE7EE)',
            borderRadius: 8,
            padding: 16,
            background: 'var(--joy-palette-background-surface, #fff)',
            color: 'var(--joy-palette-text-primary, #1A2027)',
            fontFamily:
              'var(--joy-fontFamily-body, Inter, system-ui, Avenir, Helvetica, Arial, sans-serif)',
            fontSize: 16,
            boxShadow:
              'var(--joy-shadow-xs, 0px 1px 2px 0px rgba(16, 24, 40, 0.05))',
          }}
          dangerouslySetInnerHTML={{ __html: value }}
        />
      )
    }

    return (
      <div className={`quill-root quill-variant-${variant}`}>
        <div
          ref={quillRef}
          style={{
            minHeight: 120,
            background: 'var(--joy-palette-background-surface, #fff)',
          }}
        />
      </div>
    )
  },
)

RichTextEditor.displayName = 'RichTextEditor'

export default RichTextEditor
