import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Sidebar } from './components/Sidebar'
import { ChatWindow } from './components/ChatWindow'
import { ChatInput } from './components/ChatInput'
import { ProviderPicker } from './components/ProviderPicker'
import { useChatStream } from './hooks/useChatStream'
import { useChatStore } from './stores/chat'

const queryClient = new QueryClient()

function ChatLayout() {
  const { sendMessage, cancel } = useChatStream()
  const streams = useChatStore((s) => s.streams)
  const isStreaming = Object.values(streams).some((s) => s.active)

  return (
    <div className="h-screen flex flex-col">
      <header className="border-b px-4 py-2 bg-white flex items-center justify-between">
        <h1 className="text-lg font-semibold text-gray-800">LLM Chat</h1>
        <ProviderPicker />
      </header>

      <div className="flex-1 flex overflow-hidden min-h-0">
        <Sidebar />
        <div className="flex-1 flex flex-col min-h-0">
          <ChatWindow />
          <ChatInput
            onSend={(msg) => sendMessage(msg)}
            onCancel={cancel}
            isStreaming={isStreaming}
          />
        </div>
      </div>
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ChatLayout />
    </QueryClientProvider>
  )
}
