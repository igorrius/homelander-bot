import { Button, Group, Modal, Text, type ButtonProps } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

/** A button that asks for confirmation in a modal before running onConfirm. */
export function ConfirmButton({ children, message, onConfirm, title, ...props }: ButtonProps & { children: ReactNode; message: string; onConfirm(): void; /** Dialog title when the button shows only an icon. */ title?: string }) {
  const { t } = useTranslation()
  const [opened, { open, close }] = useDisclosure(false)
  // The dialog stays clickable while it fades out; confirm at most once per opening.
  const confirmed = useRef(false)
  return (
    <>
      <Button
        {...props}
        onClick={() => {
          confirmed.current = false
          open()
        }}
      >
        {children}
      </Button>
      <Modal opened={opened} onClose={close} title={title ?? children} centered>
        <Text mb="lg">{message}</Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={close}>
            {t('common.cancel')}
          </Button>
          <Button
            color={props.color}
            onClick={() => {
              if (confirmed.current) return
              confirmed.current = true
              close()
              onConfirm()
            }}
          >
            {t('common.confirm')}
          </Button>
        </Group>
      </Modal>
    </>
  )
}
