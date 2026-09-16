import { cn } from "@/shared/lib/cn"

function Skeleton({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("animate-pulse rounded-xl bg-secondary", className)}
      {...props}
    />
  )
}

export { Skeleton }
