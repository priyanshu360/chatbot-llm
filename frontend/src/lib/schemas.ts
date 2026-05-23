import { z } from 'zod'

export const ConversationSchema = z.object({
  id: z.string().uuid(),
  title: z.string(),
  provider: z.string(),
  model: z.string(),
  status: z.enum(['active', 'cancelled']),
  created_at: z.string(),
  updated_at: z.string(),
})
export type Conversation = z.infer<typeof ConversationSchema>

export const MessageSchema = z.object({
  id: z.string().uuid(),
  conversation_id: z.string().uuid(),
  role: z.enum(['user', 'assistant', 'system']),
  content: z.string(),
  seq: z.number().int().nonnegative(),
  created_at: z.string(),
})
export type Message = z.infer<typeof MessageSchema>

export const ChatRequestSchema = z.object({
  conversation_id: z.string().uuid().optional(),
  provider: z.string().min(1),
  model: z.string().min(1),
  message: z.string().min(1),
})
export type ChatRequest = z.infer<typeof ChatRequestSchema>

export const ChatMetaSchema = z.object({
  conversation_id: z.string().uuid(),
  message_id: z.string().uuid(),
})
export type ChatMeta = z.infer<typeof ChatMetaSchema>

export const ChatDoneSchema = z.object({
  content: z.string(),
  usage: z.object({
    input_tokens: z.number().int().nonnegative(),
    output_tokens: z.number().int().nonnegative(),
  }).nullable().optional(),
})
export type ChatDone = z.infer<typeof ChatDoneSchema>

export const CreateConversationSchema = z.object({
  title: z.string().default(''),
  provider: z.string().min(1),
  model: z.string().min(1),
})

export const ProviderSchema = z.record(z.string(), z.object({
  models: z.array(z.string()),
}))
export type Providers = z.infer<typeof ProviderSchema>

export const UpdateConversationSchema = z.object({
  status: z.enum(['active', 'cancelled']),
})

export const SSEEventSchemas = {
  meta: ChatMetaSchema,
  token: z.object({ delta: z.string() }),
  done: ChatDoneSchema,
  error: z.object({ error: z.string() }),
} as const

export function parseSSEData<T extends keyof typeof SSEEventSchemas>(
  eventType: T,
  raw: string,
): z.infer<(typeof SSEEventSchemas)[T]> | null {
  try {
    const parsed = JSON.parse(raw)
    const schema = SSEEventSchemas[eventType]
    return schema.parse(parsed) as any
  } catch {
    return null
  }
}

export function parseResponse<T>(schema: z.ZodSchema<T>, data: unknown): T {
  return schema.parse(data)
}

export function safeParseResponse<T>(schema: z.ZodSchema<T>, data: unknown): { success: true; data: T } | { success: false; error: string } {
  const result = schema.safeParse(data)
  if (result.success) return { success: true, data: result.data }
  const issues = (result.error as any).issues || []
  return { success: false, error: issues.map((e: any) => `${e.path.join('.')}: ${e.message}`).join('; ') }
}
