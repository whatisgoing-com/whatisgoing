import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useEntitySearch } from '../api/hooks'
import { TypeBadge } from './TypeBadge'

export function EntitySearch() {
  const [input, setInput] = useState('')
  const [debounced, setDebounced] = useState('')
  const [open, setOpen] = useState(false)

  useEffect(() => {
    const t = setTimeout(() => setDebounced(input), 300)
    return () => clearTimeout(t)
  }, [input])

  const { data: results } = useEntitySearch(debounced)

  return (
    <div className="relative w-full sm:w-72">
      <input
        type="search"
        placeholder="Find an entity…"
        autoComplete="off"
        value={input}
        onChange={(e) => {
          setInput(e.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      />
      {open && results && results.length > 0 && (
        <ul className="absolute z-10 mt-1 w-full divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white shadow-lg">
          {results.map((r) => (
            <li key={r.id}>
              <Link to={`/entities/${r.id}`} className="flex items-center justify-between px-3 py-2 text-sm hover:bg-gray-50">
                <span className="font-medium text-gray-900">{r.name}</span>
                <TypeBadge type={r.type} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
