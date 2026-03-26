import { describe, it, expect } from 'vitest'
import type { PayoutEntry, BonusType } from '../types/league'

// --- helpers mirrored from LeagueSettings.tsx ---
// These are pure functions; we test the logic independently of React.

const BONUS_TYPE_OPTIONS: { value: BonusType; label: string }[] = [
  { value: 'weekly_high_score', label: 'Weekly High Score' },
  { value: 'score_threshold',   label: 'Score Threshold' },
]

function defaultLabelForBonusType(bt: BonusType): string {
  return BONUS_TYPE_OPTIONS.find(o => o.value === bt)?.label ?? bt
}

function normalizeEntries(entries: PayoutEntry[]): PayoutEntry[] {
  return entries.map((e, i) => ({
    type: e.type ?? 'placement',
    bonus_type: e.type === 'weekly' ? (e.bonus_type ?? 'weekly_high_score') : undefined,
    label: e.label || (e.type === 'weekly'
      ? defaultLabelForBonusType(e.bonus_type ?? 'weekly_high_score')
      : `${i + 1}th Place`),
    place: e.place,
    amount_cents: e.amount_cents,
    weeks: e.weeks,
    criteria: e.criteria,
  }))
}

// ------------------------------------------------

describe('defaultLabelForBonusType', () => {
  it('returns readable label for weekly_high_score', () => {
    expect(defaultLabelForBonusType('weekly_high_score')).toBe('Weekly High Score')
  })

  it('returns readable label for score_threshold', () => {
    expect(defaultLabelForBonusType('score_threshold')).toBe('Score Threshold')
  })
})

describe('normalizeEntries', () => {
  it('defaults bonus_type to weekly_high_score for old weekly entries without bonus_type', () => {
    const old: PayoutEntry[] = [
      { type: 'weekly', label: 'Weekly High Score', amount_cents: 1000, weeks: 14 },
    ]
    const result = normalizeEntries(old)
    expect(result[0].bonus_type).toBe('weekly_high_score')
  })

  it('preserves existing bonus_type on weekly entries', () => {
    const entries: PayoutEntry[] = [
      { type: 'weekly', bonus_type: 'score_threshold', label: 'Score Threshold', amount_cents: 500, weeks: 14, criteria: { threshold: 150 } },
    ]
    const result = normalizeEntries(entries)
    expect(result[0].bonus_type).toBe('score_threshold')
    expect(result[0].criteria?.threshold).toBe(150)
  })

  it('does not set bonus_type on placement entries', () => {
    const entries: PayoutEntry[] = [
      { type: 'placement', label: '1st Place', place: 1, amount_cents: 5000 },
    ]
    const result = normalizeEntries(entries)
    expect(result[0].bonus_type).toBeUndefined()
  })

  it('generates label from bonus_type when label is empty', () => {
    const entries: PayoutEntry[] = [
      { type: 'weekly', label: '', amount_cents: 1000, weeks: 14 },
    ]
    const result = normalizeEntries(entries)
    expect(result[0].label).toBe('Weekly High Score')
  })

  it('generates Score Threshold label when bonus_type is score_threshold and label is empty', () => {
    const entries: PayoutEntry[] = [
      { type: 'weekly', bonus_type: 'score_threshold', label: '', amount_cents: 500, weeks: 14 },
    ]
    const result = normalizeEntries(entries)
    expect(result[0].label).toBe('Score Threshold')
  })

  it('preserves criteria through normalization', () => {
    const entries: PayoutEntry[] = [
      { type: 'weekly', bonus_type: 'score_threshold', label: 'Score Threshold', amount_cents: 500, weeks: 10, criteria: { threshold: 200 } },
    ]
    const result = normalizeEntries(entries)
    expect(result[0].criteria?.threshold).toBe(200)
  })

  it('passes through placement entries unchanged (no criteria, no bonus_type)', () => {
    const entries: PayoutEntry[] = [
      { type: 'placement', label: '1st Place', place: 1, amount_cents: 10000 },
      { type: 'placement', label: '2nd Place', place: 2, amount_cents: 5000 },
    ]
    const result = normalizeEntries(entries)
    expect(result).toHaveLength(2)
    result.forEach(r => {
      expect(r.bonus_type).toBeUndefined()
      expect(r.criteria).toBeUndefined()
    })
  })
})
