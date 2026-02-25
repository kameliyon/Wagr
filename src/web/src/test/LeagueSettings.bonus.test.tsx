import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import LeagueSettingsPage from '../pages/LeagueSettings'
import type { LeagueSettings } from '../types/league'

// Mock useWallet so we don't need a real wallet provider
vi.mock('../hooks/useWallet', () => ({
  useWallet: () => ({ isAuthenticated: true, token: 'test-token' }),
}))

const commissionerSettings: LeagueSettings = {
  entry_fee_cents: 5000,
  total_rosters: 10,
  is_commissioner: true,
  payout_structure: [
    {
      type: 'weekly',
      bonus_type: 'weekly_high_score',
      label: 'Weekly High Score',
      amount_cents: 1000,
      weeks: 14,
    },
  ],
}

const nonCommissionerSettings: LeagueSettings = {
  entry_fee_cents: 5000,
  total_rosters: 10,
  is_commissioner: false,
  payout_structure: [
    {
      type: 'weekly',
      bonus_type: 'score_threshold',
      label: 'Score Threshold',
      amount_cents: 500,
      weeks: 14,
      criteria: { threshold: 150 },
    },
  ],
}

function renderSettings(settings: LeagueSettings) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => settings,
    text: async () => '',
  })

  return render(
    <MemoryRouter initialEntries={['/leagues/test-league-id/settings']}>
      <Routes>
        <Route path="/leagues/:leagueId/settings" element={<LeagueSettingsPage />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('LeagueSettings — bonus type dropdown (commissioner)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders bonus-type dropdown instead of text input for weekly entries', async () => {
    renderSettings(commissionerSettings)

    const dropdown = await screen.findByRole('combobox')
    expect(dropdown).toBeInTheDocument()
    expect((dropdown as HTMLSelectElement).value).toBe('weekly_high_score')
  })

  it('shows both bonus type options in the dropdown', async () => {
    renderSettings(commissionerSettings)

    await screen.findByRole('combobox')
    expect(screen.getByRole('option', { name: 'Weekly High Score' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Score Threshold' })).toBeInTheDocument()
  })

  it('does not show Min pts input when bonus type is weekly_high_score', async () => {
    renderSettings(commissionerSettings)

    await screen.findByRole('combobox')
    expect(screen.queryByPlaceholderText('Min pts')).not.toBeInTheDocument()
  })

  it('shows Min pts input when score_threshold is selected', async () => {
    renderSettings(commissionerSettings)

    const dropdown = await screen.findByRole('combobox')
    fireEvent.change(dropdown, { target: { value: 'score_threshold' } })

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Min pts')).toBeInTheDocument()
    })
  })

  it('hides Min pts input when switching back to weekly_high_score', async () => {
    renderSettings(commissionerSettings)

    const dropdown = await screen.findByRole('combobox')
    fireEvent.change(dropdown, { target: { value: 'score_threshold' } })
    await screen.findByPlaceholderText('Min pts')

    fireEvent.change(dropdown, { target: { value: 'weekly_high_score' } })
    await waitFor(() => {
      expect(screen.queryByPlaceholderText('Min pts')).not.toBeInTheDocument()
    })
  })
})

describe('LeagueSettings — bonus type read-only view (non-commissioner)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows threshold annotation in read-only view for score_threshold entries', async () => {
    renderSettings(nonCommissionerSettings)

    await waitFor(() => {
      expect(screen.getByText(/≥ 150 pts/)).toBeInTheDocument()
    })
  })

  it('does not render a dropdown in read-only view', async () => {
    renderSettings(nonCommissionerSettings)

    await waitFor(() => {
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
    })
  })
})
