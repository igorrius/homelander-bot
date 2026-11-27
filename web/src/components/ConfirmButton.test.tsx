import { MantineProvider } from '@mantine/core'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { i18n } from '../i18n'
import uk from '../locales/uk.json'
import { ConfirmButton } from './ConfirmButton'

describe('ConfirmButton', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('uk')
  })

  it('confirms once on a double click while the dialog is closing', async () => {
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    // Real transitions (no env="test"): the dialog stays clickable while it fades out.
    render(
      <MantineProvider>
        <ConfirmButton message="Sure?" onConfirm={onConfirm}>
          Remove
        </ConfirmButton>
      </MantineProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'Remove' }))
    await user.dblClick(await screen.findByRole('button', { name: uk.common.confirm }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('can confirm again after reopening', async () => {
    const onConfirm = vi.fn()
    const user = userEvent.setup()
    render(
      <MantineProvider env="test">
        <ConfirmButton message="Sure?" onConfirm={onConfirm}>
          Remove
        </ConfirmButton>
      </MantineProvider>,
    )
    for (let i = 0; i < 2; i++) {
      await user.click(screen.getByRole('button', { name: 'Remove' }))
      await user.click(await screen.findByRole('button', { name: uk.common.confirm }))
    }
    expect(onConfirm).toHaveBeenCalledTimes(2)
  })
})
