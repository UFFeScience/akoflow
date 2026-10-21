import React from "react";
import Link from "@docusaurus/Link";
import useBaseUrl from "@docusaurus/useBaseUrl";
import { useBlogPost } from "@docusaurus/plugin-content-blog/client";
import BlogThemeSwitch from "@site/src/components/blog/BlogThemeSwitch";
import WorkflowDag from "@site/src/components/WorkflowDag";
import BlogPhotoHero from "@site/src/components/blog/BlogPhotoHero";
import BlogAuthorByline from "@site/src/components/blog/BlogAuthorByline";
import styles from "@site/src/components/blog/blog.module.css";

export default function BlogPostItemHeader() {
  const { metadata } = useBlogPost();
  const { title, description, frontMatter } = metadata;
  const image = useBaseUrl(
    frontMatter.image ??
      "/img/interface/onboarding/first-simulation-result.png",
  );
  const coverDag = Array.isArray(frontMatter.cover_dag)
    ? frontMatter.cover_dag.filter(
        (stage): stage is string => typeof stage === "string",
      )
    : [];
  return (
    <header className={`${styles.articleHeader} ${styles.photoArticleHeader}`}>
      <div className={styles.topline}>
        <Link to="/blog">← All stories</Link>
        <BlogThemeSwitch />
      </div>
      <BlogPhotoHero
        src={image}
        alt={String(frontMatter.cover_alt ?? title)}
        title={title}
        description={description}
        category={metadata.tags.map((tag) => tag.label).join(" / ")}
      />
      <BlogAuthorByline metadata={metadata} />
      {coverDag.length > 0 && (
        <div className={styles.articleCover}>
          <WorkflowDag title={title} stages={coverDag} />
        </div>
      )}
    </header>
  );
}
