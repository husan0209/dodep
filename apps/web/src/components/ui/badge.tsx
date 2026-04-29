import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/cn'

const badgeVariants = cva(
  'inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-bold uppercase tracking-wider transition-colors',
  {
    variants: {
      variant: {
        default: 'bg-accent-gold/15 text-accent-gold border border-accent-gold/20',
        live: 'bg-red-500/15 text-red-400 border border-red-500/20',
        gold: 'bg-yellow-500/15 text-yellow-400 border border-yellow-500/20',
        cyan: 'bg-cyan-500/15 text-cyan-400 border border-cyan-500/20',
        emerald: 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/20',
        violet: 'bg-violet-500/15 text-violet-400 border border-violet-500/20',
        outline: 'border border-border-light text-text-secondary',
        ghost: 'bg-transparent text-text-muted',
      },
      size: {
        default: 'px-2 py-0.5 text-[11px]',
        sm: 'px-1.5 py-0.5 text-[10px]',
        lg: 'px-2.5 py-1 text-xs',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, size, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant, size, className }))} {...props} />
  )
}

export { Badge, badgeVariants }
