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
import { renderHook } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { useUsersColumns } from '../users-columns'

const featureStatus = vi.hoisted(() => ({ referralProgramEnabled: false }))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: {
      referral_program_enabled: featureStatus.referralProgramEnabled,
    },
  }),
}))

function hasInviteInfoColumn(
  columns: ReturnType<typeof useUsersColumns>
): boolean {
  return columns.some((column) => column.id === 'invite_info')
}

describe('user-list referral columns', () => {
  test('omits invite information while the referral program is disabled', () => {
    featureStatus.referralProgramEnabled = false

    const { result } = renderHook(() => useUsersColumns())

    expect(hasInviteInfoColumn(result.current)).toBe(false)
  })

  test('restores invite information when the referral program is enabled', () => {
    featureStatus.referralProgramEnabled = true

    const { result } = renderHook(() => useUsersColumns())

    expect(hasInviteInfoColumn(result.current)).toBe(true)
  })
})
