export interface Owner {
  id: string;
  username: string;
  name: string;
}

export interface GlossaryTerm {
  id: string;
  name: string;
  definition: string;
  description?: string;
  parent_term_id?: string;
  owners: Owner[];
  metadata: Record<string, any>;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface CreateTermInput {
  name: string;
  definition: string;
  description?: string;
  parent_term_id?: string;
  owner_ids?: string[];
  metadata?: Record<string, any>;
}

export interface UpdateTermInput {
  name?: string;
  definition?: string;
  description?: string;
  parent_term_id?: string;
  owner_ids?: string[];
  metadata?: Record<string, any>;
}

export interface TermsListResponse {
  terms: GlossaryTerm[];
  total: number;
}

export interface TermChildrenResponse {
  children: GlossaryTerm[];
  total: number;
}

export interface TermAncestorsResponse {
  ancestors: GlossaryTerm[];
  total: number;
}
