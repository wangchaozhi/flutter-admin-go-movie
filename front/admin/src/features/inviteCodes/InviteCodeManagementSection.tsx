import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Ban, Check, Copy, Loader, Ticket } from 'lucide-react'

import type { ApiResponse, InviteCode } from '../../adminTypes'
import { PanelTitle } from '../../components/shared'
import { useI18n } from '../../i18n'

type InviteForm = {
  code: string
  max_uses: number
  note: string
  expires_at: string
}

const emptyForm: InviteForm = { code: '', max_uses: 1, note: '', expires_at: '' }

export function InviteCodeManagementSection({
  token,
  can,
}: {
  token: string
  can: (permission: string) => boolean
}) {
  const { t } = useI18n()
  const [codes, setCodes] = useState<InviteCode[]>([])
  const [form, setForm] = useState<InviteForm>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState('')

  const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }

  async function load() {
    try {
      const res = await fetch('/api/admin/invite-codes', { headers })
      const json: ApiResponse<InviteCode[]> = await res.json()
      if (json.code === 0) setCodes(json.data ?? [])
      else setError(json.msg || t('invite.loadFailed'))
    } catch {
      setError(t('invite.loadFailed'))
    }
  }

  // mount-only load; `load` is recreated each render so it stays out of deps
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [])

  async function generate(event: FormEvent) {
    event.preventDefault()
    setSaving(true)
    setError('')
    try {
      const body: Record<string, unknown> = {
        code: form.code.trim(),
        max_uses: Number.isFinite(form.max_uses) ? form.max_uses : 0,
        note: form.note.trim(),
      }
      if (form.expires_at) body.expires_at = new Date(form.expires_at).toISOString()
      const res = await fetch('/api/admin/invite-codes', {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
      })
      const json: ApiResponse<InviteCode> = await res.json()
      if (json.code !== 0) {
        setError(json.msg || t('invite.generateFailed'))
        return
      }
      setForm(emptyForm)
      await load()
    } catch {
      setError(t('invite.generateFailed'))
    } finally {
      setSaving(false)
    }
  }

  async function setStatus(code: InviteCode, action: 'disable' | 'enable') {
    setSaving(true)
    try {
      await fetch(`/api/admin/invite-codes/${code.id}/${action}`, { method: 'POST', headers })
      await load()
    } finally {
      setSaving(false)
    }
  }

  async function copyCode(code: string) {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(code)
      setTimeout(() => setCopied(''), 1500)
    } catch {
      /* clipboard unavailable: ignore */
    }
  }

  const canCreate = can('invite:create')
  const canDisable = can('invite:disable')

  return (
    <section className="content-grid">
      <section className="table-panel">
        <PanelTitle title={t('invite.title')} count={codes.length} />
        {error && <span className="status error">{error}</span>}
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{t('invite.code')}</th>
                <th>{t('invite.uses')}</th>
                <th>{t('common.status')}</th>
                <th>{t('invite.expiresAt')}</th>
                <th>{t('invite.note')}</th>
                <th>{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {codes.map((code) => (
                <tr key={code.id}>
                  <td>
                    <button
                      type="button"
                      className="secondary"
                      title={copied === code.code ? t('invite.copied') : t('invite.copy')}
                      onClick={() => copyCode(code.code)}
                    >
                      <strong>{code.code}</strong>{' '}
                      {copied === code.code ? <Check size={13} /> : <Copy size={13} />}
                    </button>
                  </td>
                  <td className="text-faint">
                    {t('invite.usesValue', {
                      used: code.used_count,
                      max: code.max_uses === 0 ? t('common.unlimited') : code.max_uses,
                    })}
                  </td>
                  <td className={code.status === 'active' ? '' : 'text-faint'}>
                    {code.status === 'active' ? t('invite.statusActive') : t('invite.statusDisabled')}
                  </td>
                  <td className="text-faint">
                    {code.expires_at ? new Date(code.expires_at).toLocaleString() : t('common.never')}
                  </td>
                  <td className="text-faint">{code.note || t('common.none')}</td>
                  <td>
                    {canDisable && (
                      <div className="row-actions">
                        {code.status === 'active' ? (
                          <button className="danger" type="button" disabled={saving} onClick={() => setStatus(code, 'disable')}>
                            <Ban size={13} /> {t('common.disable')}
                          </button>
                        ) : (
                          <button type="button" disabled={saving} onClick={() => setStatus(code, 'enable')}>
                            <Check size={13} /> {t('common.enable')}
                          </button>
                        )}
                      </div>
                    )}
                  </td>
                </tr>
              ))}
              {codes.length === 0 && (
                <tr>
                  <td colSpan={6} className="text-faint">{t('invite.empty')}</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {canCreate && (
        <div className="editor-panel">
          <form onSubmit={generate}>
            <PanelTitle title={t('invite.generate')} />
            <label>
              {t('invite.codeLabel')}
              <input
                value={form.code}
                placeholder={t('invite.codePlaceholder')}
                onChange={(event) => setForm({ ...form, code: event.target.value })}
              />
            </label>
            <label>
              {t('invite.maxUses')}
              <input
                type="number"
                min={0}
                value={form.max_uses}
                onChange={(event) => setForm({ ...form, max_uses: Number(event.target.value) })}
                placeholder="1"
              />
            </label>
            <label>
              {t('invite.expiresAt')}
              <input
                type="datetime-local"
                value={form.expires_at}
                onChange={(event) => setForm({ ...form, expires_at: event.target.value })}
              />
            </label>
            <label>
              {t('invite.note')}
              <input
                value={form.note}
                placeholder={t('invite.notePlaceholder')}
                onChange={(event) => setForm({ ...form, note: event.target.value })}
              />
            </label>
            <div className="form-actions">
              <button className="primary-button" type="submit" disabled={saving}>
                {saving ? <Loader size={14} className="spin" /> : <Ticket size={14} />}
                {t('invite.generate')}
              </button>
              <button type="button" className="secondary" onClick={() => setForm(emptyForm)}>
                {t('common.cancel')}
              </button>
            </div>
          </form>
        </div>
      )}
    </section>
  )
}
