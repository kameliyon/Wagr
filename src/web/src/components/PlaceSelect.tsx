import { useState, useRef, useEffect } from 'react'
import './PlaceSelect.css'

/* ── Shared dropdown internals ────────────────────────────── */

function SelectChevron() {
  return (
    <svg className="place-select__chevron" width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden="true">
      <path d="M2 4.5L6 8.5L10 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function SelectCheck() {
  return (
    <svg className="place-select__check" width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
      <path d="M1.5 5L4 7.5L8.5 2.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

/* ── CustomSelect — string-based, generic ─────────────────── */

export interface SelectOption {
  value: string
  label: string
}

interface CustomSelectProps {
  value: string
  options: SelectOption[]
  onChange: (value: string) => void
  width?: string | number
}

export function CustomSelect({ value, options, onChange, width }: CustomSelectProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const selectedLabel = options.find((o) => o.value === value)?.label ?? 'Select…'

  return (
    <div className={`place-select${open ? ' place-select--open' : ''}`} ref={ref} style={width ? { width } : undefined}>
      <button
        type="button"
        className="place-select__trigger"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="place-select__value">{selectedLabel}</span>
        <SelectChevron />
      </button>

      <div className="place-select__dropdown" role="listbox">
        {options.map((opt) => {
          const isSelected = opt.value === value
          return (
            <button
              key={opt.value}
              type="button"
              role="option"
              aria-selected={isSelected}
              className={`place-select__option${isSelected ? ' place-select__option--selected' : ''}`}
              onClick={() => { onChange(opt.value); setOpen(false) }}
            >
              {isSelected && <SelectCheck />}
              <span>{opt.label}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}

/* ── PlaceSelect — number-based wrapper ───────────────────── */

interface PlaceSelectProps {
  value: number | ''
  options: number[]
  getLabel: (n: number) => string
  onChange: (n: number) => void
}

export function PlaceSelect({ value, options, getLabel, onChange }: PlaceSelectProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const selectedLabel = value !== '' ? getLabel(value) : 'Select place…'

  return (
    <div className={`place-select${open ? ' place-select--open' : ''}`} ref={ref}>
      <button
        type="button"
        className="place-select__trigger"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="place-select__value">{selectedLabel}</span>
        <svg
          className="place-select__chevron"
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
        >
          <path d="M2 4.5L6 8.5L10 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>

      <div className="place-select__dropdown" role="listbox">
        {options.map((n) => {
          const isSelected = value === n
          return (
            <button
              key={n}
              type="button"
              role="option"
              aria-selected={isSelected}
              className={`place-select__option${isSelected ? ' place-select__option--selected' : ''}`}
              onClick={() => {
                onChange(n)
                setOpen(false)
              }}
            >
              {isSelected && (
                <svg className="place-select__check" width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
                  <path d="M1.5 5L4 7.5L8.5 2.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              )}
              <span>{getLabel(n)}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
