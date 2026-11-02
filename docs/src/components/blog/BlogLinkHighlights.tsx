import { useEffect } from "react";
import { useLocation } from "@docusaurus/router";
import styles from "./BlogLinkHighlights.module.css";

export default function BlogLinkHighlights() {
  const { pathname } = useLocation();

  useEffect(() => {
    const root = document.querySelector("[data-blog-link-highlights]");
    if (!root) return;

    const tracked = new Set<HTMLAnchorElement>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          entry.target.classList.add(styles.painted);
          observer.unobserve(entry.target);
        }
      },
      { rootMargin: "0px 0px -12% 0px", threshold: 0.1 },
    );

    function observeLinks() {
      const links = root!.querySelectorAll<HTMLAnchorElement>(
        ".markdown a[href]:not(.hash-link):not([aria-hidden='true']), article h2 a[href], article a[aria-label^='Read '], aside p a[href]",
      );
      for (const link of Array.from(links)) {
        if (tracked.has(link) || link.querySelector("img, svg")) continue;
        tracked.add(link);
        link.classList.add(styles.link);
        observer.observe(link);
      }
    }

    observeLinks();
    const mutations = new MutationObserver(observeLinks);
    mutations.observe(root, { childList: true, subtree: true });

    return () => {
      mutations.disconnect();
      observer.disconnect();
      for (const link of tracked) {
        link.classList.remove(styles.link, styles.painted);
      }
    };
  }, [pathname]);

  return null;
}
