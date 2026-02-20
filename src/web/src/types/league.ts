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

export interface PayoutEntry {
  type: 'placement' | 'weekly';
  label: string;
  place?: number;       // placement entries only
  amount_cents: number;
  weeks?: number;       // weekly entries: number of occurrences
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
