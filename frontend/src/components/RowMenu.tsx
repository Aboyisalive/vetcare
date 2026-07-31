import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

export interface RowAction {
  label: string
  danger?: boolean
  onClick: () => void
}

const GAP = 4
const EDGE = 8

export default function RowMenu({ actions }: { actions: RowAction[] }) {
  const [open, setOpen] = useState(false)
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null)
  const btnRef = useRef<HTMLButtonElement>(null)
  const popRef = useRef<HTMLDivElement>(null)

  // The popup renders in a portal on document.body: tables set `overflow:
  // hidden` for their rounded corners, which would otherwise clip it.
  // Position is therefore measured from the trigger each time it opens.
  useLayoutEffect(() => {
    if (!open) return
    const place = () => {
      const trigger = btnRef.current?.getBoundingClientRect()
      const pop = popRef.current
      if (!trigger || !pop) return

      const { offsetHeight: h, offsetWidth: w } = pop
      let top = trigger.bottom + GAP
      if (top + h > window.innerHeight - EDGE) {
        // Not enough room below — flip above the trigger when that fits.
        const above = trigger.top - GAP - h
        top = above >= EDGE ? above : Math.max(EDGE, window.innerHeight - EDGE - h)
      }
      const left = Math.min(
        Math.max(EDGE, trigger.right - w),
        window.innerWidth - w - EDGE,
      )
      setPos({ top, left })
    }

    place()
    // Capture phase so scrolling inside any ancestor repositions the menu too.
    window.addEventListener('scroll', place, true)
    window.addEventListener('resize', place)
    return () => {
      window.removeEventListener('scroll', place, true)
      window.removeEventListener('resize', place)
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    const onPointerDown = (e: MouseEvent) => {
      const target = e.target as Node
      if (btnRef.current?.contains(target) || popRef.current?.contains(target)) return
      setOpen(false)
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  if (actions.length === 0) return null

  const toggle = () => {
    setPos(null)
    setOpen((o) => !o)
  }

  return (
    <div className="row-menu">
      <button
        ref={btnRef}
        className="btn small menu-btn"
        aria-label="Options"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={toggle}
      >
        ⋯
      </button>
      {open &&
        createPortal(
          <div
            ref={popRef}
            className="menu-pop"
            role="menu"
            // Hidden for the first paint only, while the size is measured.
            style={{
              top: pos?.top ?? 0,
              left: pos?.left ?? 0,
              visibility: pos ? 'visible' : 'hidden',
            }}
          >
            {actions.map((a) => (
              <button
                key={a.label}
                role="menuitem"
                className={a.danger ? 'danger' : ''}
                onClick={() => {
                  setOpen(false)
                  a.onClick()
                }}
              >
                {a.label}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </div>
  )
}
