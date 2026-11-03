import assert from "node:assert/strict";
import { test } from "node:test";
import {
  fetchAllReleases,
  normalizeReleases,
  releasesEndpoint,
} from "../src/lib/releases.mjs";

function release(index, extra = {}) {
  return {
    id: index,
    tag_name: `v1.0.${index}`,
    name: `Release ${index}`,
    body: `Changes in ${index}`,
    published_at: new Date(Date.UTC(2026, 0, 1, 0, index)).toISOString(),
    draft: false,
    prerelease: false,
    assets: [],
    ...extra,
  };
}

test("loads the complete paginated history, including public pre-releases", async () => {
  const calls = [];
  const second =
    "https://api.github.com/repositories/827382806/releases?per_page=100&page=2";
  const fetcher = async (url) => {
    calls.push(url);
    const first = calls.length === 1;
    const records = first
      ? Array.from({ length: 100 }, (_, index) => release(index))
      : [
          release(100, { prerelease: true }),
          release(101),
          release(102, { draft: true }),
        ];
    return new Response(JSON.stringify(records), {
      headers: first ? { Link: `<${second}>; rel="next"` } : {},
    });
  };
  const history = await fetchAllReleases(fetcher);
  assert.equal(calls.length, 2);
  assert.equal(calls[1], second);
  assert.equal(history.length, 102);
  assert.equal(history[0].tag, "v1.0.101");
  assert.equal(history[1].prerelease, true);
  assert.ok(!history.some((item) => item.tag === "v1.0.102"));
});

test("reflects edited release bodies without duplicating version tags", () => {
  const history = normalizeReleases([
    release(1),
    release(2, {
      tag_name: "v1.0.1",
      body: "Updated notes with ![Screen](https://example.com/screen.png)",
    }),
    release(3, { name: "", assets: [{ name: "Desktop app.zip", size: 1234 }] }),
    { draft: false },
  ]);
  assert.equal(history.length, 2);
  assert.equal(history[0].name, "v1.0.3");
  assert.match(history[0].assets[0].url, /Desktop%20app\.zip$/);
  assert.match(history[1].body, /Updated notes/);
});

test("fails explicitly on rate limits and malformed responses so callers can keep their saved history", async () => {
  await assert.rejects(
    fetchAllReleases(async () => new Response("{}", { status: 429 })),
    /429/,
  );
  await assert.rejects(
    fetchAllReleases(async () => new Response("{}")),
    /not a list/,
  );
});

test("never follows an unexpected pagination host or a repeated next page", async () => {
  let requests = 0;
  await assert.rejects(
    fetchAllReleases(
      async () => {
        requests++;
        return new Response("[]", {
          headers: { Link: '<https://example.com/releases>; rel="next"' },
        });
      },
      { token: "test-token" },
    ),
    /Unexpected/,
  );
  assert.equal(requests, 1);
  await assert.rejects(
    fetchAllReleases(
      async () =>
        new Response("[]", {
          headers: {
            Link: `<${releasesEndpoint}?per_page=100&page=1>; rel="next"`,
          },
        }),
    ),
    /Unexpected/,
  );
});
