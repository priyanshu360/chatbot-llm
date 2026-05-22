import {
  ConversationSchema,
  MessageSchema,
  ChatRequestSchema,
  CreateConversationSchema,
  UpdateConversationSchema,
  SSEEventSchemas,
  safeParseResponse,
  type Conversation,
  type Message,
  type ChatRequest,
  type ChatMeta,
  type ChatDone,
} from './schemas'

const BASE = '/api'

export type SSEHandler = {
  onMeta?: (meta: ChatMeta) => void
  onToken?: (delta: string) => void
  onDone?: (data: ChatDone) => void
  onError?: (error: string) => void
}

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error((body as any).error || `HTTP ${res.status}`)
  }
  return res.json()
}

export async function listConversations(): Promise<Conversation[]> {
  const data = await fetchJSON<unknown>(`${BASE}/conversations`)
  const result = safeParseResponse(ConversationSchema.array(), data)
  if (!result.success) throw new Error(result.error)
  return result.data
}

export async function getConversation(id: string): Promise<Conversation> {
  const data = await fetchJSON<unknown>(`${BASE}/conversations/${id}`)
  const result = safeParseResponse(ConversationSchema, data)
  if (!result.success) throw new Error(result.error)
  return result.data
}

export async function getMessages(conversationId: string): Promise<Message[]> {
  const data = await fetchJSON<unknown>(`${BASE}/conversations/${conversationId}/messages`)
  const result = safeParseResponse(MessageSchema.array(), data)
  if (!result.success) throw new Error(result.error)
  return result.data
}

export async function createConversation(title: string, provider: string, model: string): Promise<Conversation> {
  const body = CreateConversationSchema.parse({ title, provider, model })
  const data = await fetchJSON<unknown>(`${BASE}/conversations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const result = safeParseResponse(ConversationSchema, data)
  if (!result.success) throw new Error(result.error)
  return result.data
}

export async function updateConversation(id: string, status: string): Promise<void> {
  const body = UpdateConversationSchema.parse({ status })
  const res = await fetch(`${BASE}/conversations/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error('failed to update conversation')
}

export async function deleteConversation(id: string): Promise<void> {
  const res = await fetch(`${BASE}/conversations/${id}`, { method: 'DELETE' })
  if (!res.ok) throw new Error('failed to delete conversation')
}

export function streamChat(req: ChatRequest, handlers: SSEHandler): AbortController {
  const controller = new AbortController()

  const validated = ChatRequestSchema.parse(req)

  fetch(`${BASE}/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(validated),
    signal: controller.signal,
  }).then(async (response) => {
    if (!response.ok) {
      const err = await response.json().catch(() => ({ error: 'request failed' }))
      handlers.onError?.(err.error || `HTTP ${response.status}`)
      return
    }

    const reader = response.body?.getReader()
    if (!reader) {
      handlers.onError?.('no response body')
      return
    }

    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      let lastEvent = ''
      for (const line of lines) {
        if (line.startsWith('event: ')) {
          lastEvent = line.slice(7).trim()
        } else if (line.startsWith('data: ') && lastEvent) {
          const raw = line.slice(6)
          const eventType = lastEvent
          lastEvent = ''

          if (!(eventType in SSEEventSchemas)) continue

          try {
            const parsed = JSON.parse(raw)
            const schema = SSEEventSchemas[eventType as keyof typeof SSEEventSchemas]
            const data = schema.parse(parsed)

            switch (eventType as keyof typeof SSEEventSchemas) {
              case 'meta':
                handlers.onMeta?.(data as any)
                break
              case 'token':
                handlers.onToken?.((data as any).delta)
                break
              case 'done':
                handlers.onDone?.(data as any)
                break
              case 'error':
                handlers.onError?.((data as any).error)
                break
            }
          } catch {
            // skip unparseable or schema-invalid lines
          }
        }
      }
    }
  }).catch((err) => {
    if (err.name !== 'AbortError') {
      handlers.onError?.(err.message)
    }
  })

  return controller
}
