import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  autoResizeSnippet,
  embedBaseUrl,
  escapeHtmlAttribute,
  fixedHeightSnippet,
  jsxSnippet,
} from "./form-snippet";

const KEY = "pk_AbC123dEf456GhI789jkl0";

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_EMBED_BASE_URL", "https://embed.jualin.test");
  vi.stubEnv("NEXT_PUBLIC_API_BASE_URL", "http://localhost:8080");
});

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("embedBaseUrl", () => {
  it("prefers the dedicated embed var over the API base", () => {
    expect(embedBaseUrl()).toBe("https://embed.jualin.test");
  });

  it("falls back to the API base when the embed var is unset", () => {
    vi.stubEnv("NEXT_PUBLIC_EMBED_BASE_URL", "");
    expect(embedBaseUrl()).toBe("http://localhost:8080");
  });

  it("strips a trailing slash so the path join never doubles it", () => {
    vi.stubEnv("NEXT_PUBLIC_EMBED_BASE_URL", "https://embed.jualin.test/");
    expect(embedBaseUrl()).toBe("https://embed.jualin.test");
  });
});

describe("escapeHtmlAttribute", () => {
  it("escapes the characters that would break a double-quoted attribute", () => {
    expect(escapeHtmlAttribute(`Toko "Anwar" <b>`)).toBe("Toko &quot;Anwar&quot; &lt;b&gt;");
  });

  it("escapes ampersands before anything else", () => {
    expect(escapeHtmlAttribute("A & B")).toBe("A &amp; B");
  });
});

describe("autoResizeSnippet", () => {
  const snippet = () => autoResizeSnippet({ publicKey: KEY, formName: "Formulir Kontak" });

  it("embeds the resolved base URL and the public key in the iframe src", () => {
    expect(snippet()).toContain(`src="https://embed.jualin.test/embed/${KEY}"`);
  });

  it("includes the companion script from the same origin", () => {
    expect(snippet()).toContain(`<script src="https://embed.jualin.test/embed.js" async></script>`);
  });

  it("keeps the data-jualin-form marker the script looks for", () => {
    expect(snippet()).toContain("data-jualin-form");
  });

  it("carries an initial height so the iframe does not flicker before the script runs", () => {
    expect(snippet()).toContain(`height="620"`);
  });

  it("escapes the form name inside the title attribute", () => {
    const out = autoResizeSnippet({ publicKey: KEY, formName: `Promo "Akhir Tahun"` });
    expect(out).toContain(`title="Promo &quot;Akhir Tahun&quot;"`);
    expect(out).not.toContain(`title="Promo "Akhir Tahun""`);
  });
});

describe("fixedHeightSnippet", () => {
  it("has the iframe but no script tag", () => {
    const out = fixedHeightSnippet({ publicKey: KEY, formName: "Formulir Kontak" });
    expect(out).toContain(`src="https://embed.jualin.test/embed/${KEY}"`);
    expect(out).not.toContain("embed.js");
    expect(out).not.toContain("<script");
  });
});

describe("jsxSnippet", () => {
  const out = () => jsxSnippet({ publicKey: KEY, formName: `Promo "Akhir Tahun"` });

  it("uses a style object and a numeric height, not HTML-attribute syntax", () => {
    expect(out()).toContain("style={{ border: 0 }}");
    expect(out()).toContain("height={620}");
  });

  it("passes the title as a JS string literal, not an HTML-escaped attribute", () => {
    expect(out()).toContain(`title={"Promo \\"Akhir Tahun\\""}`);
  });

  it("still loads the companion script", () => {
    expect(out()).toContain(`<script src="https://embed.jualin.test/embed.js" async></script>`);
  });
});
