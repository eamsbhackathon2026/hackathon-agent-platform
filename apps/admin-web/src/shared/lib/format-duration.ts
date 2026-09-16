export function formatDuration(milliseconds: number) {
  if (milliseconds < 1_000) return `${Math.max(0, milliseconds)} ms`;
  if (milliseconds < 60_000) return `${(milliseconds / 1_000).toFixed(1)} sec`;
  return `${Math.floor(milliseconds / 60_000)} min ${Math.round((milliseconds % 60_000) / 1_000)} sec`;
}
