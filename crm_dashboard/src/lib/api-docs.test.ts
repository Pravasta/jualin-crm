import { describe, expect, it } from "vitest";
import { buildCurlExample } from "./api-docs";

describe("buildCurlExample", () => {
  it("targets POST /v1/leads on the given base URL", () => {
    const example = buildCurlExample("https://api.example.com", "jln_live_abc_secret");
    expect(example).toContain("curl -X POST https://api.example.com/v1/leads");
  });

  it("substitutes the credential into the Authorization header verbatim", () => {
    const example = buildCurlExample("https://api.example.com", "jln_live_abc_secret");
    expect(example).toContain('-H "Authorization: Bearer jln_live_abc_secret"');
  });

  it("works identically for a real secret and a placeholder-style credential — same function, same shape, both callers", () => {
    const real = buildCurlExample("https://api.example.com", "jln_live_a3f9_realSecretValue");
    const placeholder = buildCurlExample("https://api.example.com", "jln_live_a3f9...<secret_anda>");
    expect(real.replace("jln_live_a3f9_realSecretValue", "X")).toBe(
      placeholder.replace("jln_live_a3f9...<secret_anda>", "X")
    );
  });
});
