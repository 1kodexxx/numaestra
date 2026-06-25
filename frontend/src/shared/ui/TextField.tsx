import { useId, useState } from 'react'

interface BaseProps {
  label: string
  value: string
  onChange: (v: string) => void
  error?: string | null
  supportingText?: string
  multiline?: boolean
  rows?: number
  type?: string
  placeholder?: string
  autoFocus?: boolean
  autoComplete?: string
  required?: boolean
  disabled?: boolean
  /** Background color behind the floating label (should match the field's container). */
  surfaceColor?: string
}

/**
 * Material outlined text field with floating label, focus ring and error state.
 */
export function TextField({
  label, value, onChange, error, supportingText,
  multiline = false, rows = 3, type = 'text', placeholder,
  autoFocus, autoComplete, required, disabled, surfaceColor = '#0f0f0f',
}: BaseProps) {
  const id = useId()
  const [focused, setFocused] = useState(false)
  const floated = focused || value.length > 0
  const accent = error ? '#ef4444' : focused ? '#00e5c0' : 'rgba(255,255,255,0.16)'

  const fieldStyle: React.CSSProperties = {
    width: '100%',
    background: 'transparent',
    border: `${focused || error ? '2px' : '1px'} solid ${accent}`,
    borderRadius: '12px',
    padding: multiline ? '15px 15px' : '0 15px',
    height: multiline ? undefined : '52px',
    color: '#fff',
    fontSize: '15px',
    fontFamily: 'inherit',
    lineHeight: multiline ? 1.6 : undefined,
    outline: 'none',
    resize: multiline ? 'vertical' : undefined,
    transition: 'border-color var(--dur-short) var(--ease-standard)',
    /* keep text from shifting when border grows 1px→2px */
    margin: focused || error ? 0 : '1px',
    opacity: disabled ? 0.5 : 1,
    cursor: disabled ? 'not-allowed' : undefined,
  }

  return (
    <div style={{ width: '100%' }}>
      <div style={{ position: 'relative' }}>
        <label
          htmlFor={id}
          style={{
            position: 'absolute',
            left: floated ? '12px' : '15px',
            top: floated ? '-8px' : (multiline ? '16px' : '50%'),
            transform: floated ? 'none' : (multiline ? 'none' : 'translateY(-50%)'),
            fontSize: floated ? '12px' : '15px',
            fontWeight: floated ? 600 : 400,
            color: error ? '#ef4444' : focused ? '#00e5c0' : 'rgba(255,255,255,0.5)',
            background: floated ? surfaceColor : 'transparent',
            padding: floated ? '0 6px' : '0',
            pointerEvents: 'none',
            transition: 'all var(--dur-short) var(--ease-standard)',
            letterSpacing: '0.01em',
          }}
        >
          {label}{required ? ' *' : ''}
        </label>

        {multiline ? (
          <textarea
            id={id}
            value={value}
            rows={rows}
            placeholder={focused ? placeholder : undefined}
            onChange={(e) => onChange(e.target.value)}
            onFocus={() => setFocused(true)}
            onBlur={() => setFocused(false)}
            autoFocus={autoFocus}
            autoComplete={autoComplete}
            required={required}
            disabled={disabled}
            style={fieldStyle}
          />
        ) : (
          <input
            id={id}
            type={type}
            value={value}
            placeholder={focused ? placeholder : undefined}
            onChange={(e) => onChange(e.target.value)}
            onFocus={() => setFocused(true)}
            onBlur={() => setFocused(false)}
            autoFocus={autoFocus}
            autoComplete={autoComplete}
            required={required}
            disabled={disabled}
            aria-required={required || undefined}
            style={fieldStyle}
          />
        )}
      </div>

      {(error || supportingText) && (
        <div style={{
          fontSize: '12px',
          color: error ? '#ef4444' : 'rgba(255,255,255,0.38)',
          margin: '6px 15px 0',
        }}>
          {error || supportingText}
        </div>
      )}
    </div>
  )
}
