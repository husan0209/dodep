import * as React from 'react'
import { cn } from '@/lib/cn'

const Card = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> & { hover?: boolean; glass?: boolean }
>(({ className, hover = false, glass = false, ...props }, ref) => (
  <div
    ref={ref}
    className={cn(
      'rounded-2xl border border-border bg-bg-secondary overflow-hidden',
      'shadow-[0_4px_24px_-6px_rgba(0,0,0,0.4),inset_0_1px_0_0_rgba(255,255,255,0.03)]',
      hover && 'transition-all duration-300 ease-out hover:border-border-light hover:shadow-[0_8px_40px_-8px_rgba(0,0,0,0.5),inset_0_1px_0_0_rgba(255,255,255,0.05)] hover:-translate-y-0.5',
      glass && 'bg-bg-glass/75 backdrop-blur-xl backdrop-saturate-[1.2] border-border-light/30',
      className
    )}
    {...props}
  />
))
Card.displayName = 'Card'

const CardHeader = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn('flex flex-col space-y-1.5 p-5', className)}
    {...props}
  />
))
CardHeader.displayName = 'CardHeader'

const CardTitle = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
  <h3
    ref={ref}
    className={cn('text-base font-bold leading-tight tracking-tight text-text-primary', className)}
    {...props}
  />
))
CardTitle.displayName = 'CardTitle'

const CardDescription = React.forwardRef<
  HTMLParagraphElement,
  React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
  <p
    ref={ref}
    className={cn('text-sm text-text-muted', className)}
    {...props}
  />
))
CardDescription.displayName = 'CardDescription'

const CardContent = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div ref={ref} className={cn('p-5 pt-0', className)} {...props} />
))
CardContent.displayName = 'CardContent'

const CardFooter = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn('flex items-center p-5 pt-0', className)}
    {...props}
  />
))
CardFooter.displayName = 'CardFooter'

export { Card, CardHeader, CardFooter, CardTitle, CardDescription, CardContent }
