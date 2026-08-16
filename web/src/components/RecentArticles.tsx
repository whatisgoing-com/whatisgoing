import type { RecentArticle } from '../api/types'
import { formatPublishedAt } from '../lib/formatDate'

export function RecentArticles({ articles }: { articles: RecentArticle[] }) {
  if (articles.length === 0) {
    return <p className="text-sm text-gray-500">No articles yet.</p>
  }

  return (
    <ul className="divide-y divide-gray-100">
      {articles.map((a) => (
        <li key={a.id} className="py-2.5">
          <a href={a.url} target="_blank" rel="noopener" className="font-medium text-gray-900 hover:text-blue-600">
            {a.title}
          </a>
          <div className="text-xs text-gray-500">
            {a.source_name} &middot; {formatPublishedAt(a.published_at)}
          </div>
        </li>
      ))}
    </ul>
  )
}
