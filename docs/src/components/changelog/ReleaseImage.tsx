import React, { type ImgHTMLAttributes } from "react";
import ExpandableImage from "../ExpandableImage";

export default function ReleaseImage({
  src,
  alt,
  title,
}: ImgHTMLAttributes<HTMLImageElement>) {
  if (!src) return null;
  return (
    <ExpandableImage
      src={src}
      alt={alt || "Release screenshot"}
      title={title}
      loading="lazy"
    />
  );
}
