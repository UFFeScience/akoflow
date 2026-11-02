import React from "react";
import type { PropBlogPostMetadata } from "@docusaurus/plugin-content-blog";
import styles from "./blog.module.css";

export default function BlogAuthorByline({
  metadata,
}: {
  metadata: PropBlogPostMetadata;
}) {
  return (
    <div id="article-content" className={styles.byline}>
      {metadata.authors
        .filter((author) => author.imageURL)
        .map((author) => (
          <a key={author.key ?? author.name} href={author.url}>
            <img
              className={styles.authorPortrait}
              src={author.imageURL}
              alt={author.name}
              width={48}
              height={48}
              loading="lazy"
              decoding="async"
            />
          </a>
        ))}
      <div className={styles.bylineContent}>
        <div>
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
        </div>
        <div>
          <time dateTime={metadata.date}>
            {new Date(metadata.date).toLocaleDateString("en-US", {
              year: "numeric",
              month: "long",
              day: "numeric",
              timeZone: "UTC",
            })}
          </time>{" "}
          · {Math.ceil(metadata.readingTime ?? 1)} min read
        </div>
      </div>
    </div>
  );
}
