import React from 'react';
import {useColorMode} from '@docusaurus/theme-common';
import useBaseUrl from '@docusaurus/useBaseUrl';

type Callout = {
  number: number;
  label: string;
  x?: number;
  y?: number;
};

type AnnotatedScreenshotProps = {
  src?: string;
  darkSrc?: string;
  alt: string;
  caption?: string;
  callouts?: Callout[];
};

export default function AnnotatedScreenshot({
  src,
  darkSrc,
  alt,
  caption,
  callouts = [],
}: AnnotatedScreenshotProps) {
  const {colorMode} = useColorMode();
  const activeSrc = colorMode === 'dark' && darkSrc ? darkSrc : src;
  const resolvedSrc = useBaseUrl(activeSrc ?? '');

  if (!activeSrc) {
    return (
      <aside className="akoflow-media-placeholder" aria-label={alt}>
        <strong>Screenshot planned</strong>
        <span>{alt}</span>
      </aside>
    );
  }

  return (
    <figure className="akoflow-annotated-screenshot">
      <div className="akoflow-annotated-screenshot__image">
        <img src={resolvedSrc} alt={alt} loading="lazy" />
        {callouts
          .filter((callout) => callout.x !== undefined && callout.y !== undefined)
          .map((callout) => (
            <span
              key={callout.number}
              className="akoflow-annotated-screenshot__marker"
              style={{left: `${callout.x}%`, top: `${callout.y}%`}}
              aria-hidden="true"
            >
              {callout.number}
            </span>
          ))}
      </div>
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
