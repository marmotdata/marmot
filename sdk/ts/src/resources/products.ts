import type { Transport } from "../_http.js";
import type {
  AddDataProductTagRequest,
  DataProduct,
  DataProductListResult,
  RemoveDataProductTagRequest,
  ReplaceDataProductTagsRequest,
  Tag,
} from "../_models.js";
import { API_PREFIX } from "./index.js";

export interface DataProductsListOptions {
  limit?: number;
  offset?: number;
}

export class DataProductsResource {
  constructor(private readonly transport: Transport) {}

  async list(opts: DataProductsListOptions = {}): Promise<DataProductListResult> {
    const query: Record<string, unknown> = {};
    if (opts.limit !== undefined) query.limit = opts.limit;
    if (opts.offset !== undefined) query.offset = opts.offset;
    return this.transport.get<DataProductListResult>(`${API_PREFIX}/products/list`, query);
  }

  async get(id: string): Promise<DataProduct> {
    return this.transport.get<DataProduct>(`${API_PREFIX}/products/${id}`);
  }

  async listTags(productId: string): Promise<Tag[]> {
    return this.transport.get<Tag[]>(`${API_PREFIX}/products/tags/${productId}`);
  }

  async addTag(productId: string, tagId: string): Promise<Tag[]> {
    const body: AddDataProductTagRequest = { tag_id: tagId };
    return this.transport.post<Tag[]>(`${API_PREFIX}/products/tags/${productId}`, body);
  }

  async removeTag(productId: string, tagId: string): Promise<Record<string, string>> {
    const body: RemoveDataProductTagRequest = { tag_id: tagId };
    return this.transport.delete<Record<string, string>>(
      `${API_PREFIX}/products/tags/${productId}`,
      body,
    );
  }

  async setTags(productId: string, tagIds: string[]): Promise<DataProduct> {
    const body: ReplaceDataProductTagsRequest = { tag_ids: tagIds };
    return this.transport.put<DataProduct>(`${API_PREFIX}/products/tags/${productId}`, body);
  }
}
