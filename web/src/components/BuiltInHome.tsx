import { Box, Center, Container, Loader, Stack, Text, Title, useComputedColorScheme } from '@mantine/core'
import type { PublicItem } from '../api/publicData'
import type { HomeItem } from '../api/types'
import { HomeCards } from './HomeCards'
import { Quarter, type QuarterHouse } from './Quarter'

const HERO_LIGHT = 'linear-gradient(180deg, #eef7f1 0%, #e4f0f6 70%, var(--mantine-color-body) 100%)'
const HERO_DARK = 'linear-gradient(180deg, #111a22 0%, #16222d 70%, var(--mantine-color-body) 100%)'

/**
 * The body of the built-in home page under our header: the community name, the welcome text,
 * the quarter drawn from the houses (undefined while they load), and the chosen data cards.
 */
export function BuiltInHome({ name, welcome, houses, homeItems, data }: {
  name: string
  welcome: string
  houses: QuarterHouse[] | undefined
  homeItems: HomeItem[]
  data: Record<string, PublicItem> | undefined
}) {
  const dark = useComputedColorScheme('light') === 'dark'
  return (
    <>
      <Box component="section" pt={48} pb={40} style={{ background: dark ? HERO_DARK : HERO_LIGHT }}>
        <Container size="lg">
          <Stack gap="md" align="center" ta="center">
            <Title order={1} maw={920} style={{ fontSize: 'clamp(1.75rem, 1.1rem + 2.6vw, 2.9rem)', lineHeight: 1.15, letterSpacing: '-0.02em', textWrap: 'balance' }}>
              {name}
            </Title>
            <Stack gap={6} maw={640}>
              {welcome.split(/\n\s*\n/).map((p, i) => (
                <Text key={i} size="lg" c="dimmed" style={{ whiteSpace: 'pre-line', textWrap: 'pretty' }}>
                  {p}
                </Text>
              ))}
            </Stack>
          </Stack>
          <Box mt={36}>
            {houses ? (
              <Quarter houses={houses} />
            ) : (
              <Center h={240}>
                <Loader />
              </Center>
            )}
          </Box>
        </Container>
      </Box>
      {data && homeItems.length > 0 && (
        <Container size="lg" pb={64} pt="md">
          <HomeCards items={homeItems} data={data} />
        </Container>
      )}
    </>
  )
}
