import React from "react";
import styles from "./blog.module.css";

export default function BlogPhotoHero({
  src,
  alt,
  title,
  description,
  category,
}: {
  src: string;
  alt: string;
  title: string;
  description: string;
  category: string;
}) {
  return (
    <div className={styles.photoHero}>
      <img
        src={src}
        alt={alt}
        loading="eager"
        fetchPriority="high"
        draggable={false}
      />
      <div className={styles.photoHeroText}>
        <p className={styles.eyebrow}>{category}</p>
        <h1>{title}</h1>
        <p>{description}</p>
        <a className={styles.readStory} href="#article-content">
          Read the story <span aria-hidden="true">↓</span>
        </a>
      </div>
    </div>
  );
}
