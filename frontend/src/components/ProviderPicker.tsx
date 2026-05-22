import { useChatStore } from '../stores/chat'

const PROVIDERS: Record<string, { models: string[] }> = {
  openai: { models: ['gpt-4o', 'gpt-4o-mini'] },
  anthropic: { models: ['claude-sonnet-4-20250514', 'claude-3-5-haiku-latest'] },
  gemini: { models: ['gemini-2.5-flash', 'gemini-1.5-flash'] },
}

export function ProviderPicker() {
  const provider = useChatStore((s) => s.provider)
  const model = useChatStore((s) => s.model)
  const setProvider = useChatStore((s) => s.setProvider)
  const setModel = useChatStore((s) => s.setModel)

  const models = PROVIDERS[provider]?.models || []

  return (
    <div className="flex gap-2 items-center">
      <select
        className="border rounded px-2 py-1 text-sm bg-white"
        value={provider}
        onChange={(e) => {
          const p = e.target.value
          setProvider(p)
          setModel(PROVIDERS[p]?.models[0] || '')
        }}
      >
        {Object.keys(PROVIDERS).map((p) => (
          <option key={p} value={p}>
            {p}
          </option>
        ))}
      </select>
      <select
        className="border rounded px-2 py-1 text-sm bg-white"
        value={model}
        onChange={(e) => setModel(e.target.value)}
      >
        {models.map((m) => (
          <option key={m} value={m}>
            {m}
          </option>
        ))}
      </select>
    </div>
  )
}
