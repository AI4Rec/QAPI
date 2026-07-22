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
import {
  ChevronLeft as ChevronLeftIcon,
  ChevronRight as ChevronRightIcon,
  ChevronsLeft as DoubleArrowLeftIcon,
  ChevronsRight as DoubleArrowRightIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn, getPageNumbers } from '@/lib/utils'

const DEFAULT_PAGE_SIZE_OPTIONS = [10, 20, 30, 50, 100] as const

export type PaginationControlsProps = {
  pageIndex: number
  pageSize: number
  totalCount: number
  pageCount: number
  pageSizeOptions?: readonly number[]
  compact?: boolean
  onPageIndexChange: (pageIndex: number) => void
  onPageSizeChange: (pageSize: number) => void
}

export function PaginationControls(props: PaginationControlsProps) {
  const { t } = useTranslation()
  const pageSizeOptions = props.pageSizeOptions ?? DEFAULT_PAGE_SIZE_OPTIONS
  const totalPages = Math.max(0, props.pageCount)
  const lastPageIndex = Math.max(0, totalPages - 1)
  const pageIndex = Math.min(Math.max(0, props.pageIndex), lastPageIndex)
  const currentPage = totalPages === 0 ? 0 : pageIndex + 1
  const pageNumbers = getPageNumbers(currentPage, totalPages)
  const canPreviousPage = pageIndex > 0
  const canNextPage = totalPages > 0 && pageIndex < lastPageIndex
  const selectItems = pageSizeOptions.map((pageSize) => ({
    value: `${pageSize}`,
    label: pageSize,
  }))

  return (
    <div
      className='@container/pagination flex min-w-0 items-center justify-end overflow-clip'
      style={{ overflowClipMargin: 1 }}
    >
      <div className='flex min-w-0 shrink-0 items-center gap-2 @xl/pagination:gap-3'>
        {!props.compact && (
          <div className='flex shrink-0 items-baseline gap-1.5 text-xs font-medium whitespace-nowrap sm:text-sm'>
            <span className='text-muted-foreground/80'>{t('Total:')}</span>
            <span className='text-foreground tabular-nums'>
              {props.totalCount.toLocaleString()}
            </span>
          </div>
        )}

        <div className='flex shrink-0 items-center gap-1.5 @lg/pagination:gap-2'>
          {!props.compact && (
            <p className='text-muted-foreground/80 hidden text-sm font-medium whitespace-nowrap @2xl/pagination:block'>
              {t('Rows per page')}
            </p>
          )}
          <Select
            items={selectItems}
            value={`${props.pageSize}`}
            onValueChange={(value) => {
              props.onPageSizeChange(Number(value))
            }}
          >
            <SelectTrigger className='text-foreground h-8 w-[64px] font-medium tabular-nums sm:w-[70px]'>
              <SelectValue placeholder={props.pageSize} />
            </SelectTrigger>
            <SelectContent side='top' alignItemWithTrigger={false}>
              <SelectGroup>
                {pageSizeOptions.map((pageSize) => (
                  <SelectItem key={pageSize} value={`${pageSize}`}>
                    {pageSize}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>

        <div className='flex min-w-0 shrink-0 items-center gap-1 @lg/pagination:gap-1.5 @xl/pagination:gap-2'>
          <Button
            variant='outline'
            className={cn(
              'text-muted-foreground hover:text-foreground disabled:text-muted-foreground/50 size-8 p-0',
              (props.compact || totalPages <= 1) && 'hidden',
              '@max-lg/pagination:hidden'
            )}
            onClick={() => props.onPageIndexChange(0)}
            disabled={!canPreviousPage}
          >
            <span className='sr-only'>{t('Go to first page')}</span>
            <DoubleArrowLeftIcon className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            className='text-muted-foreground hover:text-foreground disabled:text-muted-foreground/50 size-8 p-0'
            onClick={() => props.onPageIndexChange(pageIndex - 1)}
            disabled={!canPreviousPage}
          >
            <span className='sr-only'>{t('Go to previous page')}</span>
            <ChevronLeftIcon className='h-4 w-4' />
          </Button>

          {pageNumbers.map((pageNumber, index) => {
            let pageKey: string | number = pageNumber
            if (pageNumber === '...') {
              pageKey = index === 1 ? 'ellipsis-start' : 'ellipsis-end'
            }
            return (
              <div key={pageKey} className='flex items-center'>
                {pageNumber === '...' ? (
                  <span className='text-muted-foreground/60 px-0.5 text-sm @lg/pagination:px-1'>
                    ...
                  </span>
                ) : (
                  <Button
                    variant={currentPage === pageNumber ? 'default' : 'outline'}
                    className={cn(
                      'h-8 min-w-8 px-2 tabular-nums',
                      currentPage === pageNumber
                        ? 'font-semibold'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                    onClick={() =>
                      props.onPageIndexChange((pageNumber as number) - 1)
                    }
                  >
                    <span className='sr-only'>
                      {t('Go to page {{page}}', { page: pageNumber })}
                    </span>
                    {pageNumber}
                  </Button>
                )}
              </div>
            )
          })}

          <Button
            variant='outline'
            className='text-muted-foreground hover:text-foreground disabled:text-muted-foreground/50 size-8 p-0'
            onClick={() => props.onPageIndexChange(pageIndex + 1)}
            disabled={!canNextPage}
          >
            <span className='sr-only'>{t('Go to next page')}</span>
            <ChevronRightIcon className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            className={cn(
              'text-muted-foreground hover:text-foreground disabled:text-muted-foreground/50 size-8 p-0',
              (props.compact || totalPages <= 1) && 'hidden',
              '@max-lg/pagination:hidden'
            )}
            onClick={() => props.onPageIndexChange(lastPageIndex)}
            disabled={!canNextPage}
          >
            <span className='sr-only'>{t('Go to last page')}</span>
            <DoubleArrowRightIcon className='h-4 w-4' />
          </Button>
        </div>
      </div>
    </div>
  )
}
