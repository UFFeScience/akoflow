import React from 'react';

type DemoVideoProps = {
  src?: string;
  poster?: string;
  title: string;
  description?: string;
};

export default function DemoVideo({src, poster, title, description}: DemoVideoProps) {
  if (!src) {
    return (
      <aside className="akoflow-media-placeholder" aria-label={title}>
        <strong>Walkthrough planned</strong>
        <span>{description || title}</span>
      </aside>
    );
  }

  return (
    <figure className="akoflow-demo-video">
      <video controls preload="metadata" poster={poster} aria-label={title}>
        <source src={src} />
        Your browser does not support embedded video.
      </video>
      {description && <figcaption>{description}</figcaption>}
    </figure>
  );
}
