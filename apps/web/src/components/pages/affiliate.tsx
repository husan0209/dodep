"use client";

import { useCallback, useEffect, useState } from "react";
import { trackEvent } from "@lib/telemetry";
import { ApiClientError } from "@lib/api/client";
import {
  affiliateApi,
  type AffiliateDashboard,
  type AffiliateEarning,
  type AffiliateLink,
  type AffiliatePayout,
  type AffiliateProfile,
  type PayoutMethod,
} from "@lib/api/affiliate";

const summaryCardsDefault = [
  {
    label: "Сегодня",
    value: "$124.80",
    hint: "Начислено по финализированному NGR",
  },
  {
    label: "За месяц",
    value: "$2,481.50",
    hint: "После бонусов, fees, chargebacks и taxes",
  },
  {
    label: "В ожидании",
    value: "$612.20",
    hint: "Удержание до конца hold period",
  },
  {
    label: "Доступно",
    value: "$438.00",
    hint: "Можно отправить payout request",
  },
];

const funnelDefault = [
  { label: "Клики", value: "12 481" },
  { label: "Регистрации", value: "684" },
  { label: "FTD", value: "179" },
  { label: "Активные игроки", value: "96" },
];

const earningsDefault = [
  {
    period: "2026-04-15",
    ggr: "$310.20",
    ngr: "$198.40",
    commission: "$39.68",
    status: "available",
  },
  {
    period: "2026-04-14",
    ggr: "$280.00",
    ngr: "$140.10",
    commission: "$28.02",
    status: "pending",
  },
  {
    period: "2026-04-13",
    ggr: "$154.90",
    ngr: "$0.00",
    commission: "$0.00",
    status: "released_zero_floor",
  },
];

const linksDefault = [
  {
    name: "Main Campaign",
    code: "AFF-SPORTS-01",
    url: "https://dod.example/r/AFF-SPORTS-01/main",
    utm: "utm_source=telegram&utm_medium=influencer&utm_campaign=main",
  },
  {
    name: "Casino Stream",
    code: "AFF-CASINO-07",
    url: "https://dod.example/r/AFF-CASINO-07/stream",
    utm: "utm_source=youtube&utm_medium=stream&utm_campaign=casino_stream",
  },
];

const payoutsDefault = [
  {
    id: "pay_001",
    amount: "$250.00",
    status: "paid",
    date: "2026-04-01",
    method: "USDT TRC20",
  },
  {
    id: "pay_002",
    amount: "$180.00",
    status: "reviewing",
    date: "2026-04-15",
    method: "Bank transfer",
  },
];

function money(value: string | number | undefined, fallback = "$0.00"): string {
  if (value === undefined || value === null || value === "") return fallback;
  const s = String(value);
  return s.startsWith("$") ? s : `$${s}`;
}

function dashboardCards(d: AffiliateDashboard) {
  return [
    {
      label: "Сегодня",
      value: money(d.earnings_today),
      hint: "Начислено по финализированному NGR",
    },
    {
      label: "За месяц",
      value: money(d.earnings_this_month),
      hint: "После бонусов, fees, chargebacks и taxes",
    },
    {
      label: "В ожидании",
      value: money(d.pending_amount),
      hint: "Удержание до конца hold period",
    },
    {
      label: "Доступно",
      value: money(d.available_amount),
      hint: "Можно отправить payout request",
    },
  ];
}

function dashboardFunnel(d: AffiliateDashboard) {
  return [
    { label: "Клики", value: String(d.clicks ?? "0") },
    { label: "Регистрации", value: String(d.registrations ?? "0") },
    { label: "FTD", value: String(d.ftd_count ?? "0") },
    { label: "Активные игроки", value: String(d.active_players ?? "0") },
  ];
}

export function AffiliatePage() {
  const [summaryCards, setSummaryCards] = useState(summaryCardsDefault);
  const [funnel, setFunnel] = useState(funnelDefault);
  const [earnings, setEarnings] = useState(earningsDefault);
  const [links, setLinks] = useState(linksDefault);
  const [payouts, setPayouts] = useState(payoutsDefault);
  const [profile, setProfile] = useState<AffiliateProfile | null>(null);
  const [methods, setMethods] = useState<PayoutMethod[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [notEnrolled, setNotEnrolled] = useState(false);
  const [enrolling, setEnrolling] = useState(false);
  const [copiedCode, setCopiedCode] = useState<string | null>(null);
  const [payoutAmount, setPayoutAmount] = useState("");
  const [payoutMethodId, setPayoutMethodId] = useState("");
  const [payoutBusy, setPayoutBusy] = useState(false);
  const [payoutError, setPayoutError] = useState<string | null>(null);
  const [payoutOk, setPayoutOk] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    setPayoutError(null);
    setPayoutOk(null);
    try {
      let prof: AffiliateProfile | null = null;
      try {
        prof = await affiliateApi.getProfile();
        setProfile(prof);
        setNotEnrolled(false);
      } catch (e) {
        if (e instanceof ApiClientError && e.isNotFound) {
          setProfile(null);
          setNotEnrolled(true);
        } else {
          throw e;
        }
      }

      const [dashboard, earningItems, linkItems, payoutItems, methodItems] =
        await Promise.all([
          affiliateApi.getDashboard().catch(() => null),
          affiliateApi
            .listEarnings({ limit: 5 })
            .catch(() => [] as AffiliateEarning[]),
          affiliateApi.listLinks().catch(() => [] as AffiliateLink[]),
          affiliateApi.listPayouts().catch(() => [] as AffiliatePayout[]),
          affiliateApi.listPayoutMethods().catch(() => [] as PayoutMethod[]),
        ]);

      if (dashboard) {
        setSummaryCards(dashboardCards(dashboard));
        setFunnel(dashboardFunnel(dashboard));
      }

      if (earningItems.length > 0) {
        setEarnings(
          earningItems.slice(0, 5).map((item) => ({
            period: String(
              item.period_end ?? item.period_start ?? item.created_at ?? "",
            ),
            ggr: money(item.ggr_amount),
            ngr: money(item.ngr_amount),
            commission: money(item.commission_amount),
            status: String(item.status ?? "pending"),
          })),
        );
      }

      if (linkItems.length > 0) {
        setLinks(
          linkItems.slice(0, 5).map((item) => ({
            name: String(item.campaign_name ?? "Campaign"),
            code: String(item.referral_code ?? ""),
            url: String(item.referral_url ?? ""),
            utm: `utm_source=${item.utm_source ?? ""}&utm_medium=${item.utm_medium ?? ""}&utm_campaign=${item.utm_campaign ?? ""}`,
          })),
        );
      }

      if (payoutItems.length > 0) {
        setPayouts(
          payoutItems.slice(0, 5).map((item) => ({
            id: String(item.id ?? ""),
            amount: money(item.amount),
            status: String(item.status ?? "requested"),
            date: String(item.requested_at ?? item.created_at ?? ""),
            method: String(item.method_id ?? "—"),
          })),
        );
      }

      setMethods(methodItems);
      const def = methodItems.find((m) => m.is_default) ?? methodItems[0];
      if (def?.id) setPayoutMethodId((prev) => prev || String(def.id));
    } catch {
      // Keep static fallback cards if API is not available yet.
      setLoadError(
        "Не удалось загрузить данные кабинета — показаны последние сохранённые значения.",
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    trackEvent("page_view", { page: "affiliate" });
    load();
  }, [load]);

  const handleEnroll = async () => {
    setEnrolling(true);
    try {
      await affiliateApi.enroll("Заявка из веб-кабинета партнёра");
      trackEvent("affiliate_enroll", {});
      await load();
    } catch {
      setLoadError("Не удалось отправить заявку на партнёрскую программу.");
    } finally {
      setEnrolling(false);
    }
  };

  const handleCopy = async (code: string, url: string) => {
    try {
      await navigator.clipboard.writeText(url);
      setCopiedCode(code);
      trackEvent("affiliate_link_copy", { code });
      setTimeout(
        () => setCopiedCode((prev) => (prev === code ? null : prev)),
        2000,
      );
    } catch {
      setCopiedCode(null);
    }
  };

  const handlePayout = async () => {
    setPayoutError(null);
    setPayoutOk(null);
    if (!payoutMethodId) {
      setPayoutError("Выберите payout method.");
      return;
    }
    if (!payoutAmount || Number(payoutAmount) <= 0) {
      setPayoutError("Введите корректную сумму больше нуля.");
      return;
    }
    setPayoutBusy(true);
    try {
      await affiliateApi.requestPayout({
        method_id: payoutMethodId,
        amount: payoutAmount,
      });
      trackEvent("affiliate_payout_request", { amount: payoutAmount });
      setPayoutOk("Заявка на выплату создана и отправлена на review.");
      setPayoutAmount("");
      const refreshed = await affiliateApi
        .listPayouts()
        .catch(() => [] as AffiliatePayout[]);
      if (refreshed.length > 0) {
        setPayouts(
          refreshed.slice(0, 5).map((item) => ({
            id: String(item.id ?? ""),
            amount: money(item.amount),
            status: String(item.status ?? "requested"),
            date: String(item.requested_at ?? item.created_at ?? ""),
            method: String(item.method_id ?? "—"),
          })),
        );
      }
    } catch (e) {
      const msg =
        e instanceof ApiClientError
          ? e.error.message
          : "Не удалось создать заявку на выплату.";
      setPayoutError(msg);
    } finally {
      setPayoutBusy(false);
    }
  };

  const statusBadge = profile?.status ? (
    <span className="badge badge-green">{profile.status}</span>
  ) : (
    <span className="badge badge-yellow">Not enrolled</span>
  );
  const holdDays = profile?.hold_period_days ?? 14;
  const rate = profile?.commission_rate
    ? `${Number(profile.commission_rate) * 100}%`
    : "20%";
  const defaultMethod =
    methods.find((m) => m.id === payoutMethodId) ??
    methods.find((m) => m.is_default) ??
    methods[0];

  return (
    <div className="section max-w-5xl">
      <div className="flex flex-col gap-2 mb-5">
        <h1 className="text-sm font-bold text-white">Партнерский кабинет</h1>
        <div className="card p-3">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-[10px] uppercase tracking-wide text-gray-500">
                RevShare From NGR
              </p>
              <p className="text-xs text-gray-300 mt-1">
                Комиссия считается только по финализированному `NGR`,
                отрицательные периоды на MVP обрезаются до `0`.
              </p>
              {loading && (
                <p className="text-[11px] text-gray-500 mt-2">
                  Загрузка данных кабинета…
                </p>
              )}
              {loadError && (
                <p className="text-[11px] text-yellow-400 mt-2">{loadError}</p>
              )}
            </div>
            <div className="flex flex-wrap gap-2">
              {statusBadge}
              <span className="badge badge-yellow">Hold {holdDays} Days</span>
              <span className="badge badge-blue">Rate {rate}</span>
            </div>
          </div>
          {notEnrolled && (
            <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between rounded border border-[rgb(var(--border))] p-3">
              <p className="text-[11px] text-gray-300">
                Вы ещё не партнёр. Подайте заявку — после manual approve
                появятся referral code и кабинет.
              </p>
              <button
                className="btn-outline"
                onClick={handleEnroll}
                disabled={enrolling}
              >
                {enrolling ? "Отправка…" : "Стать партнёром"}
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4 mb-5">
        {summaryCards.map((card) => (
          <div key={card.label} className="card p-3">
            <p className="text-[10px] uppercase tracking-wide text-gray-500">
              {card.label}
            </p>
            <p className="text-lg font-semibold text-white mt-2">
              {card.value}
            </p>
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
                <div
                  key={item.label}
                  className="rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3"
                >
                  <p className="text-[10px] text-gray-500">{item.label}</p>
                  <p className="mt-1 text-sm font-semibold text-white">
                    {item.value}
                  </p>
                </div>
              ))}
            </div>
          </div>

          <div className="card p-3">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-xs font-semibold text-white">Начисления</h2>
            </div>
            <div className="mb-3 rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3">
              <p className="text-[11px] text-gray-400 mb-2">
                Запросить выплату (min{" "}
                {money(profile?.min_payout_amount, "$100.00")})
              </p>
              <div className="flex flex-col gap-2 sm:flex-row">
                <select
                  className="input-field flex-1"
                  value={payoutMethodId}
                  onChange={(e) => setPayoutMethodId(e.target.value)}
                >
                  <option value="">Выберите метод…</option>
                  {methods.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.display_name ?? m.method_type}{" "}
                      {m.is_default ? "(default)" : ""}
                    </option>
                  ))}
                </select>
                <input
                  className="input-field w-32"
                  placeholder="Сумма"
                  inputMode="decimal"
                  value={payoutAmount}
                  onChange={(e) => setPayoutAmount(e.target.value)}
                />
                <button
                  className="btn-outline"
                  onClick={handlePayout}
                  disabled={payoutBusy}
                >
                  {payoutBusy ? "Отправка…" : "Запросить выплату"}
                </button>
              </div>
              {payoutError && (
                <p className="text-[11px] text-red-400 mt-2">{payoutError}</p>
              )}
              {payoutOk && (
                <p className="text-[11px] text-green-400 mt-2">{payoutOk}</p>
              )}
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
                    <p className="mt-1">
                      {item.ggr} / {item.ngr}
                    </p>
                  </div>
                  <div>
                    <p className="text-[10px] text-gray-500">Комиссия</p>
                    <p className="mt-1 text-white">{item.commission}</p>
                  </div>
                  <div>
                    <p className="text-[10px] text-gray-500">Статус</p>
                    <p className="mt-1">
                      {item.status === "available" && (
                        <span className="badge badge-green">available</span>
                      )}
                      {item.status === "pending" && (
                        <span className="badge badge-yellow">pending</span>
                      )}
                      {item.status === "released_zero_floor" && (
                        <span className="badge badge-blue">zero floor</span>
                      )}
                      {![
                        "available",
                        "pending",
                        "released_zero_floor",
                      ].includes(item.status) && (
                        <span className="badge">{item.status}</span>
                      )}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">
              Referral Links
            </h2>
            <div className="space-y-2">
              {links.map((link) => (
                <div
                  key={link.code}
                  className="rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3"
                >
                  <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <p className="text-xs text-white">{link.name}</p>
                      <p className="text-[10px] text-gray-500 mt-0.5">
                        {link.code}
                      </p>
                    </div>
                    <button
                      className="btn-outline"
                      onClick={() => handleCopy(link.code, link.url)}
                    >
                      {copiedCode === link.code ? "Скопировано" : "Скопировать"}
                    </button>
                  </div>
                  <p className="mt-2 break-all text-[11px] text-blue-400">
                    {link.url}
                  </p>
                  <p className="mt-1 break-all text-[10px] text-gray-600">
                    {link.utm}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="space-y-4">
          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">
              Payout Settings
            </h2>
            <div className="space-y-2 text-[11px] text-gray-300">
              <div className="flex items-center justify-between border-b border-[rgb(var(--border))] pb-2">
                <span className="text-gray-500">Метод</span>
                <span>
                  {defaultMethod?.display_name ??
                    defaultMethod?.method_type ??
                    "USDT TRC20"}
                </span>
              </div>
              <div className="flex items-center justify-between border-b border-[rgb(var(--border))] pb-2">
                <span className="text-gray-500">Min payout</span>
                <span>{money(profile?.min_payout_amount, "$100.00")}</span>
              </div>
              <div className="flex items-center justify-between border-b border-[rgb(var(--border))] pb-2">
                <span className="text-gray-500">Schedule</span>
                <span>{profile?.payout_schedule ?? "Monthly"}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-500">KYC</span>
                <span className="badge badge-green">verified</span>
              </div>
            </div>
          </div>

          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">
              История выплат
            </h2>
            <div className="space-y-2">
              {payouts.map((payout) => (
                <div
                  key={payout.id}
                  className="rounded border border-[rgb(var(--border))] bg-[rgb(var(--bg-primary))] p-3"
                >
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-white">{payout.amount}</p>
                    <span
                      className={
                        payout.status === "paid"
                          ? "badge badge-green"
                          : "badge badge-yellow"
                      }
                    >
                      {payout.status}
                    </span>
                  </div>
                  <p className="mt-1 text-[10px] text-gray-500">
                    {payout.method}
                  </p>
                  <p className="mt-1 text-[10px] text-gray-600">
                    {payout.date}
                  </p>
                </div>
              ))}
            </div>
          </div>

          <div className="card p-3">
            <h2 className="text-xs font-semibold text-white mb-3">
              Risk Notes
            </h2>
            <ul className="space-y-2 text-[11px] text-gray-300">
              <li>
                Self-referral и duplicate payment instruments блокируют payout.
              </li>
              <li>Комиссия становится `available` только после hold period.</li>
              <li>Affiliate earnings не смешиваются с игровым балансом.</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
