export type ReleaseAsset = { name: string; size: number; url: string };
export type Release = {
  id: string;
  tag: string;
  name: string;
  body: string;
  url: string;
  publishedAt: string;
  prerelease: boolean;
  assets: ReleaseAsset[];
};
export type ReleaseSnapshot = { updatedAt: string; releases: Release[] };
export const releasesEndpoint: string;
export const releasesPage: string;
export function normalizeReleases(records: unknown[]): Release[];
export function fetchAllReleases(
  fetcher?: typeof fetch,
  options?: { token?: string; signal?: AbortSignal },
): Promise<Release[]>;
