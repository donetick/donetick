import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'

const EditNotificationTarget = () => {
  const { id } = useParams()
  const [notificationTarget, setNotificationTarget] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    // const fetchNotificationTarget = async () => {
    //   try {
    //     const response = await fetch(`/api/notification-targets/${id}`)
    //     const data = await response.json()
    //     setNotificationTarget(data)
    //   } catch (error) {
    //     setError(error)
    //   } finally {
    //     setLoading(false)
    //   }
    // }
    // fetchNotificationTarget()
  }, [id])

  if (loading) {
    return <div>Loading...</div>
  }

  if (error) {
    return <div>Error: {error.message}</div>
  }

  return (
    <div>
      <h1>Edit Notification Target</h1>
      <form>
        <label>
          Name:
          <input type='text' value={notificationTarget.name} />
        </label>
        <label>
          Email:
          <input type='email' value={notificationTarget.email} />
        </label>
        <button type='submit'>Save</button>
      </form>
    </div>
  )
}

export default EditNotificationTarget
