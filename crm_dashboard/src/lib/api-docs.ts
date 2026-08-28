// Builds the one curl example both the create-key reveal dialog (#48,
// where a genuinely working secret is available) and the integration
// docs page (#49, a durable reference — only ever has key_prefix, see
// that page's own doc comment for why) render. One function so the two
// places can never quietly drift into different formats.
export function buildCurlExample(apiBaseUrl: string, credential: string): string {
  return `curl -X POST ${apiBaseUrl}/v1/leads \\
  -H "Authorization: Bearer ${credential}" \\
  -H "Content-Type: application/json" \\
  -d '{"name": "Budi Santoso", "email": "budi@example.com"}'`;
}
