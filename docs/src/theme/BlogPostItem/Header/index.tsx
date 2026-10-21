import React from "react";
import Link from "@docusaurus/Link";
import useBaseUrl from "@docusaurus/useBaseUrl";
import { useBlogPost } from "@docusaurus/plugin-content-blog/client";
import BlogThemeSwitch from "@site/src/components/blog/BlogThemeSwitch";
import ExpandableImage from "@site/src/components/ExpandableImage";
import WorkflowDag from "@site/src/components/WorkflowDag";
import BlogPhotoHero from "@site/src/components/blog/BlogPhotoHero";
import styles from "@site/src/components/blog/blog.module.css";

export default function BlogPostItemHeader() {
  const { metadata } = useBlogPost();
  const { title, description, readingTime, frontMatter } = metadata;
  const image = useBaseUrl(
    frontMatter.image ??
      "/img/interface/onboarding/first-simulation-result.png",
  );
  const coverDag = Array.isArray(frontMatter.cover_dag)
    ? frontMatter.cover_dag.filter(
        (stage): stage is string => typeof stage === "string",
      )
    : [];
  const photoCover = frontMatter.cover_style === "photograph";
  return (
    <header className={styles.articleHeader}>
      <div className={styles.topline}>
        <Link to="/blog">← All stories</Link>
        <BlogThemeSwitch />
      </div>
      {photoCover ? (
        <BlogPhotoHero
          src={image}
          alt={String(frontMatter.cover_alt ?? title)}
          title={title}
          description={description}
          category={metadata.tags.map((tag) => tag.label).join(" / ")}
        />
      ) : (
        <>
          <p className={styles.eyebrow}>
            {metadata.tags.map((tag) => tag.label).join(" / ")}
          </p>
          <h1>{title}</h1>
          <p className={styles.dek}>{description}</p>
        </>
      )}
      <p className={styles.byline}>
        {metadata.authors.map((author, index) => (
          <React.Fragment key={author.key ?? author.name}>
            {index > 0 && ", "}
            {author.url ? (
              <a href={author.url}>
                {author.name}
                {author.title ? ` (${author.title})` : ""}
              </a>
            ) : (
              <span>{author.name}</span>
            )}
          </React.Fragment>
        ))}
        {metadata.authors.length > 0 && " · "}
        <time dateTime={metadata.date}>
          {new Date(metadata.date).toLocaleDateString("en-US", {
            year: "numeric",
            month: "long",
            day: "numeric",
            timeZone: "UTC",
          })}
        </time>{" "}
        · {Math.ceil(readingTime ?? 1)} min read
      </p>
      {!photoCover && (
        <div className={styles.articleCover}>
          {coverDag.length > 0 ? (
            <WorkflowDag title={title} stages={coverDag} />
          ) : (
            <ExpandableImage
              src={image}
              alt={String(
                frontMatter.cover_alt ??
                  "AkôFlow workflow execution and its observed results",
              )}
            />
          )}
        </div>
      )}
    </header>
  );
}
