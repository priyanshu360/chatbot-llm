import { useChatStore } from '../stores/chat'
import { useConversations, useDeleteConversation, useCancelConversation, useResumeConversation } from '../hooks/useConversations'
import * as api from '../lib/api'

export function Sidebar() {
  const { data: conversations, isLoading, isError } = useConversations()
  const currentConversationId = useChatStore((s) => s.currentConversationId)
  const setCurrentConversation = useChatStore((s) => s.setCurrentConversation)
  const setMessages = useChatStore((s) => s.setMessages)

  const deleteMutation = useDeleteConversation()
  const cancelMutation = useCancelConversation()
  const resumeMutation = useResumeConversation()

  const handleSelect = async (id: string) => {
    setCurrentConversation(id)
    try {
      const msgs = await api.getMessages(id)
      setMessages(msgs)
    } catch {
      // messages will be empty on error
    }
  }

  const handleNew = () => {
    setCurrentConversation(null)
    setMessages([])
  }

  return (
    <div className="w-72 bg-gray-50 border-r h-full flex flex-col">
      <div className="p-3 border-b">
        <button
          onClick={handleNew}
          className="w-full px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium"
        >
          + New Conversation
        </button>
      </div>

      <div className="flex-1 overflow-y-auto">
        {isLoading && (
          <div className="flex items-center justify-center gap-2 p-4 text-sm text-gray-400">
            <div className="w-4 h-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin" />
            Loading...
          </div>
        )}

        {isError && (
          <div className="p-3 text-sm text-red-500 text-center bg-red-50 mx-2 mt-2 rounded">
            Failed to load conversations
          </div>
        )}

        {conversations?.map((conv) => (
          <div
            key={conv.id}
            className={`group p-3 border-b cursor-pointer hover:bg-gray-100 transition-colors ${
              conv.id === currentConversationId ? 'bg-blue-50 border-l-4 border-l-blue-600' : ''
            }`}
            onClick={() => handleSelect(conv.id)}
          >
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs font-medium text-gray-500 uppercase">
                {conv.provider}
              </span>
              <span
                className={`text-xs px-1.5 py-0.5 rounded ${
                  conv.status === 'active'
                    ? 'bg-green-100 text-green-700'
                    : 'bg-gray-200 text-gray-600'
                }`}
              >
                {conv.status}
              </span>
            </div>
            <p className="text-sm text-gray-800 truncate">
              {conv.title || 'Untitled'}
            </p>
            <p className="text-xs text-gray-400 mt-1">
              {new Date(conv.updated_at).toLocaleDateString()}
            </p>

            <div className="hidden group-hover:flex gap-1 mt-2">
              {conv.status === 'active' ? (
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    cancelMutation.mutate(conv.id)
                  }}
                  className="text-xs px-2 py-0.5 bg-yellow-100 text-yellow-700 rounded hover:bg-yellow-200"
                >
                  Cancel
                </button>
              ) : (
                <button
                  onClick={(e) => {
                    e.stopPropagation()
                    resumeMutation.mutate(conv.id)
                  }}
                  className="text-xs px-2 py-0.5 bg-green-100 text-green-700 rounded hover:bg-green-200"
                >
                  Resume
                </button>
              )}
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  if (confirm('Delete this conversation?')) {
                    deleteMutation.mutate(conv.id)
                  }
                }}
                className="text-xs px-2 py-0.5 bg-red-100 text-red-700 rounded hover:bg-red-200"
              >
                Delete
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
