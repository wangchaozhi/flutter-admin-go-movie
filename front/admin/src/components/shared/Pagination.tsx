import { ChevronLeft, ChevronRight } from 'lucide-react'

export function Pagination({
  page,
  perPage,
  total,
  onPage,
}: {
  page: number
  perPage: number
  total: number
  onPage: (page: number) => void
}) {
  const totalPages = Math.max(1, Math.ceil(total / perPage))
  const from = total === 0 ? 0 : (page - 1) * perPage + 1
  const to = Math.min(total, page * perPage)

  return (
    <div className="pagination">
      <span className="pagination-info">
        共 {total} 条{total > 0 && ` · 第 ${from}-${to} 条`}
      </span>
      <div className="pagination-controls">
        <button type="button" disabled={page <= 1} onClick={() => onPage(page - 1)}>
          <ChevronLeft size={14} /> 上一页
        </button>
        <span className="pagination-page">{page} / {totalPages}</span>
        <button type="button" disabled={page >= totalPages} onClick={() => onPage(page + 1)}>
          下一页 <ChevronRight size={14} />
        </button>
      </div>
    </div>
  )
}
