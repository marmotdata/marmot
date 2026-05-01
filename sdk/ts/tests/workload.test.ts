import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { GCPWorkloadIdentitySource } from "../src/auth/workload/gcp.js";
import { GitHubActionsSource } from "../src/auth/workload/github.js";
import { KubernetesServiceAccountSource } from "../src/auth/workload/kubernetes.js";

describe("kubernetes source", () => {
  test("returns null when token file missing", async () => {
    const src = new KubernetesServiceAccountSource({ tokenPath: "/nonexistent/path" });
    expect(await src.fetch()).toBeNull();
  });

  test("reads token from file", async () => {
    const dir = mkdtempSync(join(tmpdir(), "marmot-k8s-"));
    const path = join(dir, "token");
    writeFileSync(path, "k8s-sa-jwt\n");
    const src = new KubernetesServiceAccountSource({ tokenPath: path });
    const tok = await src.fetch();
    expect(tok?.token).toBe("k8s-sa-jwt");
  });

  test("returns null for empty token", async () => {
    const dir = mkdtempSync(join(tmpdir(), "marmot-k8s-"));
    const path = join(dir, "token");
    writeFileSync(path, "  \n");
    const src = new KubernetesServiceAccountSource({ tokenPath: path });
    expect(await src.fetch()).toBeNull();
  });
});

describe("gcp source", () => {
  const KEYS = ["GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT", "K_SERVICE", "FUNCTION_TARGET"];
  let saved: Record<string, string | undefined> = {};

  beforeEach(() => {
    saved = {};
    for (const k of KEYS) {
      saved[k] = process.env[k];
      delete process.env[k];
    }
  });

  afterEach(() => {
    for (const k of KEYS) {
      if (saved[k] === undefined) delete process.env[k];
      else process.env[k] = saved[k];
    }
  });

  test("returns null outside GCP", async () => {
    const src = new GCPWorkloadIdentitySource({ audience: "http://m" });
    expect(await src.fetch()).toBeNull();
  });

  test("fetches token from metadata server", async () => {
    process.env.GOOGLE_CLOUD_PROJECT = "p";
    const fetchImpl = vi.fn(async () => new Response("gcp-id-token", { status: 200 }));
    const src = new GCPWorkloadIdentitySource({
      audience: "http://m",
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    const tok = await src.fetch();
    expect(tok?.token).toBe("gcp-id-token");
    expect(fetchImpl).toHaveBeenCalledOnce();
    const calledUrl = (fetchImpl.mock.calls[0]?.[0] as URL).toString();
    expect(calledUrl).toContain("audience=http%3A%2F%2Fm");
  });

  test("returns null on metadata error", async () => {
    process.env.GOOGLE_CLOUD_PROJECT = "p";
    const fetchImpl = vi.fn(async () => new Response("", { status: 500 }));
    const src = new GCPWorkloadIdentitySource({
      audience: "http://m",
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    expect(await src.fetch()).toBeNull();
  });
});

describe("github actions source", () => {
  const KEYS = ["ACTIONS_ID_TOKEN_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_TOKEN"];
  let saved: Record<string, string | undefined> = {};

  beforeEach(() => {
    saved = {};
    for (const k of KEYS) {
      saved[k] = process.env[k];
      delete process.env[k];
    }
  });

  afterEach(() => {
    for (const k of KEYS) {
      if (saved[k] === undefined) delete process.env[k];
      else process.env[k] = saved[k];
    }
  });

  test("returns null without env", async () => {
    const src = new GitHubActionsSource({ audience: "http://m" });
    expect(await src.fetch()).toBeNull();
  });

  test("fetches OIDC token", async () => {
    process.env.ACTIONS_ID_TOKEN_REQUEST_URL = "http://gh/token";
    process.env.ACTIONS_ID_TOKEN_REQUEST_TOKEN = "request-bearer";
    const fetchImpl = vi.fn(async (url: URL | string, init?: RequestInit) => {
      const auth = (init?.headers as Record<string, string> | undefined)?.Authorization;
      expect(auth).toBe("Bearer request-bearer");
      return new Response(JSON.stringify({ value: "github-oidc-jwt" }), { status: 200 });
    });
    const src = new GitHubActionsSource({
      audience: "http://m",
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });
    const tok = await src.fetch();
    expect(tok?.token).toBe("github-oidc-jwt");
  });

  test("returns null on bad response shape", async () => {
    process.env.ACTIONS_ID_TOKEN_REQUEST_URL = "http://gh/token";
    process.env.ACTIONS_ID_TOKEN_REQUEST_TOKEN = "request-bearer";
    const fetchImpl = vi.fn(
      async () => new Response(JSON.stringify({ unexpected: "shape" }), { status: 200 }),
    );
    const src = new GitHubActionsSource({ fetchImpl: fetchImpl as unknown as typeof fetch });
    expect(await src.fetch()).toBeNull();
  });
});
