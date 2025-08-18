import { useQuery } from '@tanstack/react-query'
import { GetResource } from '../utils/Fetcher'

export const useResource = () => {
  const { data, isLoading, error } = useQuery({
    queryKey: [],
    queryFn: async () => {
      const response = await GetResource()
      return response
    },
    staleTime: 6 * 60 * 60 * 1000, // 6 hours in milliseconds
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })
  return { data, isLoading, error }
}
