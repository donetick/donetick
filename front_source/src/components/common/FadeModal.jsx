import { Modal, ModalDialog, ModalOverflow } from '@mui/joy'
import { Z_INDEX } from '../../constants/zIndex'

/**
 * FadeModal component with consistent fade-in/out animations
 * Can be used as a drop-in replacement for Joy UI's Modal component
 */
const FadeModal = ({
  open,
  onClose,
  children,
  size = 'md',
  fullWidth = false,
  backdropBlur = true,
  ...props
}) => {
  return (
    <Modal
      open={open}
      onClose={onClose}
      sx={{
        '& .MuiModal-backdrop': {
          backdropFilter: backdropBlur ? 'blur(3px)' : 'none',
        },
      }}
      keepMounted
      // These transition properties create a smooth fade + slide effect
      transition={{
        mount: { opacity: 1, transform: 'translateY(0px)' },
        unmount: { opacity: 0, transform: 'translateY(20px)' },
        duration: 250, // Animation duration in ms
        easing: {
          enter: 'cubic-bezier(0.34, 1.56, 0.64, 1)', // Slight overshoot for natural feel
          exit: 'cubic-bezier(0.4, 0, 0.2, 1)', // Standard ease out
        },
      }}
      {...props}
    >
      <ModalOverflow>
        <ModalDialog
          size={size}
          sx={{
            zIndex: Z_INDEX.MODAL_CONTENT,
            minWidth: fullWidth ? '100%' : 'auto',
            animation: open
              ? 'modalFadeIn 0.35s forwards'
              : 'modalFadeOut 0.25s forwards',
            '@keyframes modalFadeIn': {
              from: { opacity: 0, transform: 'translateY(8px)' },
              to: { opacity: 1, transform: 'translateY(0)' },
            },
            '@keyframes modalFadeOut': {
              from: { opacity: 1, transform: 'translateY(0)' },
              to: { opacity: 0, transform: 'translateY(8px)' },
            },
            // Add staggered animation for child elements
            '& > *': {
              opacity: 0,
              animation: open
                ? 'contentFadeIn 0.35s forwards'
                : 'contentFadeOut 0.2s forwards',
            },
            // Stagger child animations
            '& > *:nth-of-type(1)': { animationDelay: '0.05s' },
            '& > *:nth-of-type(2)': { animationDelay: '0.1s' },
            '& > *:nth-of-type(3)': { animationDelay: '0.15s' },
            '& > *:nth-of-type(4)': { animationDelay: '0.2s' },
            '& > *:nth-of-type(5)': { animationDelay: '0.25s' },
            '@keyframes contentFadeIn': {
              to: { opacity: 1 },
            },
            '@keyframes contentFadeOut': {
              to: { opacity: 0 },
            },
          }}
        >
          {children}
        </ModalDialog>
      </ModalOverflow>
    </Modal>
  )
}

export default FadeModal
