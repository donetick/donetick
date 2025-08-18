import { useMutation, useQueryClient } from '@tanstack/react-query'
import { networkManager } from '../hooks/NetworkManager'
import { CompleteSubTask, SaveChore } from '../utils/Fetcher'
import { localStore } from '../utils/LocalStore'

export const useUpdate = () => {
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

export const useCompleteSubTask = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (subTaskId, choreId, completedAt) => {
      if (!networkManager.isOnline) {
        throw new Error('Cannot complete subtask while offline')
      }
      const resp = await CompleteSubTask(subTaskId, choreId, completedAt)
      if (!resp || !resp.ok) {
        throw new Error('Failed to complete subtask')
      }
      const result = await resp.json()
      if (!result || !result.res) {
        throw new Error('Failed to get completed subtask data')
      }
      return result.res
    },
    onSuccess: (data, variables) => {},
  })
}
