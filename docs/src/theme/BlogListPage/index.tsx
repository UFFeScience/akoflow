import React, { useState } from "react";
import { usePluginData } from "@docusaurus/useGlobalData";
import { useLocation } from "@docusaurus/router";
import useIsBrowser from "@docusaurus/useIsBrowser";
import type { PropBlogPostMetadata } from "@docusaurus/plugin-content-blog";
import { blogPreviewQuery, blogSchedulePluginName, isBlogPreview, isScheduledBlogPost } from "@site/src/utils/blogSchedule";
import Layout from "@theme/Layout";
import type { Props } from "@theme/BlogListPage";
import BlogBackdrop from "@site/src/components/blog/BlogBackdrop";
import BlogLinkHighlights from "@site/src/components/blog/BlogLinkHighlights";
import BlogThemeSwitch from "@site/src/components/blog/BlogThemeSwitch";
import BlogCard from "@site/src/components/blog/BlogCard";
import styles from "@site/src/components/blog/blog.module.css";

export default function BlogListPage({ items, metadata }: Props) {
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("All stories");
  const { search } = useLocation();
  const isBrowser = useIsBrowser();
  const preview = isBrowser && isBlogPreview(search);
  const scheduledPosts = usePluginData(blogSchedulePluginName) as PropBlogPostMetadata[];
  const posts = [
    ...items.map(({ content }) => content.metadata),
    ...scheduledPosts.filter(({ frontMatter }) =>
      isBrowser && (preview || !isScheduledBlogPost(frontMatter)),
    ),
  ].sort((a, b) => Date.parse(b.date) - Date.parse(a.date));
  const categories = [
    "All stories",
    ...new Set(posts.flatMap((post) => post.tags.map((tag) => tag.label))),
  ];
  const visible = posts.filter((post) => {
    const matchesCategory =
      category === "All stories" ||
      post.tags.some((tag) => tag.label === category);
    const searchText = `${post.title} ${post.description} ${post.tags.map((tag) => tag.label).join(" ")}`;
    return (
      matchesCategory &&
      searchText.toLowerCase().includes(query.trim().toLowerCase())
    );
  });
  function clearFilters() {
    setQuery("");
    setCategory("All stories");
  }
  return (
    <Layout title={metadata.blogTitle} description={metadata.blogDescription}>
      <main className={styles.shell} data-blog-link-highlights>
        <BlogBackdrop />
        <BlogLinkHighlights />
        <div className={styles.content}>
          <header className={styles.intro}>
            <div className={styles.topline}>
              <span className={styles.eyebrow}>SCIENTIFIC WORKFLOWS</span>
              <BlogThemeSwitch />
            </div>
            <h1>
              AkôFlow <span>Blog</span>
            </h1>
            <p>Tutorials, experiments, and results from the AkôFlow project.</p>
            <div className={styles.introFooter}>
              <span>↓ Explore the stories</span>
            </div>
          </header>
          <section aria-label="Browse articles" className={styles.browse}>
            <div className={styles.filterRow}>
              <div
                className={styles.filters}
                role="group"
                aria-label="Article categories"
              >
                {categories.map((label) => (
                  <button
                    key={label}
                    type="button"
                    aria-pressed={label === category}
                    onClick={() => setCategory(label)}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <input
                type="search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                aria-label="Search articles"
                placeholder="Search the journal…"
              />
            </div>
            <p className={styles.resultCount} role="status">
              {visible.length} {visible.length === 1 ? "story" : "stories"}
            </p>
            <div className={styles.grid}>
              {visible.map((post) => (
                <BlogCard key={post.permalink} post={post} query={preview ? blogPreviewQuery : undefined} />
              ))}
            </div>
            {visible.length === 0 && (
              <div className={styles.empty}>
                <h2>No stories found.</h2>
                <p>Try a different topic or search term.</p>
                <button type="button" onClick={clearFilters}>
                  Clear filters
                </button>
              </div>
            )}
          </section>
        </div>
      </main>
    </Layout>
  );
}
