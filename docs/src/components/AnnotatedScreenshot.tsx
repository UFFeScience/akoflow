import React from 'react';

type Callout = {
  number: number;
  label: string;
};

type AnnotatedScreenshotProps = {
  src?: string;
  alt: string;
  caption?: string;
  callouts?: Callout[];
};

export default function AnnotatedScreenshot({
  src,
  alt,
  caption,
  callouts = [],
}: AnnotatedScreenshotProps) {
  if (!src) {
    return (
      <aside className="akoflow-media-placeholder" aria-label={alt}>
        <strong>Screenshot planned</strong>
        <span>{alt}</span>
      </aside>
    );
  }

  return (
    <figure className="akoflow-annotated-screenshot">
      <img src={src} alt={alt} loading="lazy" />
      {(caption || callouts.length > 0) && (
        <figcaption>
          {caption && <p>{caption}</p>}
          {callouts.length > 0 && (
            <ol>
              {callouts.map((callout) => (
                <li key={callout.number} value={callout.number}>
                  {callout.label}
                </li>
              ))}
            </ol>
          )}
        </figcaption>
      )}
    </figure>
  );
}
