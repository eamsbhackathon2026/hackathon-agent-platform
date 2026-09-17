import * as React from "react"

import { cn } from "@/shared/lib/cn"

import { Card } from "./card"

/**
 * Shared shell for every row collection: a card whose own header names the list
 * and says how much of it is on screen, above a body that runs to the card edge.
 * Rows may be table markup or a responsive grid; the shell looks the same either way.
 */
const TableCard = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> & {
    title: string
    description?: React.ReactNode
    /** Name of the region, when a screen reader needs something other than the title. */
    label?: string
  }
>(({ className, title, description, label, children, ...props }, ref) => (
  <Card
    ref={ref}
    role="region"
    aria-label={label ?? title}
    className={cn("overflow-hidden", className)}
    {...props}
  >
    <div className="border-b bg-muted/30 px-5 py-4 sm:px-6">
      <h2 className="font-semibold tracking-[-0.015em]">{title}</h2>
      {description ? <p className="mt-1 text-sm text-muted-foreground">{description}</p> : null}
    </div>
    {children}
  </Card>
))
TableCard.displayName = "TableCard"

/** Padding for a state that replaces the rows, so it lines up with the header above it. */
const TableCardState = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <div ref={ref} className={cn("px-5 py-6 sm:px-6", className)} {...props} />
))
TableCardState.displayName = "TableCardState"

export { TableCard, TableCardState }
