import { Card, createTheme, NavLink, Paper } from '@mantine/core'

/** App theme: a calm primary colour, soft corners and bordered surfaces. */
export const theme = createTheme({
  primaryColor: 'teal',
  primaryShade: { light: 7, dark: 5 },
  defaultRadius: 'md',
  fontFamily: 'system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
  headings: { fontWeight: '650' },
  components: {
    Card: Card.extend({ defaultProps: { withBorder: true, radius: 'md', padding: 'lg' } }),
    Paper: Paper.extend({ defaultProps: { radius: 'md' } }),
    NavLink: NavLink.extend({ defaultProps: { variant: 'light' }, styles: { root: { borderRadius: 'var(--mantine-radius-md)' } } }),
  },
})
