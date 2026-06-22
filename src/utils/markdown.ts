import { marked } from 'marked'

/**
 * Markdown 转 HTML
 */
export const markdownToHtml = (markdown: string): string => {
  return marked(markdown) as string
}
