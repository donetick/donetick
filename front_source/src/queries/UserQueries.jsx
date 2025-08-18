import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  GetAllCircleMembers,
  GetAllUsers,
  GetUserProfile,
} from '../utils/Fetcher'
import { isTokenValid } from '../utils/TokenManager'

export const useAllUsers = () => {
  return useQuery({
    queryKey: ['allUsers'],
    queryFn: GetAllUsers,
  })
}

export const useCircleMembers = () => {
  const queryClient = useQueryClient()

  const { data, error, isLoading } = useQuery({
    queryKey: ['allCircleMembers'],
    queryFn: GetAllCircleMembers,
  })

  const handleRefetch = () => {
    queryClient.invalidateQueries(['allCircleMembers'])
  }

  return { data, error, isLoading, handleRefetch }
}

export const useUserProfile = () => {
  const queryClient = useQueryClient()

  const { data, error, isLoading } = useQuery({
    queryKey: ['userProfile'],
    queryFn: async () => {
      if (!isTokenValid()) {
        return null // Token is invalid, return null to indicate no profile
      }
      const resp = await GetUserProfile()
      const result = await resp.json()

      return result.res // Return the actual user profile data
    },
    staleTime: 30 * 60 * 1000, // 30 minutes in milliseconds
    gcTime: 30 * 60 * 1000, // 30 minutes in milliseconds
  })
  return {
    data,
    error,
    isLoading,
    refetch: () => queryClient.invalidateQueries(['userProfile']),
  }
}
