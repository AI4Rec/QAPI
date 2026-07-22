/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { Table } from '@tanstack/react-table'

import { PaginationControls } from '@/components/pagination-controls'

type DataTablePaginationProps<TData> = {
  table: Table<TData>
}

export function DataTablePagination<TData>({
  table,
}: DataTablePaginationProps<TData>) {
  const pagination = table.getState().pagination

  return (
    <PaginationControls
      pageIndex={pagination.pageIndex}
      pageSize={pagination.pageSize}
      totalCount={table.getRowCount()}
      pageCount={table.getPageCount()}
      onPageIndexChange={(pageIndex) => table.setPageIndex(pageIndex)}
      onPageSizeChange={(pageSize) => table.setPageSize(pageSize)}
    />
  )
}
