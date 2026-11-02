import React from "react";
import Link from "@docusaurus/Link";
import useBaseUrl from "@docusaurus/useBaseUrl";
import type { PropBlogPostMetadata } from "@docusaurus/plugin-content-blog";
import styles from "./blog.module.css";

export default function BlogCard({ post }: { post: PropBlogPostMetadata }) {
  const image = useBaseUrl(
    post.frontMatter.image ??
      "/img/interface/onboarding/first-simulation-result.png",
  );
  return (
    <article className={styles.card}>
      <Link
        to={post.permalink}
        className={`${styles.cardImage} ${post.frontMatter.cover_style === "photograph" ? styles.photoCard : ""}`}
        tabIndex={-1}
        aria-hidden="true"
      >
        <img src={image} alt="" loading="lazy" />
        <span>FIELD NOTES</span>
      </Link>
      <div className={styles.cardBody}>
        <p className={styles.eyebrow}>
          {post.tags.map((tag) => tag.label).join(" · ")}
        </p>
        <h2>
          <Link to={post.permalink}>{post.title}</Link>
        </h2>
        <p>{post.description}</p>
        <div className={styles.cardMeta}>
          <span>{Math.ceil(post.readingTime ?? 1)} min read</span>
          <Link to={post.permalink} aria-label={`Read ${post.title}`}>
            Read the story ↗
          </Link>
        </div>
      </div>
    </article>
  );
}
