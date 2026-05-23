import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Message, Conversation } from '../lib/schemas'

interface StreamState {
  content: string
  active: boolean
  abortController: AbortController | null
}

interface ChatState {
  conversations: Conversation[]
  currentConversationId: string | null
  messages: Message[]
  streams: Record<string, StreamState>
  provider: string
  model: string

  setConversations: (conversations: Conversation[]) => void
  setCurrentConversation: (id: string | null) => void
  setMessages: (messages: Message[]) => void
  addMessage: (message: Message) => void
  setProvider: (provider: string) => void
  setModel: (model: string) => void
  setStreamState: (sessionId: string, state: Partial<StreamState>) => void
  clearStreamState: (sessionId: string) => void
  appendToStream: (sessionId: string, delta: string) => void
  setStreamDone: (sessionId: string) => void
  cancelAllStreams: () => void
}

export const useChatStore = create<ChatState>()(
  persist(
    (set) => ({
      conversations: [],
      currentConversationId: null,
      messages: [],
      streams: {},
      provider: 'gemini',
      model: 'gemini-2.5-flash',

      setConversations: (conversations) => set({ conversations }),
      setCurrentConversation: (id) => set({ currentConversationId: id }),
      setMessages: (messages) => set({ messages }),
      addMessage: (message) => set((state) => ({ messages: [...state.messages, message] })),
      setProvider: (provider) => set({ provider }),
      setModel: (model) => set({ model }),

      setStreamState: (sessionId, state) =>
        set((prev) => ({
          streams: {
            ...prev.streams,
            [sessionId]: { ...prev.streams[sessionId], ...state },
          },
        })),

      clearStreamState: (sessionId) =>
        set((prev) => {
          const { [sessionId]: _, ...rest } = prev.streams
          return { streams: rest }
        }),

      appendToStream: (sessionId, delta) =>
        set((prev) => {
          const existing = prev.streams[sessionId]?.content || ''
          return {
            streams: {
              ...prev.streams,
              [sessionId]: {
                content: existing + delta,
                active: true,
                abortController: prev.streams[sessionId]?.abortController || null,
              },
            },
          }
        }),

      setStreamDone: (sessionId) =>
        set((prev) => ({
          streams: {
            ...prev.streams,
            [sessionId]: {
              ...prev.streams[sessionId],
              active: false,
              abortController: null,
            },
          },
        })),

      cancelAllStreams: () =>
        set((state) => {
          Object.values(state.streams).forEach((s) => s.abortController?.abort())
          return { streams: {} }
        }),
    }),
    {
      name: 'chat-store',
      partialize: (state) => ({ provider: state.provider, model: state.model }),
    },
  ),
)
