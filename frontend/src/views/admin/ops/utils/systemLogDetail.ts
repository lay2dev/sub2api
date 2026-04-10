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

  const cryptoParts: string[] = []
  if (cryptoPrefetch) cryptoParts.push(`crypto_prefetch=${cryptoPrefetch}`)
  if (fallbackToUpstream) cryptoParts.push(`fallback_to_upstream=${fallbackToUpstream}`)
  if (prefetchTransport) cryptoParts.push(`prefetch_transport=${prefetchTransport}`)
  if (upstreamRequestID) cryptoParts.push(`upstream_request_id=${upstreamRequestID}`)
  if (accountName) cryptoParts.push(`account_name=${accountName}`)
  if (cryptoAdapterNames) cryptoParts.push(`crypto_adapter_names=${cryptoAdapterNames}`)
  if (cryptoParts.length > 0) parts.push(cryptoParts.join(' '))

  const errors = getExtraString(extra, 'errors')
  if (errors) parts.push(`errors=${errors}`)
  const err = getExtraString(extra, 'err') || getExtraString(extra, 'error')
  if (err) parts.push(`error=${err}`)

  return parts.join('  ')
}
