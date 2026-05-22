import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import * as api from '../lib/api'
import { useChatStore } from '../stores/chat'

export function useConversations() {
  const setConversations = useChatStore((s) => s.setConversations)

  return useQuery({
    queryKey: ['conversations'],
    queryFn: async () => {
      const list = await api.listConversations()
      setConversations(list)
      return list
    },
    refetchInterval: 5000,
  })
}

export function useMessages(conversationId: string | null) {
  const setMessages = useChatStore((s) => s.setMessages)

  return useQuery({
    queryKey: ['messages', conversationId],
    queryFn: async () => {
      if (!conversationId) return []
      const msgs = await api.getMessages(conversationId)
      setMessages(msgs)
      return msgs
    },
    enabled: !!conversationId,
  })
}

export function useDeleteConversation() {
  const qc = useQueryClient()
  const setCurrentConversation = useChatStore((s) => s.setCurrentConversation)

  return useMutation({
    mutationFn: (id: string) => api.deleteConversation(id),
    onSuccess: () => {
      setCurrentConversation(null)
      qc.invalidateQueries({ queryKey: ['conversations'] })
    },
  })
}

export function useCancelConversation() {
  const qc = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => api.updateConversation(id, 'cancelled'),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['conversations'] })
    },
  })
}

export function useResumeConversation() {
  const qc = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => api.updateConversation(id, 'active'),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['conversations'] })
    },
  })
}
