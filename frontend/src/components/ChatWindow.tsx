import { useEffect, useRef } from 'react'
import { useIsFetching } from '@tanstack/react-query'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useChatStore } from '../stores/chat'

function Spinner() {
  return (
    <div className="flex items-center justify-center py-8">
      <div className="w-6 h-6 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
    </div>
  )
}

export function ChatWindow() {
  const messages = useChatStore((s) => s.messages)
  const streams = useChatStore((s) => s.streams)
  const currentConversationId = useChatStore((s) => s.currentConversationId)
  const bottomRef = useRef<HTMLDivElement>(null)
  const fetching = useIsFetching({ queryKey: ['messages', currentConversationId] })

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streams])

  const streamEntries = Object.entries(streams).filter(
    ([_, s]) => s.content || s.active,
  )

  if (!currentConversationId && streamEntries.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-gray-400">
        <div className="text-center">
          <p className="text-lg mb-2">Select a conversation or start a new chat</p>
          <p className="text-sm">Choose a provider and model above, then type a message</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto p-4 space-y-4">
      {fetching > 0 && messages.length === 0 && <Spinner />}

      {messages.map((msg) => (
        <div
          key={msg.id}
          className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
        >
          <div
            className={`max-w-[70%] rounded-lg px-4 py-2 ${
              msg.role === 'user'
                ? 'bg-blue-600 text-white'
                : 'bg-gray-100 text-gray-900'
            }`}
          >
            {msg.role === 'user' ? (
              <p className="whitespace-pre-wrap text-sm">{msg.content}</p>
            ) : (
              <div className="prose prose-sm max-w-none">
                <Markdown remarkPlugins={[remarkGfm]}>
                  {msg.content}
                </Markdown>
              </div>
            )}
          </div>
        </div>
      ))}

      {streamEntries.map(([sessionId, stream]) => (
        <div key={sessionId} className="flex justify-start">
          <div className="max-w-[70%] rounded-lg px-4 py-2 bg-gray-100 text-gray-900">
            <div className="prose prose-sm max-w-none">
              <Markdown remarkPlugins={[remarkGfm]}>
                {stream.content}
              </Markdown>
            </div>
            {stream.active && (
              <span className="inline-block w-2 h-4 bg-gray-400 animate-pulse ml-0.5" />
            )}
          </div>
        </div>
      ))}

      <div ref={bottomRef} />
    </div>
  )
}
