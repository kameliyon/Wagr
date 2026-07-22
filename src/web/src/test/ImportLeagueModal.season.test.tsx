import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ImportLeagueModal from '../components/ImportLeagueModal'

vi.mock('../hooks/useWallet', () => ({
  useWallet: () => ({ token: 'test-token' }),
}))

const CURRENT_YEAR = new Date().getFullYear()

function mockFetchSequence() {
  global.fetch = vi
    .fn()
    .mockResolvedValueOnce({
      ok: true,
      json: async () => ({ platform_user_id: 'user-1', platform_username: 'testuser' }),
    })
    .mockResolvedValueOnce({
      ok: true,
      json: async () => [],
    })
}

describe('ImportLeagueModal — season selection', () => {
  beforeEach(() => {
    mockFetchSequence()
  })

  it('defaults the season dropdown to the current year and offers the current + prior 2 years', () => {
    render(
      <ImportLeagueModal isOpen={true} onClose={() => {}} onImported={() => {}} existingLeagues={[]} />
    )

    const select = screen.getByLabelText('Season') as HTMLSelectElement
    expect(select.value).toBe(String(CURRENT_YEAR))

    const optionValues = Array.from(select.options).map((o) => o.value)
    expect(optionValues).toEqual([
      String(CURRENT_YEAR),
      String(CURRENT_YEAR - 1),
      String(CURRENT_YEAR - 2),
    ])
  })

  it('fetches leagues using the selected season and reflects it in step 2 copy', async () => {
    render(
      <ImportLeagueModal isOpen={true} onClose={() => {}} onImported={() => {}} existingLeagues={[]} />
    )

    fireEvent.change(screen.getByLabelText('Season'), { target: { value: String(CURRENT_YEAR - 1) } })
    fireEvent.change(screen.getByLabelText('Sleeper Username'), { target: { value: 'testuser' } })
    fireEvent.click(screen.getByRole('button', { name: 'Connect' }))

    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(2))

    const leaguesCallUrl = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[1][0] as string
    expect(leaguesCallUrl).toContain(`season=${CURRENT_YEAR - 1}`)

    await waitFor(() => {
      expect(screen.getByText(new RegExp(`${CURRENT_YEAR - 1} NFL season`))).toBeInTheDocument()
    })
  })
})
