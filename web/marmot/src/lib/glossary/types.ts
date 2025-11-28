export interface Owner {
  id: string;
  username?: string;
  name: string;
  type: 'user' | 'team';
  email?: string;
}

export interface GlossaryTerm {
  id: string;
  name: string;
  definition: string;
  description?: string;
  parent_term_id?: string;
  owners: Owner[];
  metadata: Record<string, any>;
  tags: string[];
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface OwnerInput {
  id: string;
  type: 'user' | 'team';
}

export interface CreateTermInput {
  name: string;
  definition: string;
  description?: string;
  parent_term_id?: string;
  owners?: OwnerInput[];
  metadata?: Record<string, any>;
  tags?: string[];
}

export interface UpdateTermInput {
  name?: string;
  definition?: string;
  description?: string;
  parent_term_id?: string;
  owners?: OwnerInput[];
  metadata?: Record<string, any>;
  tags?: string[];
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
