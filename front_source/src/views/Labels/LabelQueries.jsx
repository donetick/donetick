import { useQuery } from '@tanstack/react-query'
import { CreateLabel, GetLabels } from '../../utils/Fetcher'

export const useLabels = () => {
  return useQuery({
    queryKey: ['labels'],
    queryFn: GetLabels,
  })
}

export const useCreateLabel = () => {
  return useQuery({
    queryKey: ['createLabel'],
    queryFn: CreateLabel,
  })
}
