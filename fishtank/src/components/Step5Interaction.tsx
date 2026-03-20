import { useEffect, useRef, useState } from 'react'
import { api } from '../api'
import '../styles/Step5Interaction.css'

type LogLine = { time: string; msg: string }

type ChatMsg = { role: 'user' | 'assistant'; content: string; time?: string }

type Props = {
  reportId?: string
  simulationId?: string | null
  systemLogs: LogLine[]
  onAddLog: (msg: string) => void
  onUpdateStatus: (s: 'processing' | 'completed' | 'error' | 'ready') => void
}

export default function Step5Interaction({ reportId, simulationId: simProp, onAddLog, onUpdateStatus }: Props) {
  const [simulationId, setSimulationId] = useState<string | null>(simProp ?? null)
  const [input, setInput] = useState('')
  const [messages, setMessages] = useState<ChatMsg[]>([])
  const [sending, setSending] = useState(false)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!reportId) return
    void (async () => {
      try {
        const res = await api.report.get(reportId)
        if (res.success && res.data) {
          const sid = (res.data as { simulation_id?: string }).simulation_id
          if (sid) setSimulationId(sid)
        }
      } catch {
        onAddLog('Could not load report metadata')
      }
    })()
  }, [reportId, onAddLog])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages.length])

  const send = async () => {
    const text = input.trim()
    if (!text || !simulationId || sending) return
    setInput('')
    setSending(true)
    onUpdateStatus('processing')
    const userMsg: ChatMsg = { role: 'user', content: text, time: new Date().toISOString() }
    setMessages((m) => [...m, userMsg])
    try {
      const history = messages.map((x) => ({ role: x.role, content: x.content }))
      const res = await api.report.chat({
        simulation_id: simulationId,
        message: text,
        chat_history: history
      })
      if (res.success && res.data) {
        const reply = (res.data as { response?: string; message?: string }).response || (res.data as { message?: string }).message || JSON.stringify(res.data)
        setMessages((m) => [...m, { role: 'assistant', content: String(reply) }])
        onUpdateStatus('ready')
      } else {
        setMessages((m) => [...m, { role: 'assistant', content: `Error: ${res.error || 'unknown'}` }])
        onUpdateStatus('error')
      }
    } catch (e: unknown) {
      const m = e instanceof Error ? e.message : String(e)
      setMessages((prev) => [...prev, { role: 'assistant', content: m }])
      onUpdateStatus('error')
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="interaction-panel">
      <div className="interaction-header">
        <h2>Report Agent Chat</h2>
        <span className="sub">Report: {reportId || '—'}</span>
      </div>
      <div className="chat-thread">
        {messages.map((msg, i) => (
          <div key={i} className={`chat-bubble ${msg.role}`}>
            <div className="bubble-role">{msg.role}</div>
            <div className="bubble-text">{msg.content}</div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
      <div className="chat-input-row">
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              void send()
            }
          }}
          placeholder={simulationId ? 'Message ReportAgent…' : 'Loading simulation id…'}
          disabled={!simulationId || sending}
          rows={3}
        />
        <button type="button" className="send-btn" disabled={!simulationId || sending} onClick={() => void send()}>
          {sending ? '…' : 'Send'}
        </button>
      </div>
    </div>
  )
}
