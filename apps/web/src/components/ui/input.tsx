import * as React from 'react'
import { cn } from '@/lib/cn'

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  error?: boolean
  icon?: React.ReactNode
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, error, icon, ...props }, ref) => {
    return (
      <div className="relative w-full">
        {icon && (
          <div className="absolute left-3 top-1/2 -translate-y-1/2 text-text-muted">
            {icon}
          </div>
        )}
        <input
          type={type}
          className={cn(
            'flex w-full rounded-xl border bg-bg-primary px-4 py-3 text-sm text-text-primary',
            'placeholder:text-text-muted',
            'focus:outline-none focus:ring-2 focus:ring-accent-gold/30 focus:border-accent-gold/50',
            'transition-all duration-200',
            'disabled:cursor-not-allowed disabled:opacity-50',
            error && 'border-red-500/40 focus:ring-red-500/30 focus:border-red-500/60',
            icon && 'pl-10',
            className
          )}
          ref={ref}
          {...props}
        />
      </div>
    )
  }
)
Input.displayName = 'Input'

export { Input }
