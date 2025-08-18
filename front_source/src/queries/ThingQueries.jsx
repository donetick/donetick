import { useInfiniteQuery } from '@tanstack/react-query'
import { GetThingHistory } from '../utils/Fetcher'

export const useThingHistory = (thingId, limit = 10) => {
  return useInfiniteQuery({
    queryKey: ['thingHistory', thingId],
    queryFn: async ({ pageParam = 0 }) => {
      const response = await GetThingHistory(thingId, pageParam)
      if (!response.ok) {
        throw new Error('Failed to fetch thing history')
      }
      const data = await response.json()
      return data
    },
    getNextPageParam: (lastPage, allPages) => {
      // If the last page has fewer items than the limit, there are no more pages
      if (lastPage.res.length < limit) {
        return undefined
      }
      // Calculate the offset for the next page
      const totalItems = allPages.reduce(
        (acc, page) => acc + page.res.length,
        0,
      )
      return totalItems
    },
    enabled: !!thingId, // Only run query if thingId exists
  })
}
