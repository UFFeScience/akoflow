export const releasesEndpoint =
  "https://api.github.com/repos/UFFeScience/akoflow/releases";
export const releasesPage = "https://github.com/UFFeScience/akoflow/releases";

export function normalizeReleases(records) {
  const releases = new Map();
  for (const record of records) {
    if (!record || record.draft || typeof record.tag_name !== "string")
      continue;
    const tag = record.tag_name.trim();
    if (!tag) continue;
    const publishedAt = record.published_at || record.created_at || "";
    const release = {
      id: String(record.id ?? tag),
      tag,
      name:
        typeof record.name === "string" && record.name.trim()
          ? record.name.trim()
          : tag,
      body: typeof record.body === "string" ? record.body : "",
      url: `${releasesPage}/tag/${encodeURIComponent(tag)}`,
      publishedAt,
      prerelease: Boolean(record.prerelease),
      assets: (Array.isArray(record.assets) ? record.assets : []).flatMap(
        (asset) => {
          if (typeof asset.name !== "string" || !asset.name) return [];
          return [
            {
              name: asset.name,
              size:
                Number.isFinite(asset.size) && asset.size >= 0 ? asset.size : 0,
              url: `https://github.com/UFFeScience/akoflow/releases/download/${encodeURIComponent(tag)}/${encodeURIComponent(asset.name)}`,
            },
          ];
        },
      ),
    };
    const previous = releases.get(tag);
    if (
      !previous ||
      Date.parse(publishedAt) >= Date.parse(previous.publishedAt)
    ) {
      releases.set(tag, release);
    }
  }
  return [...releases.values()].sort((a, b) => {
    const dateOrder =
      (Date.parse(b.publishedAt) || 0) - (Date.parse(a.publishedAt) || 0);
    return dateOrder || b.tag.localeCompare(a.tag);
  });
}

export async function fetchAllReleases(
  fetcher = globalThis.fetch,
  options = {},
) {
  const headers = {
    Accept: "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
  };
  if (options.token) headers.Authorization = `Bearer ${options.token}`;
  let next = `${releasesEndpoint}?per_page=100&page=1`;
  const visited = new Set();
  const records = [];
  while (next) {
    const url = new URL(next);
    const expected = new URL(releasesEndpoint);
    const validPath =
      url.pathname === expected.pathname ||
      url.pathname === "/repositories/827382806/releases";
    if (url.origin !== expected.origin || !validPath || visited.has(next)) {
      throw new Error("Unexpected GitHub releases pagination link");
    }
    visited.add(next);
    const response = await fetcher(next, {
      headers,
      signal: options.signal,
      cache: "no-store",
    });
    if (!response.ok)
      throw new Error(`GitHub releases request failed (${response.status})`);
    const page = await response.json();
    if (!Array.isArray(page))
      throw new Error("GitHub releases response is not a list");
    records.push(...page);
    const link = response.headers.get("link") || "";
    next = /<([^>]+)>;\s*rel="next"/.exec(link)?.[1] || "";
  }
  return normalizeReleases(records);
}
