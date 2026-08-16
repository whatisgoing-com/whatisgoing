import { NavLink } from 'react-router-dom'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  `rounded-md px-3 py-1.5 text-sm font-medium ${isActive ? 'bg-gray-100 text-gray-900' : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'}`

export function Header() {
  return (
    <header className="border-b border-gray-200 bg-white">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-4">
        <NavLink to="/" className="font-brand text-xl font-extrabold tracking-tight text-[#1C2430]">
          what is<span className="rounded-[3px] bg-[#1C2430] px-1 text-[#F6F3EC]">going</span>
          <span className="font-normal text-gray-400">.com</span>
        </NavLink>
        <nav className="flex gap-1">
          <NavLink to="/" end className={navLinkClass}>
            Trending
          </NavLink>
          <NavLink to="/search" className={navLinkClass}>
            Search
          </NavLink>
        </nav>
      </div>
    </header>
  )
}
