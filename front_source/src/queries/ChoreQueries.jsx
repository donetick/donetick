import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { networkManager } from '../hooks/NetworkManager'
import {
  CreateChore,
  GetChoreByID,
  GetChoreDetailById,
  GetChoresHistory,
  GetChoresNew,
  SaveChore,
} from '../utils/Fetcher'
import { localStore } from '../utils/LocalStore'

export const useChores = includeArchive => {
  return useQuery({
    queryKey: ['chores', includeArchive],
    queryFn: async () => {
      const onlineChores = await GetChoresNew(includeArchive)

      const offlineTasks = (await localStore.getFromCache('offlineTasks')) || []
      // go throught each and if there is two chores with same id in offline and online, prefer the offline one:
      var finalChores = []
      if (onlineChores && onlineChores.res) {
        finalChores = onlineChores.res.filter(
          onlineChore =>
            !offlineTasks.some(offlineTask => {
              // Match by id or tempId
              return (
                String(onlineChore.id) === String(offlineTask.id) ||
                (offlineTask.tempId &&
                  String(onlineChore.id) === String(offlineTask.tempId))
              )
            }),
        )
      }
      // Combine online chores with offline tasks
      if (offlineTasks.length > 0) {
        // Merge the offline tasks with the online chores
        finalChores = [
          ...finalChores,
          ...offlineTasks.map(task => ({
            ...task,
            id: task.id || task.tempId, // Ensure we have an id for consistency
          })),
        ]
      }

      return { res: finalChores }

      // return { res: [...onlineChores.res, ...offlineTasks] }
    },
  })
}

export const useCreateChore = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: CreateChore,
    onMutate: async newTask => {
      if (!networkManager.isOnline) {
        const tempId = crypto.randomUUID() // Generate temp ID
        const offlineTasks =
          (await localStore.getFromCache('offlineTasks')) || []
        const updateOfflineTasks = [
          ...offlineTasks,
          { ...newTask, id: tempId, tempId }, // Use the tempId for offline tracking
        ]
        await localStore.saveToCache('offlineTasks', updateOfflineTasks) // Save to local storage
        // force useChores to refetch:
        queryClient.invalidateQueries(['chores'])
        // Force the chores query to refetch
        queryClient.refetchQueries(['chores'])
        // Update the chores query cache immediately
        // queryClient.setQueryData(['chores'], oldData => {
        //   console.log('ATTEMPT TO SAVE OFFLINE TASKS:', updateOfflineTasks)

        //   if (!oldData)
        //     return {
        //       res: [{ ...newTask, id: tempId, tempId }],
        //     } // If no data, return offline tasks
        //   return {
        //     res: [...oldData.res, { ...newTask, id: tempId, tempId }],
        //   }
        // })
        return { tempId }
      }
      return { tempId: null }
    },
  })
}

export const useUpdateChore = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async updatedChore => {
      if (!networkManager.isOnline) {
        updatedChore['updatedAt'] = new Date().toISOString()
        if (!updatedChore['nextDueDate']) {
          updatedChore['nextDueDate'] = updatedChore['dueDate']
        }
        const offlineTasks =
          (await localStore.getFromCache('offlineTasks')) || []

        for (const task of offlineTasks) {
          // Find the task with the same id or tempId and update it
          if (task.id === updatedChore.id || task.tempId === updatedChore.id) {
            // Update the task in local storage
            const updatedTask = { ...task, ...updatedChore }
            const updatedOfflineTasks = offlineTasks.map(t =>
              t.id === task.id ? updatedTask : t,
            )
            await localStore.saveToCache('offlineTasks', updatedOfflineTasks)
            return new Promise((resolve, reject) => {
              resolve(updatedTask)
            })
          }
        }
        const newTaskId = crypto.randomUUID()
        const updatedChoreWithNewId = {
          ...updatedChore,
          tempId: newTaskId,
        }

        await localStore.saveToCache('offlineTasks', [
          ...offlineTasks,
          updatedChoreWithNewId,
        ])
        return new Promise((resolve, reject) => {
          // Resolve with the updated task
          resolve(updatedChoreWithNewId)
        })
      } else {
        // Call the API to update the chore
        const resp = await SaveChore(updatedChore)
        if (!resp || !resp.ok) {
          throw new Error('Failed to save chore')
        }
        const updatedChoreRes = await resp.json()
        if (!updatedChoreRes) {
          throw new Error('Failed to get updated chore data')
        }
        // Successfully updated the chore on the server, return the updated chore
        return updatedChoreRes?.res || updatedChoreRes
      }
    },
    onSuccess: (data, variables) => {
      // Invalidate the chores query to refresh the data
      queryClient.invalidateQueries(['chores'])
    },
    onMutate: async updatedChore => {
      if (!networkManager.isOnline) {
        // Handle offline case here if needed
        return
      }
    },
  })
}

export const useChoresHistory = (initialLimit, includeMembers) => {
  const [limit, setLimit] = useState(initialLimit) // Initially, no limit is selected

  const { data, error, isLoading } = useQuery({
    queryKey: ['choresHistory', limit],
    queryFn: async () => {
      const resp = await GetChoresHistory(limit, includeMembers)
      return resp?.res || []
    },
  })

  const handleLimitChange = newLimit => {
    setLimit(newLimit)
  }

  return { data, error, isLoading, handleLimitChange }
}

export const useChoreDetails = choreId => {
  return useQuery({
    queryKey: ['choreDetails', choreId],
    queryFn: async () => {
      var onlineChore = null

      try {
        const response = await GetChoreDetailById(choreId)

        if (response && response.ok) {
          onlineChore = await response.json()
        }
      } catch (error) {
        console.error('Error fetching chore detail:', error)
      }

      const offlineTasks = (await localStore.getFromCache('offlineTasks')) || []
      const offline = offlineTasks.find(task => {
        // Match by tempId or id if it was created offline
        return task.id === choreId || (task.tempId && task.tempId === choreId)
      })

      return { res: offline ? { ...offline } : onlineChore.res }
    },
  })
}

export const useChore = choreId => {
  const queryClient = useQueryClient()

  return useQuery({
    queryKey: ['chore', choreId],
    queryFn: async () => {
      if (!choreId) {
        throw new Error('Chore ID is required to fetch chore details')
      }
      var onlineChore = null

      try {
        const response = await GetChoreByID(choreId)

        if (response && response.ok) {
          onlineChore = await response.json()
        }
      } catch (error) {
        console.error('Error fetching chore detail:', error)
      }

      const offlineTasks = (await localStore.getFromCache('offlineTasks')) || []
      const offline = offlineTasks.find(task => {
        return (
          String(task.id) === choreId ||
          (task.tempId && task.tempId === choreId)
        )
      })

      return { res: offline ? { ...offline } : onlineChore.res }
    },
    onSuccess: () => {
      queryClient.invalidateQueries(['chores'])
    },
  })
}
