import { z } from 'zod'

export const cpaBatchFormSchema = z
  .object({
    action: z.enum(['disable', 'enable', 'move', 'cost', 'archive', 'delete']),
    poolType: z.enum(['cpa_import', 'temporary']),
    cost: z.string(),
    costDate: z.string(),
    costNote: z.string().max(500),
    archiveReason: z.string().max(500),
    deleteConfirmation: z.string().max(64),
  })
  .superRefine((value, context) => {
    if (value.action !== 'cost') return
    const cost = Number(value.cost)
    if (!Number.isFinite(cost) || cost < 0) {
      context.addIssue({
        code: 'custom',
        path: ['cost'],
        message: 'Enter a valid non-negative cost',
      })
    }
    if (!value.costDate) {
      context.addIssue({
        code: 'custom',
        path: ['costDate'],
        message: 'Select a cost date',
      })
    }
  })

export type CPABatchFormValues = z.infer<typeof cpaBatchFormSchema>
