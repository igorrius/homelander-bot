import { Button, Group, Modal, Text, type ButtonProps } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

/** A button that asks for confirmation in a modal before running onConfirm. */
export function ConfirmButton({ children, message, onConfirm, ...props }: ButtonProps & { children: ReactNode; message: string; onConfirm(): void }) {
  const { t } = useTranslation()
  const [opened, { open, close }] = useDisclosure(false)
  return (
    <>
      <Button {...props} onClick={open}>
        {children}
      </Button>
      <Modal opened={opened} onClose={close} title={children} centered>
        <Text mb="lg">{message}</Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={close}>
            {t('common.cancel')}
          </Button>
          <Button
            color={props.color}
            onClick={() => {
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
