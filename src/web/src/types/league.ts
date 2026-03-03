export interface League {
  id: string;
  platform: string;
  platform_league_id: string;
  name: string;
  season: string;
  sport: string;
  status: string;
  total_rosters: number;
  scoring_type?: string;
  entry_fee_cents: number;
  is_commissioner: boolean;
  imported_by: string;
  imported_at: string;
  last_synced_at?: string;
}

export type BonusType = 'weekly_high_score' | 'score_threshold' | 'highest_weekly_score'

export interface BonusCriteria {
  threshold?: number  // points required for score_threshold
}

export interface PayoutEntry {
  type: 'placement' | 'weekly';
  bonus_type?: BonusType;
  label: string;
  place?: number;       // placement entries only
  amount_cents: number;
  weeks?: number;       // weekly entries: number of occurrences
  criteria?: BonusCriteria;
}

export interface LeagueSettings {
  entry_fee_cents: number;
  total_rosters: number;
  payout_structure: PayoutEntry[];
  is_commissioner: boolean;
}

export interface PlatformLeague {
  platform_league_id: string;
  name: string;
  sport: string;
  season: string;
  status: string;
  total_rosters: number;
  scoring_type: string;
}

export interface LeagueMember {
  id: string
  league_id: string
  user_id: string | null
  platform: string
  platform_user_id: string
  platform_username: string
  team_name: string
  display_name: string
  avatar_url: string
  is_owner: boolean
  roster_id: number
  wins: number
  losses: number
  ties: number
  total_points: number
  wallet_address: string
  payment_status: 'unpaid' | 'paid' | 'refunded'
  joined_at: string
}

export interface LeagueDetail {
  league: League
  members: LeagueMember[]
}

export interface PlatformProfile {
  id: string;
  user_id: string;
  platform: string;
  platform_user_id: string;
  platform_username: string;
  display_name: string;
  avatar_url: string;
  linked_at: string;
  updated_at: string;
}
