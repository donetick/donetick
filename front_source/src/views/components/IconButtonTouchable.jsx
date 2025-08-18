import IconButton from '@mui/joy/IconButton'
import React, { useRef, useState } from 'react'

const IconButtonTouchable = ({ onHold, onClick, ...props }) => {
  const [holdTimeout, setHoldTimeout] = useState(null)
  const holdRef = useRef(false)

  const handleMouseDown = () => {
    holdRef.current = false
    setHoldTimeout(
      setTimeout(() => {
        holdRef.current = true
        onHold && onHold()
      }, 1000),
    )
  }

  const handleMouseUp = () => {
    clearTimeout(holdTimeout)
    if (!holdRef.current) {
      onClick && onClick()
    }
  }

  return (
    <IconButton
      {...props}
      onMouseDown={handleMouseDown}
      onMouseUp={handleMouseUp}
    />
  )
}

export default IconButtonTouchable
