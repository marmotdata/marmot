import type { Transport } from "../_http.js";
import type {
  AddGlossaryTermTagRequest,
  CreateTermRequest,
  GlossaryListResult,
  GlossaryTerm,
  RemoveGlossaryTermTagRequest,
  ReplaceGlossaryTermTagsRequest,
  Tag,
  UpdateTermRequest,
} from "../_models.js";
import { API_PREFIX } from "./index.js";

export interface GlossaryListOptions {
  limit?: number;
  offset?: number;
}

export interface GlossarySearchOptions extends GlossaryListOptions {
  query?: string;
  parentTermId?: string;
}

export interface CreateTermArgs {
  name: string;
  definition: string;
  description?: string;
  parentTermId?: string;
}

export interface UpdateTermArgs {
  name?: string;
  definition?: string;
  description?: string;
  parentTermId?: string;
}

export class GlossaryResource {
  constructor(private readonly transport: Transport) {}

  async list(opts: GlossaryListOptions = {}): Promise<GlossaryListResult> {
    const query: Record<string, unknown> = {};
    if (opts.limit !== undefined) query.limit = opts.limit;
    if (opts.offset !== undefined) query.offset = opts.offset;
    return this.transport.get<GlossaryListResult>(`${API_PREFIX}/glossary/list`, query);
  }

  async search(opts: GlossarySearchOptions = {}): Promise<GlossaryListResult> {
    const query: Record<string, unknown> = {};
    if (opts.query !== undefined) query.q = opts.query;
    if (opts.parentTermId !== undefined) query.parent_term_id = opts.parentTermId;
    if (opts.limit !== undefined) query.limit = opts.limit;
    if (opts.offset !== undefined) query.offset = opts.offset;
    return this.transport.get<GlossaryListResult>(`${API_PREFIX}/glossary/search`, query);
  }

  async get(termId: string): Promise<GlossaryTerm> {
    return this.transport.get<GlossaryTerm>(`${API_PREFIX}/glossary/${termId}`);
  }

  async create(args: CreateTermArgs): Promise<GlossaryTerm> {
    const body: CreateTermRequest = {
      name: args.name,
      definition: args.definition,
    };
    if (args.description) body.description = args.description;
    if (args.parentTermId) body.parent_term_id = args.parentTermId;
    return this.transport.post<GlossaryTerm>(`${API_PREFIX}/glossary/`, body);
  }

  async update(termId: string, args: UpdateTermArgs): Promise<GlossaryTerm> {
    const body: UpdateTermRequest = {};
    if (args.name) body.name = args.name;
    if (args.definition) body.definition = args.definition;
    if (args.description) body.description = args.description;
    if (args.parentTermId) body.parent_term_id = args.parentTermId;
    return this.transport.put<GlossaryTerm>(`${API_PREFIX}/glossary/${termId}`, body);
  }

  async delete(termId: string): Promise<void> {
    await this.transport.delete(`${API_PREFIX}/glossary/${termId}`);
  }

  async listTermTags(termId: string): Promise<Tag[]> {
    return this.transport.get<Tag[]>(`${API_PREFIX}/glossary/tags/${termId}`);
  }

  async addTermTag(termId: string, tagId: string): Promise<Tag[]> {
    const body: AddGlossaryTermTagRequest = { tag_id: tagId };
    return this.transport.post<Tag[]>(`${API_PREFIX}/glossary/tags/${termId}`, body);
  }

  async removeTermTag(termId: string, tagId: string): Promise<Record<string, string>> {
    const body: RemoveGlossaryTermTagRequest = { tag_id: tagId };
    return this.transport.delete<Record<string, string>>(
      `${API_PREFIX}/glossary/tags/${termId}`,
      body,
    );
  }

  async setTermTags(termId: string, tagIds: string[]): Promise<GlossaryTerm> {
    const body: ReplaceGlossaryTermTagsRequest = { tag_ids: tagIds };
    return this.transport.put<GlossaryTerm>(`${API_PREFIX}/glossary/tags/${termId}`, body);
  }
}
