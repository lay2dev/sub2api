import type { OpsSystemLog } from '@/api/admin/ops'

const getExtraString = (extra: Record<string, any> | undefined, key: string) => {
  if (!extra) return ''
  const v = extra[key]
  if (v == null) return ''
  if (typeof v === 'string') return v.trim()
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  return ''
}

const getExtraStringList = (extra: Record<string, any> | undefined, key: string) => {
  if (!extra) return ''
  const v = extra[key]
  if (!Array.isArray(v)) return ''

  const items = v
    .map((item) => {
      if (typeof item === 'string') return item.trim()
      if (typeof item === 'number' || typeof item === 'boolean') return String(item)
      return ''
    })
    .filter(Boolean)

  return items.join(',')
}

const getExtraJSON = (extra: Record<string, any> | undefined, key: string) => {
  if (!extra) return ''
  const value = extra[key]
  if (value == null) return ''

  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return ''
    try {
      return JSON.stringify(JSON.parse(trimmed))
    } catch {
      return trimmed
    }
  }

  try {
    return JSON.stringify(value)
  } catch {
    return ''
  }
}

export const buildSystemLogDetail = (row: OpsSystemLog) => {
  const parts: string[] = []
  const msg = String(row.message || '').trim()
  if (msg) parts.push(msg)

  const extra = row.extra || {}
  const statusCode = getExtraString(extra, 'status_code')
  const latencyMs = getExtraString(extra, 'latency_ms')
  const method = getExtraString(extra, 'method')
  const path = getExtraString(extra, 'path')
  const clientIP = getExtraString(extra, 'client_ip')
  const protocol = getExtraString(extra, 'protocol')

  const accessParts: string[] = []
  if (statusCode) accessParts.push(`status=${statusCode}`)
  if (latencyMs) accessParts.push(`latency_ms=${latencyMs}`)
  if (method) accessParts.push(`method=${method}`)
  if (path) accessParts.push(`path=${path}`)
  if (clientIP) accessParts.push(`ip=${clientIP}`)
  if (protocol) accessParts.push(`proto=${protocol}`)
  if (accessParts.length > 0) parts.push(accessParts.join(' '))

  const corrParts: string[] = []
  if (row.request_id) corrParts.push(`req=${row.request_id}`)
  if (row.client_request_id) corrParts.push(`client_req=${row.client_request_id}`)
  if (row.user_id != null) corrParts.push(`user=${row.user_id}`)
  if (row.account_id != null) corrParts.push(`acc=${row.account_id}`)
  if (row.platform) corrParts.push(`platform=${row.platform}`)
  if (row.model) corrParts.push(`model=${row.model}`)
  if (corrParts.length > 0) parts.push(corrParts.join(' '))

  const cryptoPrefetch = getExtraString(extra, 'crypto_prefetch')
  const fallbackToUpstream = getExtraString(extra, 'fallback_to_upstream')
  const prefetchTransport = getExtraString(extra, 'prefetch_transport')
  const upstreamRequestID = getExtraString(extra, 'upstream_request_id')
  const accountName = getExtraString(extra, 'account_name')
  const cryptoAdapterNames = getExtraStringList(extra, 'crypto_adapter_names')
  const toolCalls = getExtraJSON(extra, 'tool_calls')
  const upstreamURL = getExtraString(extra, 'upstream_url')
  const upstreamPath = getExtraString(extra, 'upstream_path')
  const openAIPassthrough = getExtraString(extra, 'openai_passthrough')
  const upstreamRequestBody = getExtraString(extra, 'upstream_request_body')
  const upstreamRequestBodyTruncated = getExtraString(extra, 'upstream_request_body_truncated')

  const cryptoParts: string[] = []
  if (cryptoPrefetch) cryptoParts.push(`crypto_prefetch=${cryptoPrefetch}`)
  if (fallbackToUpstream) cryptoParts.push(`fallback_to_upstream=${fallbackToUpstream}`)
  if (prefetchTransport) cryptoParts.push(`prefetch_transport=${prefetchTransport}`)
  if (upstreamRequestID) cryptoParts.push(`upstream_request_id=${upstreamRequestID}`)
  if (accountName) cryptoParts.push(`account_name=${accountName}`)
  if (cryptoAdapterNames) cryptoParts.push(`crypto_adapter_names=${cryptoAdapterNames}`)
  if (toolCalls) cryptoParts.push(`tool_calls=${toolCalls}`)
  if (cryptoParts.length > 0) parts.push(cryptoParts.join(' '))

  const outboundParts: string[] = []
  const hasOutboundInfo = Boolean(
    upstreamURL || upstreamPath || openAIPassthrough || upstreamRequestBody || upstreamRequestBodyTruncated,
  )
  if (hasOutboundInfo) {
    if (accountName) outboundParts.push(`account_name=${accountName}`)
    if (upstreamURL) outboundParts.push(`upstream_url=${upstreamURL}`)
    if (upstreamPath) outboundParts.push(`upstream_path=${upstreamPath}`)
    if (method) outboundParts.push(`method=${method}`)
    if (openAIPassthrough) outboundParts.push(`openai_passthrough=${openAIPassthrough}`)
    if (upstreamRequestBodyTruncated) outboundParts.push(`upstream_request_body_truncated=${upstreamRequestBodyTruncated}`)
    if (upstreamRequestBody) outboundParts.push(`upstream_request_body=${upstreamRequestBody}`)
    if (outboundParts.length > 0) parts.push(outboundParts.join(' '))
  }

  const errors = getExtraString(extra, 'errors')
  if (errors) parts.push(`errors=${errors}`)
  const err = getExtraString(extra, 'err') || getExtraString(extra, 'error')
  if (err) parts.push(`error=${err}`)

  return parts.join('  ')
}
