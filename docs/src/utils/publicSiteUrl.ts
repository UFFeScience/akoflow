export function publicSiteUrl(path: string): string {
  const url = new URL(path, "https://akoflow.com");
  const segments = url.pathname.split("/");
  if (segments[1] === "akoflow") {
    url.pathname = "/" + segments.slice(2).join("/");
  }
  url.protocol = "https:";
  url.host = "akoflow.com";
  url.search = "";
  url.hash = "";
  return url.href;
}
