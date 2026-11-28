/** Whether src is one of our site pages (published or a draft preview), the only frames that get viewer data. */
export function isOurPageUrl(src: string): boolean {
  return /^\/site\/(?!\/)/.test(src) && !src.split(/[?#]/)[0].split('/').includes('..')
}
