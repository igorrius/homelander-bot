import { Container, Group, Title } from '@mantine/core'
import type { ReactNode } from 'react'
import { LanguageSwitcher } from './LanguageSwitcher'

/** Simple page frame for the public pages: title on the left, language on the right. */
export function PageShell({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <Container size="md" py="xl">
      <Group justify="space-between" mb="lg">
        <Title order={1}>{title}</Title>
        <LanguageSwitcher />
      </Group>
      {children}
    </Container>
  )
}
