import type { TFunction } from 'i18next'
import type { Chat, Label, Topic } from '../api/types'

/** "TG · forum" and the like. */
export function chatKind(t: TFunction, c: Chat): string {
  const kind = c.kind === 'channel' ? 'channel' : c.isForum ? 'forum' : 'group'
  return `${t(`admin.chats.messenger.${c.messenger}`, { defaultValue: c.messenger })} · ${t(`admin.chats.kind.${kind}`)}`
}

export function topicTitle(t: TFunction, tp: Topic): string {
  return tp.title || t('admin.chats.unnamedTopic', { id: tp.externalId })
}

/** Whether a label hangs on at least one place that receives messages. */
export function reachesChat(l: Label, chats: Chat[]): boolean {
  return l.targets.some((tg) =>
    chats.some((c) => c.state === 'accepted' && (tg.type === 'chat' ? c.id === tg.id : c.topics.some((tp) => tp.id === tg.id && !tp.gone))),
  )
}

/** Readable names of the places a label hangs on ("Our yard › News"). */
export function labelPlaces(t: TFunction, l: Label, chats: Chat[]): string[] {
  return l.targets.map((tg) => {
    for (const c of chats) {
      if (tg.type === 'chat' && c.id === tg.id) return c.title
      const tp = c.topics.find((x) => x.id === tg.id)
      if (tg.type === 'chat_topic' && tp) return `${c.title} › ${topicTitle(t, tp)}`
    }
    return `#${tg.id}`
  })
}
