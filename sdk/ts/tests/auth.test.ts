import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { resolve } from "../src/auth/index.js";
import type { SubjectToken, WorkloadIdentitySource } from "../src/auth/workload/index.js";
import { AuthError } from "../src/errors.js";

class StaticSource implements WorkloadIdentitySource {
  constructor(
    public readonly name: string,
    private readonly token: string | null,
  ) {}
  async fetch(): Promise<SubjectToken | null> {
    return this.token
      ? { token: this.token, tokenType: "urn:ietf:params:oauth:token-type:id_token" }
      : null;
  }
}

const KEYS_TO_RESET = [
  "MARMOT_API_KEY",
  "MARMOT_TOKEN",
  "MARMOT_HOST",
  "MARMOT_CONTEXT",
  "XDG_CONFIG_HOME",
];

describe("auth chain ordering", () => {
  let saved: Record<string, string | undefined> = {};

  beforeEach(() => {
    saved = {};
    for (const k of KEYS_TO_RESET) {
      saved[k] = process.env[k];
      delete process.env[k];
    }
    // Point XDG_CONFIG_HOME at a path that definitely has no cached creds.
    process.env.XDG_CONFIG_HOME = "/nonexistent-marmot-test-dir";
  });

  afterEach(() => {
    for (const k of KEYS_TO_RESET) {
      if (saved[k] === undefined) delete process.env[k];
      else process.env[k] = saved[k];
    }
  });

  test("explicit api key wins", async () => {
    process.env.MARMOT_API_KEY = "should-not-be-used";
    process.env.MARMOT_TOKEN = "should-not-be-used";
    const { credential } = await resolve({ baseUrl: "http://x", apiKey: "explicit-key" });
    expect(credential.scheme).toBe("X-API-Key");
    expect(credential.token).toBe("explicit-key");
  });

  test("explicit token wins", async () => {
    const { credential } = await resolve({ baseUrl: "http://x", token: "explicit-tok" });
    expect(credential.scheme).toBe("Bearer");
    expect(credential.token).toBe("explicit-tok");
  });

  test("env api key", async () => {
    process.env.MARMOT_API_KEY = "env-key";
    const { credential } = await resolve({ baseUrl: "http://x" });
    expect(credential.token).toBe("env-key");
  });

  test("api key beats token in env", async () => {
    process.env.MARMOT_API_KEY = "k";
    process.env.MARMOT_TOKEN = "t";
    const { credential } = await resolve({ baseUrl: "http://x" });
    expect(credential.scheme).toBe("X-API-Key");
  });

  test("no credential and no host throws", async () => {
    await expect(resolve({ workloadSources: [] })).rejects.toThrow(AuthError);
  });

  test("no credential throws", async (ctx) => {
    process.env.XDG_CONFIG_HOME = ctx.task.id;
    await expect(resolve({ baseUrl: "http://x", workloadSources: [] })).rejects.toThrow(
      /no Marmot credentials/,
    );
  });

  test("workload source runs when no other credential", async () => {
    const fetchImpl = vi.fn(
      async () =>
        new Response(JSON.stringify({ access_token: "exchanged-jwt", expires_in: 3600 }), {
          status: 200,
        }),
    );
    // Override default sources with our static one; pass fetch via no-op since exchange uses global fetch.
    const src = new StaticSource("test", "subject-jwt");
    globalThis.fetch = fetchImpl as unknown as typeof fetch;
    try {
      const { credential } = await resolve({
        baseUrl: "http://x",
        workloadSources: [src],
      });
      expect(credential.token).toBe("exchanged-jwt");
      expect(credential.scheme).toBe("Bearer");
      expect(credential.source).toContain("test");
    } finally {
      // restore — vitest doesn't auto-restore globals
    }
  });

  test("explicit credential skips workload sources", async () => {
    const src: WorkloadIdentitySource = {
      name: "should-not-fire",
      fetch: vi.fn(async () => null),
    };
    await resolve({ baseUrl: "http://x", token: "explicit", workloadSources: [src] });
    expect(src.fetch).not.toHaveBeenCalled();
  });
});
