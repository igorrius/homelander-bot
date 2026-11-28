import { Anchor, Text, type TextProps } from '@mantine/core'
import { splitLinks } from '../lib/links'

/** Plain text with its line breaks kept and its http(s) URLs as links that open in a new tab. */
export function LinkifiedText({ text, ...props }: TextProps & { text: string }) {
  return (
    <Text {...props} style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
      {splitLinks(text).map((p, i) =>
        p.href ? (
          <Anchor key={i} href={p.href} target="_blank" rel="noopener noreferrer" inherit>
            {p.text}
          </Anchor>
        ) : (
          p.text
        ),
      )}
    </Text>
  )
}
