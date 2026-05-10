export function CheckboxGroup<T extends { id: number }>({
  label,
  items,
  selected,
  getLabel,
  onChange,
}: {
  label: string
  items: T[]
  selected: number[]
  getLabel: (item: T) => string
  onChange: (ids: number[]) => void
}) {
  function toggle(id: number) {
    onChange(selected.includes(id) ? selected.filter((item) => item !== id) : [...selected, id])
  }

  return (
    <fieldset className="check-group">
      <legend>{label}</legend>
      <div>
        {items.map((item) => (
          <label key={item.id}>
            <input
              checked={selected.includes(item.id)}
              type="checkbox"
              onChange={() => toggle(item.id)}
            />
            <span>{getLabel(item)}</span>
          </label>
        ))}
        {items.length === 0 && <p className="empty">暂无可选数据</p>}
      </div>
    </fieldset>
  )
}
