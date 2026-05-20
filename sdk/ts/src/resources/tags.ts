import type { Transport } from "../_http.js";
import type { CreateTagRequest, Tag } from "../_models.js";
import { API_PREFIX } from "./index.js";

export interface CreateTagArgs {
  name: string;
  description?: string;
}

export interface UpdateTagArgs {
  name?: string;
  description?: string;
}

export class TagsResource {
  constructor(private readonly transport: Transport) {}

  async list(): Promise<Tag[]> {
    return this.transport.get<Tag[]>(`${API_PREFIX}/tags`);
  }

  async get(id: string): Promise<Tag> {
    return this.transport.get<Tag>(`${API_PREFIX}/tags/${id}`);
  }

  async create(args: CreateTagArgs): Promise<Tag> {
    const body: CreateTagRequest = { name: args.name };
    if (args.description !== undefined) {
      body.description = args.description;
    }
    return this.transport.post<Tag>(`${API_PREFIX}/tags`, body);
  }

  async update(id: string, args: UpdateTagArgs): Promise<Tag> {
    const body: CreateTagRequest = {};
    if (args.name !== undefined) {
      body.name = args.name;
    }
    if (args.description !== undefined) {
      body.description = args.description;
    }
    return this.transport.put<Tag>(`${API_PREFIX}/tags/${id}`, body);
  }

  async delete(id: string): Promise<void> {
    await this.transport.delete(`${API_PREFIX}/tags/${id}`);
  }
}
