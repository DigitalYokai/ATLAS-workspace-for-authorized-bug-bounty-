import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";

describe("collection API client", () => {
  afterEach(() => vi.restoreAllMocks());

  it("sends asset type and scope filters to the target-scoped endpoint", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: [] }), { status: 200 }));

    await api.entities("target-1", { type: "subdomain", scopeStatus: "included" });

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(String(fetchMock.mock.calls[0][0])).toBe("http://localhost:8080/api/v1/targets/target-1/entities?type=subdomain&scopeStatus=included");
  });
});
