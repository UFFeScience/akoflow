export const blogSchedulePluginName = "akoflow-blog-schedule";
export const blogPreviewQuery = "?preview=scheduled";

export function isBlogPreview(search: string): boolean {
  return new URLSearchParams(search).get("preview") === "scheduled";
}

export function isScheduledBlogPost(
  frontMatter: Record<string, unknown>,
  now = Date.now(),
): boolean {
  const date = frontMatter.publish_at;
  if (date === undefined) return false;
  if (
    typeof date !== "string" ||
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})$/.test(date) ||
    !Number.isFinite(Date.parse(date))
  ) {
    throw new Error("Blog publish_at must be a quoted ISO timestamp with an explicit timezone.");
  }
  return Date.parse(date) > now;
}
