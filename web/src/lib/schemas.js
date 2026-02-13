import { z } from 'zod'

const toNumber = (v) => {
  if (typeof v === 'number') return v
  if (typeof v === 'string' && v.trim() !== '') {
    const n = Number(v)
    if (!Number.isNaN(n)) return n
  }
  return undefined
}

const Num = z.preprocess(toNumber, z.number())
const NumDefault0 = z.preprocess((v) => toNumber(v) ?? 0, z.number())
const StrDefault = z.preprocess((v) => (v == null ? '' : String(v)), z.string())

export const PublicRoomsSchema = z.object({
  items: z.array(
    z.object({
      id: z.string(),
      name: z.string().optional().default(''),
      min_buyin_cc: NumDefault0,
      small_blind_cc: NumDefault0,
      big_blind_cc: NumDefault0
    })
  ).default([])
})

export const PublicTablesSchema = z.object({
  items: z.array(
    z.object({
      table_id: z.string(),
      room_id: z.string().optional().default(''),
      status: z.string().optional().default(''),
      created_at: z.string().optional().default(''),
      small_blind_cc: NumDefault0,
      big_blind_cc: NumDefault0
    })
  ).default([])
})

export const AgentTableSchema = z.object({
  agent_id: z.string(),
  room_id: z.string(),
  table_id: z.string()
})

export const LeaderboardSchema = z.object({
  items: z.array(z.object({
    rank: NumDefault0,
    agent_id: z.string().optional().default(''),
    name: z.string().optional().default(''),
    score: NumDefault0,
    bb_per_100: NumDefault0,
    net_cc_from_play: NumDefault0,
    hands_played: NumDefault0,
    win_rate: NumDefault0,
    last_active_at: StrDefault.optional().default('')
  })).default([]),
  total: NumDefault0.optional().default(0),
  limit: NumDefault0.optional().default(0),
  offset: NumDefault0.optional().default(0)
})

export const TableHistorySchema = z.object({
  items: z.array(
    z.object({
      table_id: z.string(),
      room_id: z.string().optional().default(''),
      room_name: z.string().optional().default(''),
      status: z.string().optional().default(''),
      small_blind_cc: NumDefault0,
      big_blind_cc: NumDefault0,
      hands_played: NumDefault0.optional().default(0),
      participants: z.array(z.object({
        agent_id: z.string().optional().default(''),
        agent_name: z.string().optional().default('')
      })).optional().default([]),
      created_at: StrDefault.optional().default(''),
      last_hand_ended_at: StrDefault.optional().default('')
    })
  ).default([]),
  total: NumDefault0.optional().default(0),
  limit: NumDefault0.optional(),
  offset: NumDefault0.optional()
})

const AgentPerformanceSnapshotSchema = z.object({
  score: NumDefault0.optional().default(0),
  bb_per_100: NumDefault0.optional().default(0),
  net_cc_from_play: NumDefault0.optional().default(0),
  hands_played: NumDefault0.optional().default(0),
  win_rate: NumDefault0.optional().default(0),
  last_active_at: StrDefault.optional().default('')
})

export const AgentProfileSchema = z.object({
  agent: z.object({
    agent_id: z.string().optional().default(''),
    name: z.string().optional().default(''),
    created_at: StrDefault.optional().default('')
  }).optional().default({ agent_id: '', name: '', created_at: '' }),
  stats_30d: AgentPerformanceSnapshotSchema.optional().default({}),
  stats_all: AgentPerformanceSnapshotSchema.optional().default({}),
  tables: TableHistorySchema.optional().default({})
})

export const TableReplayResponseSchema = z.object({
  items: z.array(
    z.object({
      id: z.string(),
      hand_id: z.string().nullable().optional(),
      global_seq: NumDefault0,
      event_type: z.string(),
      actor_agent_id: z.string().nullable().optional(),
      payload: z.record(z.string(), z.any()).optional().default({})
    })
  ).default([]),
  next_from_seq: NumDefault0.optional(),
  has_more: z.boolean().optional().default(false)
})

export const TableTimelineResponseSchema = z.object({
  items: z.array(
    z.object({
      hand_id: z.string(),
      start_seq: NumDefault0.optional(),
      end_seq: NumDefault0.optional(),
      winner_agent_id: z.string().optional().default(''),
      pot_cc: NumDefault0.optional(),
      street_end: z.string().optional().default(''),
      started_at: StrDefault.optional().default(''),
      ended_at: StrDefault.optional().default('')
    })
  ).default([])
})

export const TableSnapshotResponseSchema = z.object({
  state: z.object({
    table_id: z.string().optional().default(''),
    hand_id: z.string().optional().default(''),
    street: z.string().optional().default('-'),
    pot_cc: NumDefault0.optional(),
    board_cards: z.array(z.string()).optional().default([]),
    current_actor_seat: Num.optional().nullable().optional(),
    seat_map: z.array(z.object({
      seat_id: NumDefault0,
      agent_id: z.string().optional().default(''),
      agent_name: z.string().optional().default('')
    })).optional().default([]),
    stacks: z.array(z.object({
      seat_id: NumDefault0,
      stack: NumDefault0,
      agent_id: z.string().optional().default(''),
      last_action: z.string().optional().default(''),
      last_action_amount: NumDefault0.optional()
    })).optional().default([])
  })
})
