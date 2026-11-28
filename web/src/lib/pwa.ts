import { useState, useSyncExternalStore } from 'react'

/** Chromium's install offer; not in the DOM typings. */
interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>
}

const dismissedKey = 'homelander.installDismissed'

// The offer can come before the cabinet is open (on the site, say), so it is
// caught as soon as the app loads and kept until used.
let offer: BeforeInstallPromptEvent | null = null
const subscribers = new Set<() => void>()

function subscribe(f: () => void) {
  subscribers.add(f)
  return () => {
    subscribers.delete(f)
  }
}

function setOffer(e: BeforeInstallPromptEvent | null) {
  offer = e
  subscribers.forEach((f) => f())
}

if (typeof window !== 'undefined') {
  window.addEventListener('beforeinstallprompt', (e) => {
    // Keep the browser's own banner away: the cabinet shows its own card.
    e.preventDefault()
    setOffer(e as BeforeInstallPromptEvent)
  })
  window.addEventListener('appinstalled', () => setOffer(null))
}

/** Registers the service worker; only in production builds, where /sw.js exists. */
export function registerServiceWorker() {
  if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch(() => {
      // No offline page then; the app works as before.
    })
  })
}

function isStandalone(): boolean {
  return window.matchMedia('(display-mode: standalone)').matches || (navigator as { standalone?: boolean }).standalone === true
}

/** iPhone, iPod or iPad (which says "Macintosh" but has a touch screen). */
function isIOS(): boolean {
  const ua = navigator.userAgent
  return /iPhone|iPad|iPod/.test(ua) || (ua.includes('Macintosh') && navigator.maxTouchPoints > 1)
}

function readDismissed(): boolean {
  try {
    return localStorage.getItem(dismissedKey) === '1'
  } catch {
    return false
  }
}

/** How the app can be installed here: the browser's prompt, the iOS share sheet, or not at all (null). */
export type InstallMode = 'prompt' | 'ios' | null

export function useInstall(): { mode: InstallMode; install(): Promise<void>; dismiss(): void } {
  const current = useSyncExternalStore(subscribe, () => offer)
  const [dismissed, setDismissed] = useState(readDismissed)
  let mode: InstallMode = null
  if (!dismissed && !isStandalone()) mode = current ? 'prompt' : isIOS() ? 'ios' : null
  return {
    mode,
    async install() {
      const e = offer
      if (!e) return
      setOffer(null) // a prompt can be shown only once
      await e.prompt()
    },
    dismiss() {
      try {
        localStorage.setItem(dismissedKey, '1')
      } catch {
        // Private mode: hidden until the next visit.
      }
      setDismissed(true)
    },
  }
}
