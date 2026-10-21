import React from "react";
import ExpandableImage from "../ExpandableImage";
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
      <ExpandableImage src={src} alt={alt} />
      <div className={styles.photoHeroText}>
        <p className={styles.eyebrow}>{category}</p>
        <h1>{title}</h1>
        <p>{description}</p>
        <span>Click the mosaic to explore the full image ↗</span>
      </div>
    </div>
  );
}
