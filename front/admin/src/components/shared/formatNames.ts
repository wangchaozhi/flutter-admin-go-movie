export function formatNames(ids: number[], names: Map<number, string>) {
  if (ids.length === 0) return '-'
  return ids.map((id) => names.get(id) ?? `#${id}`).join('、')
}
