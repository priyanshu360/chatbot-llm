import { useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import * as api from '../lib/api'
import { useChatStore } from '../stores/chat'

export function useChatStream() {
  const qc = useQueryClient()
  const {
    currentConversationId,
    setCurrentConversation,
    setStreamState,
    clearStreamState,
    appendToStream,
    setStreamDone,
    addMessage,
    provider,
    model,
  } = useChatStore()

  const sendMessage = useCallback(
    (message: string, conversationId?: string) => {
      const sessionId = crypto.randomUUID()
      setStreamState(sessionId, { content: '', active: true, abortController: null })

      const controller = api.streamChat(
        {
          conversation_id: conversationId || currentConversationId || undefined,
          provider,
          model,
          message,
        },
        {
          onMeta: (meta) => {
            setCurrentConversation(meta.conversation_id)
            addMessage({
              id: meta.message_id,
              conversation_id: meta.conversation_id,
              role: 'user',
              content: message,
              seq: 0,
              created_at: new Date().toISOString(),
            })
            qc.invalidateQueries({ queryKey: ['conversations'] })
            qc.invalidateQueries({ queryKey: ['messages', meta.conversation_id] })
          },
          onToken: (delta) => {
            appendToStream(sessionId, delta)
          },
          onDone: () => {
            setStreamDone(sessionId)
            qc.invalidateQueries({ queryKey: ['messages', currentConversationId] })
          },
          onError: () => {
            setStreamDone(sessionId)
          },
        },
      )

      setStreamState(sessionId, { abortController: controller })

      return sessionId
    },
    [currentConversationId, provider, model, setCurrentConversation, setStreamState, appendToStream, setStreamDone, addMessage, qc],
  )

  const cancel = useCallback(() => {
    const activeStreams = useChatStore.getState().streams
    for (const [sessionId, state] of Object.entries(activeStreams)) {
      state.abortController?.abort()
      clearStreamState(sessionId)
    }
  }, [clearStreamState])

  return { sendMessage, cancel }
}
