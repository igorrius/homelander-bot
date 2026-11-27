import { screen } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'

/** Opens the Mantine Select labelled label and picks the option named option. */
export async function choose(user: UserEvent, label: string | RegExp, option: string | RegExp) {
  await user.click(await screen.findByRole('combobox', { name: label }))
  await user.click(await screen.findByRole('option', { name: option }))
}
