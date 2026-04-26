'use client'

import { useEffect, useState } from 'react'
import { trackEvent } from '@lib/telemetry'
import { api } from '@lib/api/client'

const summaryCardsDefault = [
  { label: 'Сегодня', value: '$124.80', hint: 'Начислено по финализированному NGR' },
  { label: 'За месяц', value: '$2,481.50', hint: 'После бонусов, fees, chargebacks и taxes' },
  { label: 'В ожидании', value: '$612.20', hint: 'Удержание до конца hold period' },
  { label: 'Доступно', value: '$438.00', hint: 'Можно отправить payout request' },
]

const funnelDefault = [
  { label: 'Клики', value: '12 481' },
  { label: 'Регистрации', value: '684' },
  { label: 'FTD', value: '179' },
  { label: 'Активные игроки', value: '96' },
]

const earningsDefault = [
  { period: '2026-04-15', ggr: '$310.20', ngr: '$198.40', commission: '$39.68', status: 'available' },
  { period: '2026-04-14', ggr: '$280.00', ngr: '$140.10', commission: '$28.02', status: 'pending' },
  { period: '2026-04-13', ggr: '$154.90', ngr: '$0.00', commission: '$0.00', status: 'released_zero_floor' },
]

const linksDefault = [
  {
    name: 'Main Campaign',
    code: 'AFF-SPORTS-01',
    url: 'https://dod.example/r/AFF-SPORTS-01/main',
    utm: 'utm_source=telegram&utm_medium=influencer&utm_campaign=main',
  },
  {
    name: 'Casino Stream',
    code: 'AFF-CASINO-07',
    url: 'https://dod.example/r/AFF-CASINO-07/stream',
    utm: 'utm_source=youtube&utm_medium=stream&utm_campaign=casino_stream',
  },
]

const payoutsDefault = [
  { id: 'pay_001', amount: '$250.00', status: 'paid', date: '2026-04-01', method: 'USDT TRC20' },
  { id: 'pay_002', amount: '$180.00', status: 'reviewing', date: '2026-04-15', method: 'Bank transfer' },
]

export function AffiliatePage() {
  const [summaryCards, setSummaryCards] = useState(summaryCardsDefault)
  const [funnel, setFunnel] = useState(funnelDefault)
  const [earnings, setEarnings] = useState(earningsDefault)
  const [links, setLinks] = useState(linksDefault)
  const [payouts, setPayouts] = useState(payoutsDefault)

  useEffect(() => {
    trackEvent('page_view', { page: 'affiliate' })

    const load = async () => {
      try {
        const [dashboard, earningsRes, linksRes, payoutsRes] = await Promise.all([
          api.get<Record<string, unknown>>('/api/v1/affiliate/dashboard'),
          api.get<{ earnings: Array<Record<string, unknown>> }>('/api/v1/affiliate/earnings'),
          api.get<{ links: Array<Record<string, unknown>> }>('/api/v1/affiliate/links'),
          api.get<{ data: Array<Record<string, unknown>> }>('/api/v1/affiliate/payouts'),
        ])

        const d = dashboard || {}
        setSummaryCards([
          { label: 'Сегодня', value: `$${d.earnings_today ?? '0.00'}`, hint: 'Начислено по финализированному NGR' },
          { label: 'За месяц', value: `$${d.earnings_this_month ?? '0.00'}`, hint: 'После бонусов, fees, chargebacks и taxes' },
          { label: 'В ожидании', value: `$${d.pending_amount ?? '0.00'}`, hint: 'Удержание до конца hold period' },
          { label: 'Доступно', value: `$${d.available_amount ?? '0.00'}`, hint: 'Можно отправить payout request' },
        ])
        setFunnel([
          { label: 'Клики', value: String(d.clicks ?? '0') },
          { label: 'Регистрации', value: String(d.registrations ?? '0') },
          { label: 'FTD', value: String(d.ftd_count ?? '0') },
          { label: 'Активные игроки', value: String(d.active_players ?? '0') },
        ])

        const earningItems = (earningsRes?.earnings ?? []).slice(0, 5).map((item) => ({
          period: String(item.period_end ?? item.created_at ?? ''),
          ggr: `$${item.ggr_amount ?? '0.00'}`,
          ngr: `$${item.ngr_amount ?? '0.00'}`,
          commission: `$${item.commission_amount ?? '0.00'}`,
          status: String(item.status ?? 'pending'),
        }))
        if (earningItems.length > 0) {
          setEarnings(earningItems)
        }

        const linkItems = (linksRes?.links ?? []).slice(0, 5).map((item) => ({
          name: String(item.campaign_name ?? 'Campaign'),
          code: String(item.referral_code ?? ''),
          url: String(item.referral_url ?? ''),
          utm: `utm_source=${item.utm_source ?? ''}&utm_medium=${item.utm_medium ?? ''}&utm_campaign=${item.utm_campaign ?? ''}`,
        }))
        if (linkItems.length > 0) {
          setLinks(linkItems)
        }

        const payoutItems = (payoutsRes?.data ?? []).slice(0, 5).map((item) => ({
          id: String(item.id ?? ''),
          amount: `$${item.amount ?? '0.00'}`,
          status: String(item.status ?? 'requested'),
          date: String(item.requested_at ?? item.created_at ?? ''),
          method: String(item.method_id ?? '—'),
        }))
        if (payoutItems.length > 0) {
          setPayouts(payoutItems)
        }
      } catch {
        // Keep static fallback cards if API is not available yet.
      }
    }

    load()
  }, [])

  return (
    <div className="section max-w-5xl">
      <div className="flex flex-col gap-2 mb-5">
        <h1 className="text-sm font-bold text-white">Партнерский кабинет</h1>
        <div className="card p-3">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-[10px] uppercase tracking-wide text-gray-500">RevShare From NGR</p>
              <p className="text-xs text-gray-300 mt-1">
                Комиссия считается только по финализированному `NGR`, отрицательные периоды на MVP обрезаются до `0`.
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <span className="badge badge-green">Affiliate Active</span>
              <span className="badge badge-yellow">Hold 14 Days</span>
              <span className="badge badge-blue">Rate 20%</span>
            </div>
          </div>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4 mb-5">
        {summaryCards.map((card) => (
          <div key={card.label} className="card p-3">
            <p className="text-[10px] uppercase tracking-wide text-gray-500">{card.label}</p>
            <p className="text-lg font-semibold text-white mt-2">{card.value}</p>
            <p className="text-[10px] text-gray-600 mt-1">{card.hint}</p>
          </div>
        ))}
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr]">
        <div className="space-y-4">
          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">Funnel</h2>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              {funnel.map((item) => (
                <div key={item.label} className="rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3">
                  <p className="text-[10px] text-gray-500">{item.label}</p>
                  <p className="mt-1 text-sm font-semibold text-white">{item.value}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="card p-3">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-xs font-semibold text-white">Начисления</h2>
              <button className="btn-outline">Запросить выплату</button>
            </div>
            <div className="space-y-2">
              {earnings.map((item) => (
                <div
                  key={item.period}
                  className="grid gap-2 rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3 text-[11px] text-gray-300 sm:grid-cols-4"
                >
                  <div>
                    <p className="text-[10px] text-gray-500">Период</p>
                    <p className="mt-1 text-white">{item.period}</p>
                  </div>
                  <div>
                    <p className="text-[10px] text-gray-500">GGR / NGR</p>
                    <p className="mt-1">{item.ggr} / {item.ngr}</p>
                  </div>
                  <div>
                    <p className="text-[10px] text-gray-500">Комиссия</p>
                    <p className="mt-1 text-white">{item.commission}</p>
                  </div>
                  <div>
                    <p className="text-[10px] text-gray-500">Статус</p>
                    <p className="mt-1">
                      {item.status === 'available' && <span className="badge badge-green">available</span>}
                      {item.status === 'pending' && <span className="badge badge-yellow">pending</span>}
                      {item.status === 'released_zero_floor' && <span className="badge badge-blue">zero floor</span>}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">Referral Links</h2>
            <div className="space-y-2">
              {links.map((link) => (
                <div key={link.code} className="rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3">
                  <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <p className="text-xs text-white">{link.name}</p>
                      <p className="text-[10px] text-gray-500 mt-0.5">{link.code}</p>
                    </div>
                    <button className="btn-outline">Скопировать</button>
                  </div>
                  <p className="mt-2 break-all text-[11px] text-blue-400">{link.url}</p>
                  <p className="mt-1 break-all text-[10px] text-gray-600">{link.utm}</p>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="space-y-4">
          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">Payout Settings</h2>
            <div className="space-y-2 text-[11px] text-gray-300">
              <div className="flex items-center justify-between border-b border-[rgb(var(--border))] pb-2">
                <span className="text-gray-500">Метод</span>
                <span>USDT TRC20</span>
              </div>
              <div className="flex items-center justify-between border-b border-[rgb(var(--border))] pb-2">
                <span className="text-gray-500">Min payout</span>
                <span>$100.00</span>
              </div>
              <div className="flex items-center justify-between border-b border-[rgb(var(--border))] pb-2">
                <span className="text-gray-500">Schedule</span>
                <span>Monthly</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-500">KYC</span>
                <span className="badge badge-green">verified</span>
              </div>
            </div>
          </div>

          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">История выплат</h2>
            <div className="space-y-2">
              {payouts.map((payout) => (
                <div key={payout.id} className="rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3">
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-white">{payout.amount}</p>
                    <span className={payout.status === 'paid' ? 'badge badge-green' : 'badge badge-yellow'}>
                      {payout.status}
                    </span>
                  </div>
                  <p className="mt-1 text-[10px] text-gray-500">{payout.method}</p>
                  <p className="mt-1 text-[10px] text-gray-600">{payout.date}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">Risk Notes</h2>
            <ul className="space-y-2 text-[11px] text-gray-300">
              <li>Self-referral и duplicate payment instruments блокируют payout.</li>
              <li>Комиссия становится `available` только после hold period.</li>
              <li>Affiliate earnings не смешиваются с игровым балансом.</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  )
}
