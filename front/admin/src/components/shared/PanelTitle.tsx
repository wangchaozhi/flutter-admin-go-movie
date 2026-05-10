export function PanelTitle({ title, count }: { title: string; count?: number }) {
  return (
    <div className="panel-title">
      <h2>{title}</h2>
      {typeof count === 'number' && <span>{count}</span>}
    </div>
  )
}
