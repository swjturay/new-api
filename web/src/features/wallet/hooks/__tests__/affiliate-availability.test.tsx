/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { useAffiliate } from '../use-affiliate'

const walletApi = vi.hoisted(() => ({
  getAffiliateCode: vi.fn(),
  transferAffiliateQuota: vi.fn(),
}))

vi.mock('../../api', () => walletApi)
vi.mock('@/lib/api', () => ({ getSelf: vi.fn() }))
vi.mock('@/hooks/use-copy-to-clipboard', () => ({
  useCopyToClipboard: () => ({ copyToClipboard: vi.fn() }),
}))

describe('affiliate feature availability', () => {
  beforeEach(() => {
    walletApi.getAffiliateCode.mockResolvedValue({
      success: true,
      data: 'saved-code',
    })
  })

  test('does not query or transfer rewards while the program is disabled', async () => {
    const { result } = renderHook(() => useAffiliate(false))

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(walletApi.getAffiliateCode).not.toHaveBeenCalled()

    let transferred = true
    await act(async () => {
      transferred = await result.current.transferQuota(100)
    })

    expect(transferred).toBe(false)
    expect(walletApi.transferAffiliateQuota).not.toHaveBeenCalled()
  })

  test('loads the referral code when the program is enabled', async () => {
    const { result } = renderHook(() => useAffiliate(true))

    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(walletApi.getAffiliateCode).toHaveBeenCalledTimes(1)
    expect(result.current.affiliateCode).toBe('saved-code')
  })
})
