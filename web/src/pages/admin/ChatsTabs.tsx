import { Tabs } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'

/** Switches between the chats list and the labels list. */
export function ChatsTabs() {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const value = pathname.startsWith('/admin/chats/labels') ? 'labels' : 'chats'
  return (
    <Tabs value={value} onChange={(v) => void navigate(v === 'labels' ? '/admin/chats/labels' : '/admin/chats')}>
      <Tabs.List>
        <Tabs.Tab value="chats">{t('admin.chats.tabs.chats')}</Tabs.Tab>
        <Tabs.Tab value="labels">{t('admin.chats.tabs.labels')}</Tabs.Tab>
      </Tabs.List>
    </Tabs>
  )
}
