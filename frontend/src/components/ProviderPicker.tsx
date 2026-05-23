import { useEffect, useState } from 'react'
import { useChatStore } from '../stores/chat'
import * as api from '../lib/api'
import type { Providers } from '../lib/schemas'

export function ProviderPicker() {
  const provider = useChatStore((s) => s.provider)
  const model = useChatStore((s) => s.model)
  const setProvider = useChatStore((s) => s.setProvider)
  const setModel = useChatStore((s) => s.setModel)
  const [providers, setProviders] = useState<Providers>({})
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.fetchProviders().then((data) => {
      setProviders(data)
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [])

  const models = providers[provider]?.models || []

  useEffect(() => {
    if (!loading && Object.keys(providers).length > 0 && !providers[provider]) {
      const first = Object.keys(providers)[0]
      setProvider(first)
      setModel(providers[first]?.models[0] || '')
    }
  }, [loading, providers, provider, setProvider, setModel])

  if (loading) return <div className="text-sm text-gray-400">Loading providers...</div>

  return (
    <div className="flex gap-2 items-center">
      <select
        className="border rounded px-2 py-1 text-sm bg-white"
        value={providers[provider] ? provider : ''}
        onChange={(e) => {
          const p = e.target.value
          setProvider(p)
          setModel(providers[p]?.models[0] || '')
        }}
      >
        {Object.keys(providers).map((name) => (
          <option key={name} value={name}>
            {name}
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
