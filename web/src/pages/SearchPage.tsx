import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useArticleSearch } from '../api/hooks'
import { formatPublishedAt } from '../lib/formatDate'

export function SearchPage() {
  const [params, setParams] = useSearchParams()
  const query = params.get('q') ?? ''
  const [input, setInput] = useState(query)

  const { data: results, isFetched } = useArticleSearch(query)

  return (
    <>
      <h1 className="text-2xl font-bold tracking-tight">Search</h1>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          setParams(input ? { q: input } : {})
        }}
        className="flex gap-2"
      >
        <input
          type="search"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Search articles…"
          autoFocus
          className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        <button type="submit" className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
          Search
        </button>
      </form>
      <div className="rounded-xl border border-gray-200 bg-white p-4">
        {isFetched && query && (
          <ul className="divide-y divide-gray-100">
            {(results ?? []).map((r) => (
              <li key={r.id} className="py-2.5">
                <a href={r.url} target="_blank" rel="noopener" className="font-medium text-gray-900 hover:text-blue-600">
                  {r.title}
                </a>
                <div className="text-xs text-gray-500">{formatPublishedAt(r.published_at)}</div>
              </li>
            ))}
            {results && results.length === 0 && <li className="py-4 text-center text-gray-500">No results.</li>}
          </ul>
        )}
      </div>
    </>
  )
}
