import { Person } from '@mui/icons-material'
import { Avatar, Box, Button, Sheet, Typography } from '@mui/joy'

import { useEffect, useState } from 'react'
import { useImpersonateUser } from '../../contexts/ImpersonateUserContext'
import { useCircleMembers, useUserProfile } from '../../queries/UserQueries'
import UserModal from '../Modals/Inputs/UserModal'
const WelcomeCard = () => {
  const { impersonatedUser, setImpersonatedUser } = useImpersonateUser()
  const [isAdmin, setIsAdmin] = useState(false)
  const { data: userProfile } = useUserProfile()

  const [isModalOpen, setIsModalOpen] = useState(false)

  const { data: circleMembersData, isLoading: isCircleMembersLoading } =
    useCircleMembers()

  useEffect(() => {
    if (userProfile && userProfile?.id) {
      const members = circleMembersData?.res || []
      const isUserAdmin = members.some(
        member =>
          member.userId === userProfile?.id &&
          (member.role === 'admin' || member.role === 'manager'),
      )

      setIsAdmin(isUserAdmin)
    }
  }, [userProfile, circleMembersData])
  if (!isAdmin) {
    return null
  } else if (isCircleMembersLoading || impersonatedUser === null) {
    return (
      <Sheet
        variant='plain'
        sx={{
          p: 2,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          mr: 10,
          justifyContent: 'space-between',
          boxShadow: 'sm',
          borderRadius: 20,
          width: '315px',
          mb: 1,
        }}
      >
        <Box sx={{ textAlign: 'center', width: '100%' }}>
          {/* Header */}
          <Box sx={{ mb: 2 }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'flex-start',
                gap: 1,
              }}
            >
              <Person color='' />
              <Typography level='title-md'>Current User</Typography>
            </Box>
          </Box>
          <Box sx={{ mb: 2 }}>
            <Typography level='title-md' sx={{ mb: 0.5 }}>
              Who&apos;s checking in?
            </Typography>
          </Box>
          <Button
            variant='plain'
            color='primary'
            onClick={() => setIsModalOpen(true)}
            size='sm'
          >
            Select User
          </Button>
          <UserModal
            isOpen={isModalOpen}
            performers={circleMembersData?.res}
            onSelect={user => {
              setImpersonatedUser(user)
              setIsModalOpen(false)
            }}
            onClose={() => setIsModalOpen(false)}
          />
        </Box>
      </Sheet>
    )
  }

  return (
    <Sheet
      variant='plain'
      sx={{
        p: 2,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        mr: 10,
        justifyContent: 'space-between',
        boxShadow: 'sm',
        borderRadius: 20,
        width: '310px',
        mb: 1,
      }}
    >
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          mb: 2,
        }}
      >
        {/* Header */}
        <Box sx={{ mb: 2, width: '100%' }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'flex-start',
              gap: 1,
            }}
          >
            <Person color='' />
            <Typography level='title-md'>Current User</Typography>
          </Box>
        </Box>

        <Box sx={{ display: 'flex', alignItems: 'center', width: '100%' }}>
          <Box sx={{ mr: 2 }}>
            <Avatar
              sx={{
                width: 48,
                height: 48,
                borderRadius: '50%',
                display: 'flex',
              }}
              src={impersonatedUser?.image || impersonatedUser?.avatar}
              alt={impersonatedUser?.displayName || impersonatedUser?.name}
            />
          </Box>
          <Box sx={{ flex: 1 }}>
            <Typography level='title-md' sx={{ mb: 1, ml: 0.5 }}>
              {impersonatedUser?.displayName || impersonatedUser?.name}
            </Typography>
            {/* <Box sx={{ fontSize: 14, color: 'text.secondary', mb: 0.5 }}>
              5 chores assigned, 2 due soon
            </Box> */}
            <Box>
              <Button
                variant='plain'
                color='neutral'
                size='sm'
                onClick={() => {
                  setIsModalOpen(true)
                }}
              >
                Change User
              </Button>
              <Button
                variant='plain'
                color='neutral'
                size='sm'
                sx={{ ml: 0.5 }}
                onClick={() => {
                  setImpersonatedUser(null)
                }}
              >
                Cancel
              </Button>
            </Box>
          </Box>
        </Box>
      </Box>
      <UserModal
        isOpen={isModalOpen}
        performers={circleMembersData?.res}
        onSelect={user => {
          setImpersonatedUser(user)
          setIsModalOpen(false)
        }}
        onClose={() => {
          setIsModalOpen(false)
        }}
      />
    </Sheet>
  )
}
export default WelcomeCard
