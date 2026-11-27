import { Container, Group, Title } from '@mantine/core'
import type { ReactNode } from 'react'
import { LanguageSwitcher } from './LanguageSwitcher'

/** Simple page frame for the public pages: title on the left, language and actions on the right. */
export function PageShell({ title, actions, children }: { title: string; actions?: ReactNode; children?: ReactNode }) {
  return (
    <Container size="md" py="xl">
      <Group justify="space-between" mb="lg" align="flex-start">
        <Title order={1}>{title}</Title>
        <Group gap="sm">
          {actions}
          <LanguageSwitcher />
        </Group>
      </Group>
      {children}
    </Container>
  )
}
